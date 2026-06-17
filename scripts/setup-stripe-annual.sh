#!/usr/bin/env bash
#
# setup-stripe-annual.sh — add ANNUAL (yearly) prices to the US market.
#
# Companion to setup-stripe.sh, which created the products + monthly prices.
# This finds each existing US product (by its safephone_slug metadata) and
# attaches a yearly price at monthly × 10 (2 months free), matching
# plans.price_annual in the DB. It then writes the STRIPE_PRICE_*_ANNUAL keys
# into the backend .env file and prints them for pasting into Railway.
#
# Idempotent: if a product already has an active yearly price at the right
# amount, that price is reused instead of creating a duplicate.
#
# Usage:
#   ./scripts/setup-stripe-annual.sh                       # test mode
#   STRIPE_API_KEY=sk_live_… ./scripts/setup-stripe-annual.sh --live
#   ./scripts/setup-stripe-annual.sh --env path            # custom .env path

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
      sed -n '2,18p' "$0"; exit 0 ;;
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
  echo "✗ stripe CLI not found. Install: brew install stripe/stripe-cli/stripe" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "✗ jq not found. Install: brew install jq" >&2
  exit 1
fi

# Auth appended to every stripe call. A full secret key in STRIPE_API_KEY wins:
# `stripe login` stores a restricted key (rk_live_…) that lacks product/price
# write scopes in live mode (403s on create). Pass a full sk_live_… instead:
#   STRIPE_API_KEY=sk_live_… ./scripts/setup-stripe-annual.sh --live
AUTH=""
if [[ -n "${STRIPE_API_KEY:-}" ]]; then
  AUTH="--api-key $STRIPE_API_KEY"
elif [[ -n "$MODE_FLAG" ]]; then
  AUTH="$MODE_FLAG"
fi

if ! stripe products list --limit 1 $AUTH >/dev/null 2>&1; then
  echo "✗ stripe CLI auth failed for ${MODE_FLAG:-test} mode." >&2
  echo "  Either: stripe login ${MODE_FLAG}" >&2
  echo "  Or pass a full secret key: STRIPE_API_KEY=sk_live_… $0 ${MODE_FLAG}" >&2
  exit 1
fi

if [[ "$MODE_FLAG" == "--live" ]]; then
  echo "⚠  LIVE MODE. Real yearly prices will be created in your Stripe account."
  read -p "Type 'yes' to continue: " confirm
  [[ "$confirm" == "yes" ]] || { echo "Aborted."; exit 1; }
fi

echo "▸ Mode:       ${MODE_FLAG:-test}"
echo "▸ Env file:   $ENV_FILE"
echo

# ──────────────────────────────────────────────────────────────────────
# Annual catalog — cents = monthly × 10 (2 months free). MUST match
# plans.price_annual in the DB (see 000040_seed_plans_v2 + market = US).
# Format: slug:annual_cents:env_var
# ──────────────────────────────────────────────────────────────────────

PLANS=(
  "us_essentiel:7990:STRIPE_PRICE_ESSENTIEL_ANNUAL"
  "us_ecran_plus:14990:STRIPE_PRICE_ECRAN_PLUS_ANNUAL"
  "us_plus:19990:STRIPE_PRICE_PLUS_ANNUAL"
  "us_premium:29990:STRIPE_PRICE_PREMIUM_ANNUAL"
  "us_total:44990:STRIPE_PRICE_TOTAL_ANNUAL"
)

# Fetch all products once and resolve product IDs by safephone_slug metadata.
ALL_PRODUCTS=$(stripe products list --limit 100 $AUTH)

declare -a NEW_ENV_LINES=()

for entry in "${PLANS[@]}"; do
  IFS=: read -r SLUG CENTS VAR <<< "$entry"
  printf "▸ %-15s  \$%-8s/yr ... " "$SLUG" "$(awk -v c=$CENTS 'BEGIN{printf "%.2f", c/100}')"

  PRODUCT_ID=$(echo "$ALL_PRODUCTS" | jq -r --arg s "$SLUG" \
    '[.data[] | select(.metadata.safephone_slug==$s) | .id] | first // empty')
  if [[ -z "$PRODUCT_ID" ]]; then
    echo
    echo "✗ No existing product with metadata safephone_slug=$SLUG." >&2
    echo "  Run setup-stripe.sh first (it creates the products + monthly prices)." >&2
    exit 1
  fi

  # Reuse an existing active yearly price at the right amount if present,
  # so re-running doesn't pile up duplicate prices.
  EXISTING=$(stripe prices list --product="$PRODUCT_ID" --active=true --limit=100 $AUTH \
    | jq -r --argjson amt "$CENTS" \
        '[.data[] | select(.recurring.interval=="year" and .unit_amount==$amt and .currency=="usd") | .id] | first // empty')

  if [[ -n "$EXISTING" ]]; then
    PRICE_ID="$EXISTING"
    echo "✓ $PRICE_ID (reused)"
  else
    PRICE_JSON=$(stripe prices create $AUTH \
      --product="$PRODUCT_ID" \
      --unit-amount="$CENTS" \
      --currency=usd \
      -d "recurring[interval]=year" \
      -d "metadata[safephone_slug]=$SLUG" \
      -d "metadata[safephone_cycle]=annual")
    PRICE_ID=$(echo "$PRICE_JSON" | jq -r '.id')
    if [[ -z "$PRICE_ID" || "$PRICE_ID" == "null" ]]; then
      echo
      echo "✗ Failed to create yearly price for $SLUG. Response:" >&2
      echo "$PRICE_JSON" >&2
      exit 1
    fi
    echo "✓ $PRICE_ID (created)"
  fi
  NEW_ENV_LINES+=("$VAR=$PRICE_ID")
done

# ──────────────────────────────────────────────────────────────────────
# Upsert into .env
# ──────────────────────────────────────────────────────────────────────

echo
echo "▸ Updating $ENV_FILE"
[[ -f "$ENV_FILE" ]] || touch "$ENV_FILE"

BACKUP="${ENV_FILE}.bak.$(date +%Y%m%d-%H%M%S)"
cp "$ENV_FILE" "$BACKUP"
echo "  (backup: $BACKUP)"

TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

# Strip any existing annual price lines we're about to rewrite.
grep -v -E "^(STRIPE_PRICE_ESSENTIEL_ANNUAL|STRIPE_PRICE_ECRAN_PLUS_ANNUAL|STRIPE_PRICE_PLUS_ANNUAL|STRIPE_PRICE_PREMIUM_ANNUAL|STRIPE_PRICE_TOTAL_ANNUAL)=" "$ENV_FILE" > "$TMP" || true

if [[ -s "$TMP" ]] && [[ "$(tail -c 1 "$TMP")" != "" ]]; then
  echo "" >> "$TMP"
fi

{
  echo "# Stripe US ANNUAL price IDs — generated $(date +%Y-%m-%d) by setup-stripe-annual.sh"
  for line in "${NEW_ENV_LINES[@]}"; do
    echo "$line"
  done
} >> "$TMP"

mv "$TMP" "$ENV_FILE"
trap - EXIT

echo
echo "Done — ${#PLANS[@]} annual prices ready."
echo
echo "Add these to Railway (backend service → Variables), then redeploy:"
for line in "${NEW_ENV_LINES[@]}"; do
  echo "  $line"
done
