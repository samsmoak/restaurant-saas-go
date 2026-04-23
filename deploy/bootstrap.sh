#!/usr/bin/env bash
# One-time GCP project bootstrap. Run this BEFORE the first `deploy/deploy.sh`.
#
# What it does:
#   1. Enables the APIs (Run, Build, Artifact Registry, Secret Manager).
#   2. Creates the Artifact Registry repo.
#   3. Creates all Secret Manager secrets (empty) so set-secrets works.
#
# Usage:
#   export GCP_PROJECT=my-project
#   export GCP_REGION=us-central1            # optional; default us-central1
#   export AR_REPO=restaurant-saas           # optional
#   bash deploy/bootstrap.sh

set -euo pipefail

: "${GCP_PROJECT:?GCP_PROJECT env var is required}"
GCP_REGION="${GCP_REGION:-us-central1}"
AR_REPO="${AR_REPO:-restaurant-saas}"

# Preflight: require an authenticated gcloud user.
if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" 2>/dev/null | grep -q '.'; then
  cat <<'EOF' >&2
No active gcloud credentials. Authenticate first:

  gcloud auth login
  gcloud auth application-default login

Then rerun this script.
EOF
  exit 1
fi

gcloud config set project "$GCP_PROJECT"

echo "==> enabling APIs"
gcloud services enable \
  run.googleapis.com \
  cloudbuild.googleapis.com \
  artifactregistry.googleapis.com \
  secretmanager.googleapis.com

echo "==> creating artifact registry repo ($AR_REPO in $GCP_REGION) if needed"
gcloud artifacts repositories describe "$AR_REPO" --location="$GCP_REGION" >/dev/null 2>&1 || \
  gcloud artifacts repositories create "$AR_REPO" \
    --repository-format=docker \
    --location="$GCP_REGION" \
    --description="Restaurant SaaS backend images"

# Create every secret referenced by cloudbuild.yaml. No values set here.
SECRETS=(
  MONGO_URI
  JWT_SECRET
  STRIPE_SECRET_KEY
  STRIPE_WEBHOOK_SECRET
  GOOGLE_OAUTH_CLIENT_ID
  AWS_S3_BUCKET
  AWS_ACCESS_KEY_ID
  AWS_SECRET_ACCESS_KEY
  S3_PUBLIC_BASE_URL
  ADMIN_LEGACY_INVITE_CODE
  ADMIN_LEGACY_RESTAURANT_SLUG
)

echo "==> creating secrets (empty) in Secret Manager"
for s in "${SECRETS[@]}"; do
  if ! gcloud secrets describe "$s" >/dev/null 2>&1; then
    gcloud secrets create "$s" --replication-policy="automatic"
    echo "   created $s"
  else
    echo "   exists  $s"
  fi
done

# Grant the Cloud Build SA permission to deploy Cloud Run + access secrets.
PROJECT_NUMBER="$(gcloud projects describe "$GCP_PROJECT" --format='value(projectNumber)')"
CB_SA="${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com"
RUN_SA="${PROJECT_NUMBER}-compute@developer.gserviceaccount.com"

echo "==> granting Cloud Build SA roles/run.admin + iam.serviceAccountUser"
gcloud projects add-iam-policy-binding "$GCP_PROJECT" --member="serviceAccount:${CB_SA}" --role="roles/run.admin" >/dev/null
gcloud projects add-iam-policy-binding "$GCP_PROJECT" --member="serviceAccount:${CB_SA}" --role="roles/iam.serviceAccountUser" >/dev/null

echo "==> granting runtime SA secret accessor on each secret"
for s in "${SECRETS[@]}"; do
  gcloud secrets add-iam-policy-binding "$s" \
    --member="serviceAccount:${RUN_SA}" \
    --role="roles/secretmanager.secretAccessor" >/dev/null
done

cat <<EOF

Bootstrap complete.

Next step: fill in each secret with real values. Example:
  echo -n "mongodb+srv://user:pass@cluster/..." | gcloud secrets versions add MONGO_URI --data-file=-
  echo -n "\$(openssl rand -hex 48)" | gcloud secrets versions add JWT_SECRET --data-file=-
  echo -n "sk_live_xxx" | gcloud secrets versions add STRIPE_SECRET_KEY --data-file=-
  ...

Then deploy with:
  bash deploy/deploy.sh
EOF
