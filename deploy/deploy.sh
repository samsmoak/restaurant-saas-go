#!/usr/bin/env bash
# Submit a Cloud Build from the current working tree.
#
# Usage:
#   export GCP_PROJECT=my-project
#   export GCP_REGION=us-central1             # optional
#   export SERVICE=restaurant-saas-api        # optional
#   export AR_REPO=restaurant-saas            # optional
#   bash deploy/deploy.sh

set -euo pipefail

: "${GCP_PROJECT:?GCP_PROJECT env var is required}"
GCP_REGION="${GCP_REGION:-us-central1}"
SERVICE="${SERVICE:-restaurant-saas-api}"
AR_REPO="${AR_REPO:-restaurant-saas}"

# Preflight: require an authenticated gcloud user.
if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" 2>/dev/null | grep -q '.'; then
  echo "gcloud is not authenticated. Run: gcloud auth login" >&2
  exit 1
fi

gcloud config set project "$GCP_PROJECT"

echo "==> submitting build for $GCP_PROJECT ($GCP_REGION) → Cloud Run service $SERVICE"
gcloud builds submit \
  --config=cloudbuild.yaml \
  --substitutions=_REGION="$GCP_REGION",_SERVICE="$SERVICE",_AR_REPO="$AR_REPO" \
  .

echo "==> service URL:"
gcloud run services describe "$SERVICE" --region="$GCP_REGION" --format='value(status.url)'
