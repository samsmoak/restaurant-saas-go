#!/usr/bin/env bash
# Interactive helper — prompts for each secret value and adds a version.
# Run after bootstrap.sh, before deploy.sh.
#
# Usage:
#   export GCP_PROJECT=my-project
#   bash deploy/set-secrets.sh

set -euo pipefail

: "${GCP_PROJECT:?GCP_PROJECT env var is required}"

if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" 2>/dev/null | grep -q '.'; then
  echo "gcloud is not authenticated. Run: gcloud auth login" >&2
  exit 1
fi

gcloud config set project "$GCP_PROJECT"

set_secret() {
  local name="$1"
  local prompt="$2"
  local sensitive="${3:-yes}"
  local value

  if [[ "$sensitive" == "yes" ]]; then
    read -rs -p "$prompt: " value
    echo ""
  else
    read -r -p "$prompt: " value
  fi
  if [[ -z "$value" ]]; then
    echo "  (skipped $name — empty input)"
    return
  fi
  printf '%s' "$value" | gcloud secrets versions add "$name" --data-file=- >/dev/null
  echo "  set $name"
}

echo "==> setting secrets (leave blank to skip one)"
set_secret MONGO_URI                "MongoDB Atlas SRV URI"
set_secret JWT_SECRET                "JWT signing secret (hex, 64+ chars)"
set_secret STRIPE_SECRET_KEY         "Stripe secret key (sk_…)"
set_secret STRIPE_WEBHOOK_SECRET     "Stripe webhook secret (whsec_…)"
set_secret GOOGLE_OAUTH_CLIENT_ID    "Google OAuth web client ID" no
set_secret AWS_S3_BUCKET             "S3 bucket name" no
set_secret AWS_ACCESS_KEY_ID         "AWS access key id"
set_secret AWS_SECRET_ACCESS_KEY     "AWS secret access key"
set_secret S3_PUBLIC_BASE_URL        "Public CDN base URL (or blank)" no
set_secret ADMIN_LEGACY_INVITE_CODE  "Bootstrap admin invite code (optional)"
set_secret ADMIN_LEGACY_RESTAURANT_SLUG "Bootstrap restaurant slug for legacy invite (optional)" no

echo "done."
