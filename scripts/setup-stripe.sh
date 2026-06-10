#!/usr/bin/env bash
#
# setup-stripe.sh — One-shot Stripe setup for SafePhone US market.
#
# Creates 5 products + monthly prices in Stripe (test mode by default),
# then upserts STRIPE_PRICE_* keys into the backend .env file.
# Re-running creates duplicate products in Stripe — clean them up in the
# dashboard if you want a fresh slate.
#
# Usage:
#   ./scripts/setup-stripe.sh              # test mode
#   ./scripts/setup-stripe.sh --live       # live mode (BE CAREFUL)
#   ./scripts/setup-stripe.sh --env path   # custom .env path

set -euo pipefail

# ──────────────────────────────────────────────────────────────────────
# Args
# ──────────────────────────────────────────────────────────────────────

MODE_FLAG=""   # empty → test mode (CLI default); "--live" → live mode
ENV_FILE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --live) MODE_FLAG="--live"; shift ;;
    --env)  ENV_FILE="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,15p' "$0"; exit 0 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="${ENV_FILE:-$BACKEND_DIR/.env}"

# ──────────────────────────────────────────────────────────────────────
# Preflight
# ──────────────────────────────────────────────────────────────────────

if ! command -v stripe >/dev/null 2>&1; then
  echo "✗ stripe CLI not found." >&2
  echo "  Install: brew install stripe/stripe-cli/stripe" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "✗ jq not found." >&2
  echo "  Install: brew install jq" >&2
  exit 1
fi

# Auth appended to every stripe call. A full secret key in STRIPE_API_KEY
# wins: `stripe login` stores a restricted key (rk_live_…) that lacks
# product/price write scopes in live mode (403s on create). Pass a full
# sk_live_… instead:  STRIPE_API_KEY=sk_live_… ./scripts/setup-stripe.sh --live
AUTH=""
if [[ -n "${STRIPE_API_KEY:-}" ]]; then
  AUTH="--api-key $STRIPE_API_KEY"
elif [[ -n "$MODE_FLAG" ]]; then
  AUTH="$MODE_FLAG"
fi

# `stripe config --list` exits 0 even when not logged in, so we probe
# via a cheap API call instead.
if ! stripe products list --limit 1 $AUTH >/dev/null 2>&1; then
  echo "✗ stripe CLI auth failed for ${MODE_FLAG:-test} mode." >&2
  echo "  Either: stripe login ${MODE_FLAG}" >&2
  echo "  Or pass a full secret key: STRIPE_API_KEY=sk_live_… $0 ${MODE_FLAG}" >&2
  exit 1
fi

if [[ "$MODE_FLAG" == "--live" ]]; then
  echo "⚠  LIVE MODE. Real products + prices will be created in your Stripe account."
  read -p "Type 'yes' to continue: " confirm
  [[ "$confirm" == "yes" ]] || { echo "Aborted."; exit 1; }
fi

echo "▸ Mode:       ${MODE_FLAG:-test}"
echo "▸ Env file:   $ENV_FILE"
echo

# ──────────────────────────────────────────────────────────────────────
# Plan catalog — cents MUST match plans.price_monthly in the DB
# (see 000040_seed_plans_v2.up.sql) so the Stripe charge equals the price
# shown on the site. Format: slug:cents:name:env_var
# ──────────────────────────────────────────────────────────────────────

PLANS=(
  "us_essentiel:799:SafePhone Essential (US):STRIPE_PRICE_ESSENTIEL"
  "us_ecran_plus:1499:SafePhone Screen+ (US):STRIPE_PRICE_ECRAN_PLUS"
  "us_plus:1999:SafePhone Plus (US):STRIPE_PRICE_PLUS"
  "us_premium:2999:SafePhone Premium (US):STRIPE_PRICE_PREMIUM"
  "us_total:4499:SafePhone Total (US):STRIPE_PRICE_TOTAL"
)

declare -a NEW_ENV_LINES=()

for entry in "${PLANS[@]}"; do
  IFS=: read -r SLUG CENTS NAME VAR <<< "$entry"
  printf "▸ %-15s  $%-6s  %s ... " "$SLUG" "$(awk -v c=$CENTS 'BEGIN{printf "%.2f", c/100}')" "$NAME"

  PRODUCT_JSON=$(stripe products create $AUTH \
    --name="$NAME" \
    -d "metadata[safephone_slug]=$SLUG" \
    -d "metadata[safephone_market]=US")

  PRODUCT_ID=$(echo "$PRODUCT_JSON" | jq -r '.id')
  if [[ -z "$PRODUCT_ID" || "$PRODUCT_ID" == "null" ]]; then
    echo
    echo "✗ Failed to create product for $SLUG. Response:" >&2
    echo "$PRODUCT_JSON" >&2
    exit 1
  fi

  PRICE_JSON=$(stripe prices create $AUTH \
    --product="$PRODUCT_ID" \
    --unit-amount="$CENTS" \
    --currency=usd \
    -d "recurring[interval]=month" \
    -d "metadata[safephone_slug]=$SLUG")

  PRICE_ID=$(echo "$PRICE_JSON" | jq -r '.id')
  if [[ -z "$PRICE_ID" || "$PRICE_ID" == "null" ]]; then
    echo
    echo "✗ Failed to create price for $SLUG. Response:" >&2
    echo "$PRICE_JSON" >&2
    exit 1
  fi

  echo "✓ $PRICE_ID"
  NEW_ENV_LINES+=("$VAR=$PRICE_ID")
done

# ──────────────────────────────────────────────────────────────────────
# Upsert into .env
# ──────────────────────────────────────────────────────────────────────

echo
echo "▸ Updating $ENV_FILE"

if [[ ! -f "$ENV_FILE" ]]; then
  touch "$ENV_FILE"
fi

# Backup once before mutating.
BACKUP="${ENV_FILE}.bak.$(date +%Y%m%d-%H%M%S)"
cp "$ENV_FILE" "$BACKUP"
echo "  (backup: $BACKUP)"

TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

# Strip any existing STRIPE_PRICE_* lines we're about to rewrite.
grep -v -E "^(STRIPE_PRICE_ESSENTIEL|STRIPE_PRICE_ECRAN_PLUS|STRIPE_PRICE_PLUS|STRIPE_PRICE_PREMIUM|STRIPE_PRICE_TOTAL)=" "$ENV_FILE" > "$TMP" || true

# Ensure trailing newline before appending.
if [[ -s "$TMP" ]] && [[ "$(tail -c 1 "$TMP")" != "" ]]; then
  echo "" >> "$TMP"
fi

{
  echo "# Stripe US price IDs — generated $(date +%Y-%m-%d) by setup-stripe.sh"
  for line in "${NEW_ENV_LINES[@]}"; do
    echo "$line"
  done
} >> "$TMP"

mv "$TMP" "$ENV_FILE"
trap - EXIT

echo
echo "Created $(echo "${#PLANS[@]}") products + monthly prices."
echo
echo "Still missing in $ENV_FILE (set these manually):"
grep -q "^STRIPE_SECRET_KEY=" "$ENV_FILE" || echo "  • STRIPE_SECRET_KEY=sk_test_...   (from dashboard.stripe.com/test/apikeys)"
grep -q "^STRIPE_WEBHOOK_SECRET=" "$ENV_FILE" || echo "  • STRIPE_WEBHOOK_SECRET=whsec_... (from \`stripe listen\`)"
echo
echo "Next: in another terminal, run"
echo "  stripe listen --forward-to localhost:8080/api/v1/webhooks/stripe"
echo "Copy the whsec_… it prints into STRIPE_WEBHOOK_SECRET, then restart the backend."
