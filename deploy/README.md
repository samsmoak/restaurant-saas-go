# Cloud Run deployment

## 0. Install and authenticate `gcloud`

```bash
# Install (macOS)
brew install --cask google-cloud-sdk

# Log in to the Google account that owns (or has Owner role on) your GCP project.
gcloud auth login

# Application Default Credentials — lets any SDK on your machine auth as you.
gcloud auth application-default login

# Sanity check — both should print your email.
gcloud auth list
```

If you don't have a GCP project yet: create one at https://console.cloud.google.com/projectcreate and make sure billing is enabled on it (Cloud Run, Cloud Build, and Artifact Registry all require billing).

Every script below does a preflight check and exits if `gcloud` isn't authenticated.

## 1. Three scripts, run in order

```bash
export GCP_PROJECT=my-project          # required
export GCP_REGION=us-central1          # optional, default us-central1
export SERVICE=restaurant-saas-api     # optional
export AR_REPO=restaurant-saas         # optional

bash deploy/bootstrap.sh       # enable APIs, create AR repo, create empty secrets, grant IAM
bash deploy/set-secrets.sh     # interactive: fill in each secret value
bash deploy/deploy.sh          # gcloud builds submit → cloudbuild.yaml → Cloud Run
```

## Prerequisites

- Authenticated `gcloud` (step 0 above).
- A MongoDB Atlas cluster URL that the Cloud Run region can reach. Atlas serverless works; the connection string goes into `MONGO_URI`.
- Stripe + Google OAuth + AWS S3 credentials ready (or leave their secrets empty to run in degraded mode).

### Why no local Docker auth?

We build via Cloud Build, which runs inside GCP and already has its own service-account credentials for pushing to Artifact Registry. You don't need `gcloud auth configure-docker` or even Docker installed locally.

## Secrets used

All of the following are mounted from Secret Manager into the Cloud Run service by `cloudbuild.yaml`. `bootstrap.sh` creates them empty; `set-secrets.sh` adds versions.

| Secret | What it is |
|--------|-----------|
| `MONGO_URI` | `mongodb+srv://user:pass@cluster/…` |
| `JWT_SECRET` | random 48+ hex chars (`openssl rand -hex 48`) |
| `STRIPE_SECRET_KEY` | `sk_live_…` or `sk_test_…` |
| `STRIPE_WEBHOOK_SECRET` | `whsec_…` from the Stripe dashboard |
| `GOOGLE_OAUTH_CLIENT_ID` | web OAuth client used by the customer app |
| `AWS_S3_BUCKET`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `S3_PUBLIC_BASE_URL` | image uploads |
| `ADMIN_LEGACY_INVITE_CODE`, `ADMIN_LEGACY_RESTAURANT_SLUG` | optional bootstrap pair to promote the first admin |

Non-secret env vars (`MONGO_DB_NAME`, `AWS_S3_REGION`, `APP_URL`, `CORS_ALLOWED_ORIGINS`) are set inline in `cloudbuild.yaml`.

## Post-deploy checks

```bash
URL=$(gcloud run services describe "$SERVICE" --region="$GCP_REGION" --format='value(status.url)')
curl -fsS "$URL/health" | jq
```

Expected: `{"status":"ok","mongo":"ok", ...}`.

## Re-deploy

Each deploy bumps the image to `…/$SERVICE:$SHORT_SHA` and also retags `:latest`. Roll back with:

```bash
gcloud run services update-traffic "$SERVICE" --region="$GCP_REGION" --to-revisions=REV=100
```

## Point the frontends at this URL

In both `restaurant-admin/.env.local` and `restaurant-customer/.env.local`:

```
NEXT_PUBLIC_API_URL=<service url from above>
```

The customer app additionally needs `NEXT_PUBLIC_RESTAURANT_SLUG` pointing at the slug of the restaurant it serves.
