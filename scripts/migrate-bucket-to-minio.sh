#!/usr/bin/env bash
#
# migrate-bucket-to-minio.sh — Copy existing media objects from one
# S3-compatible bucket to another (Railway Bucket → self-hosted MinIO).
#
# Why a plain bucket mirror is enough: the DB only ever stores the RELATIVE
# token "<userID>/<uuid>.<ext>" (see internal/storage/verification_media.go),
# never the endpoint or bucket. As long as the destination keeps the SAME key
# layout "<prefix>/<userID>/<uuid>.<ext>", every existing token resolves
# against the new store with NO database change. So we mirror the whole
# bucket, which carries both the commercial-activity/ and verification-uploads/
# prefixes along automatically.
#
# Object content-type is preserved by the copy, which matters: the serve
# handler returns it straight from the object on read.
#
# ── Where to run this ────────────────────────────────────────────────────
#   • Railway Bucket (source) is a public S3 endpoint → reachable anywhere.
#   • MinIO (dest) is only reachable where its endpoint is:
#       - local dev MinIO  → run from your laptop (dest = http://localhost:9000)
#       - MinIO on Railway → run from inside Railway (`railway run ...` in the
#         MinIO/backend service so DST_S3_ENDPOINT=*.railway.internal resolves),
#         OR temporarily give MinIO a public domain, migrate, then remove it.
#
# ── Usage ────────────────────────────────────────────────────────────────
#   # Source from the backend .env, dest defaults to local MinIO:
#   ./scripts/migrate-bucket-to-minio.sh --src-env .env --dry-run
#   ./scripts/migrate-bucket-to-minio.sh --src-env .env            # for real
#
#   # Or pass everything explicitly via env vars:
#   SRC_S3_ENDPOINT=https://<railway-bucket-host> \
#   SRC_S3_BUCKET=<railway-bucket> \
#   SRC_S3_ACCESS_KEY_ID=... SRC_S3_SECRET_ACCESS_KEY=... \
#   DST_S3_ENDPOINT=http://localhost:9000 DST_S3_BUCKET=safephone-media \
#   DST_S3_ACCESS_KEY_ID=safephone DST_S3_SECRET_ACCESS_KEY=safephone-secret \
#   ./scripts/migrate-bucket-to-minio.sh
#
#   Flags:
#     --src-env <path>   load SRC_* from a .env-style file (maps S3_* → SRC_S3_*)
#     --dst-env <path>   load DST_* from a .env-style file (maps S3_* → DST_S3_*)
#     --prefix <p>       mirror only objects under this prefix (default: whole bucket)
#     --dry-run          show what WOULD copy, transfer nothing
#     --yes              skip the confirmation prompt
#     -h, --help         this help

set -euo pipefail

# ──────────────────────────────────────────────────────────────────────
# Args
# ──────────────────────────────────────────────────────────────────────

SRC_ENV_FILE=""
DST_ENV_FILE=""
PREFIX=""
DRY_RUN="false"
ASSUME_YES="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --src-env) SRC_ENV_FILE="$2"; shift 2 ;;
    --dst-env) DST_ENV_FILE="$2"; shift 2 ;;
    --prefix)  PREFIX="$2"; shift 2 ;;
    --dry-run) DRY_RUN="true"; shift ;;
    --yes)     ASSUME_YES="true"; shift ;;
    -h|--help) sed -n '2,55p' "$0"; exit 0 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

# ──────────────────────────────────────────────────────────────────────
# Load env files. We read only the S3_* keys and remap them to SRC_/DST_
# so the same backend .env can seed the source side untouched.
# ──────────────────────────────────────────────────────────────────────

# load_env <file> <prefix>  — export <prefix>_KEY for each S3_KEY in <file>,
# without clobbering values already set in the environment.
load_env() {
  local file="$1" dest_prefix="$2" key val line
  [[ -f "$file" ]] || { echo "env file not found: $file" >&2; exit 1; }
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%%$'\r'}"
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    [[ "$line" =~ ^[[:space:]]*$ ]] && continue
    key="${line%%=*}"; key="${key//[[:space:]]/}"
    [[ "$key" == S3_* ]] || continue
    val="${line#*=}"
    val="${val%\"}"; val="${val#\"}"; val="${val%\'}"; val="${val#\'}"
    local target="${dest_prefix}_${key}"
    [[ -n "${!target:-}" ]] || export "$target=$val"
  done < "$file"
}

[[ -n "$SRC_ENV_FILE" ]] && load_env "$SRC_ENV_FILE" "SRC"
[[ -n "$DST_ENV_FILE" ]] && load_env "$DST_ENV_FILE" "DST"

# Destination defaults → local MinIO from the infra compose.
: "${DST_S3_ENDPOINT:=http://localhost:9000}"
: "${DST_S3_BUCKET:=safephone-media}"
: "${DST_S3_ACCESS_KEY_ID:=safephone}"
: "${DST_S3_SECRET_ACCESS_KEY:=safephone-secret}"

require() {
  local name="$1"
  [[ -n "${!name:-}" ]] || { echo "Missing required value: $name" >&2; exit 1; }
}
require SRC_S3_ENDPOINT
require SRC_S3_BUCKET
require SRC_S3_ACCESS_KEY_ID
require SRC_S3_SECRET_ACCESS_KEY

# ──────────────────────────────────────────────────────────────────────
# mc wrapper — prefer a local `mc`, else run the minio/mc container with a
# persistent config dir so aliases survive across invocations. When running
# via docker, localhost endpoints are rewritten to host.docker.internal.
# ──────────────────────────────────────────────────────────────────────

MC_CONFIG_DIR="$(mktemp -d)"
trap 'rm -rf "$MC_CONFIG_DIR"' EXIT

# NOTE: `mc` is also GNU Midnight Commander on many machines — only treat a
# local mc as usable if it's actually the MinIO client (RELEASE.* version).
have_minio_mc() {
  command -v mc >/dev/null 2>&1 && mc --version 2>&1 | grep -qiE 'minio|RELEASE'
}

USE_DOCKER="false"
if have_minio_mc; then
  mc() { command mc --config-dir "$MC_CONFIG_DIR" "$@"; }
elif command -v docker >/dev/null 2>&1; then
  USE_DOCKER="true"
  mc() {
    docker run --rm -i \
      --add-host host.docker.internal:host-gateway \
      -v "$MC_CONFIG_DIR:/root/.mc" \
      minio/mc:latest "$@"
  }
else
  echo "Neither 'mc' nor 'docker' is available — install one." >&2
  exit 1
fi

# From inside a container, localhost is the container itself.
dockerize_endpoint() {
  local ep="$1"
  if [[ "$USE_DOCKER" == "true" ]]; then
    ep="${ep/localhost/host.docker.internal}"
    ep="${ep/127.0.0.1/host.docker.internal}"
  fi
  printf '%s' "$ep"
}

SRC_EP="$(dockerize_endpoint "$SRC_S3_ENDPOINT")"
DST_EP="$(dockerize_endpoint "$DST_S3_ENDPOINT")"

# ──────────────────────────────────────────────────────────────────────
# Configure aliases + ensure destination bucket exists
# ──────────────────────────────────────────────────────────────────────

echo "Source:      $SRC_S3_ENDPOINT  bucket=$SRC_S3_BUCKET"
echo "Destination: $DST_S3_ENDPOINT  bucket=$DST_S3_BUCKET"
[[ -n "$PREFIX" ]] && echo "Prefix:      $PREFIX (scoped)" || echo "Prefix:      <whole bucket>"
echo

mc alias set src "$SRC_EP" "$SRC_S3_ACCESS_KEY_ID" "$SRC_S3_SECRET_ACCESS_KEY" >/dev/null
mc alias set dst "$DST_EP" "$DST_S3_ACCESS_KEY_ID" "$DST_S3_SECRET_ACCESS_KEY" >/dev/null
mc mb --ignore-existing "dst/$DST_S3_BUCKET" >/dev/null

SRC_PATH="src/$SRC_S3_BUCKET"
DST_PATH="dst/$DST_S3_BUCKET"
if [[ -n "$PREFIX" ]]; then
  SRC_PATH="$SRC_PATH/$PREFIX"
  DST_PATH="$DST_PATH/$PREFIX"
fi

SRC_COUNT="$(mc ls --recursive "$SRC_PATH" 2>/dev/null | wc -l | tr -d ' ')"
echo "Objects at source: $SRC_COUNT"
echo

if [[ "$DRY_RUN" == "true" ]]; then
  echo "── DRY RUN: nothing will be written ──"
  mc mirror --dry-run --overwrite "$SRC_PATH" "$DST_PATH"
  exit 0
fi

if [[ "$ASSUME_YES" != "true" ]]; then
  printf "Copy %s objects → %s ? [y/N] " "$SRC_COUNT" "$DST_S3_BUCKET"
  read -r reply
  [[ "$reply" =~ ^[Yy]$ ]] || { echo "Aborted."; exit 1; }
fi

# ──────────────────────────────────────────────────────────────────────
# Mirror + verify. --overwrite makes the run idempotent / re-runnable.
# ──────────────────────────────────────────────────────────────────────

mc mirror --overwrite "$SRC_PATH" "$DST_PATH"

DST_COUNT="$(mc ls --recursive "$DST_PATH" 2>/dev/null | wc -l | tr -d ' ')"
echo
echo "Done. source=$SRC_COUNT  destination=$DST_COUNT"
if [[ "$SRC_COUNT" != "$DST_COUNT" ]]; then
  echo "⚠️  Counts differ — re-run to retry, or inspect with: mc ls --recursive $DST_PATH" >&2
  exit 1
fi
echo "✅ All objects mirrored. Tokens in the DB now resolve against MinIO."
