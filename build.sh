#!/bin/bash
set -x

# --- Configuration ---
PROJECT_ID=$(gcloud config get-value project)
PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')
# Use region from gcloud config or default to us-central1
REGION=$(gcloud config get-value compute/region 2>/dev/null)
REGION=${REGION:-europe-west1}
REPO_NAME="multi-agent-cra"
COMPUTE_SA="${PROJECT_NUMBER}-compute@developer.gserviceaccount.com"
BUCKET_NAME="cra-data-${PROJECT_ID}"

echo "Using Project: $PROJECT_ID ($PROJECT_NUMBER)"
echo "Using Region: $REGION"

# --- Handle Destroy Flag ---
if [[ "$1" == "--destroy" || "$1" == "-d" ]]; then
  echo "Destroy flag detected. Tearing down resources..."

  echo "Deleting Cloud Run services..."
  gcloud run services delete cra-server --region="$REGION" --quiet || true
  gcloud run services delete cra-worker --region="$REGION" --quiet || true

  echo "Deleting Pub/Sub subscription and topic..."
  gcloud pubsub subscriptions delete scan-requests-sub --quiet || true
  gcloud pubsub topics delete scan-requests --quiet || true

  echo "Deleting GCS Bucket..."
  gcloud storage rm -r "gs://${BUCKET_NAME}" --quiet || true

  echo "Deleting Secret Manager secret..."
  gcloud secrets delete GEMINI_API_KEY --quiet || true

  echo "Deleting Artifact Registry repository..."
  gcloud artifacts repositories delete "$REPO_NAME" --location="$REGION" --quiet || true

  echo "Resource teardown complete."
  exit 0
fi

# --- 1. Enable Required Services ---
echo "Enabling GCP services..."
gcloud services enable \
  artifactregistry.googleapis.com \
  cloudbuild.googleapis.com \
  run.googleapis.com \
  secretmanager.googleapis.com \
  pubsub.googleapis.com

# --- 2. Setup Artifact Registry ---
if ! gcloud artifacts repositories describe "$REPO_NAME" --location="$REGION" &>/dev/null; then
  echo "Creating Artifact Registry repository: $REPO_NAME..."
  gcloud artifacts repositories create "$REPO_NAME" \
    --repository-format=docker \
    --location="$REGION" \
    --description="Docker repository for Multi-Agent CRA"
else
  echo "Artifact Registry repository already exists."
fi

# --- 3. Setup Secret Manager ---
if ! gcloud secrets describe GEMINI_API_KEY &>/dev/null; then
  echo "GEMINI_API_KEY secret not found."
  
  if [ -z "$GEMINI_API_KEY" ]; then
     echo "Error: GEMINI_API_KEY environment variable is not set."
     echo "Please export GEMINI_API_KEY='your-key' and try again."
     exit 1
  fi
  
  echo "Creating GEMINI_API_KEY secret from environment variable..."
  printf "%s" "$GEMINI_API_KEY" | gcloud secrets create GEMINI_API_KEY --data-file=-
  echo "Secret created."
else
  echo "GEMINI_API_KEY secret already exists."
fi

# Grant access to the Compute SA
echo "Ensuring Compute SA has access to GEMINI_API_KEY..."
gcloud secrets add-iam-policy-binding GEMINI_API_KEY \
  --member="serviceAccount:$COMPUTE_SA" \
  --role="roles/secretmanager.secretAccessor" \
  --quiet

# --- 4. Setup GCS Bucket (Data Persistence) ---
if ! gcloud storage buckets describe "gs://${BUCKET_NAME}" &>/dev/null; then
  echo "Creating GCS Bucket: ${BUCKET_NAME}..."
  gcloud storage buckets create "gs://${BUCKET_NAME}" --location="$REGION" --uniform-bucket-level-access
else
  echo "GCS Bucket ${BUCKET_NAME} already exists."
fi
sleep 10
# Grant Object Admin to Compute SA
echo "Granting Storage Object Admin to Compute SA..."
gcloud storage buckets add-iam-policy-binding "gs://${BUCKET_NAME}" \
  --member="serviceAccount:$COMPUTE_SA" \
  --role="roles/storage.objectAdmin" \
  --quiet

# --- 5. Setup Pub/Sub ---
if ! gcloud pubsub topics describe scan-requests &>/dev/null; then
  echo "Creating Pub/Sub topic: scan-requests..."
  gcloud pubsub topics create scan-requests
else
  echo "Pub/Sub topic 'scan-requests' already exists."
fi

if ! gcloud pubsub subscriptions describe scan-requests-sub &>/dev/null; then
  echo "Creating Pub/Sub subscription: scan-requests-sub..."
  gcloud pubsub subscriptions create scan-requests-sub --topic=scan-requests
else
  echo "Pub/Sub subscription 'scan-requests-sub' already exists."
fi

# --- 6. Fix Cloud Build Permissions (Artifact Registry & Logging) ---
# The default compute SA needs writer access to AR and log writer access
echo "Granting Artifact Registry Writer role to Compute SA..."
gcloud artifacts repositories add-iam-policy-binding "$REPO_NAME" \
  --location="$REGION" \
  --member="serviceAccount:$COMPUTE_SA" \
  --role="roles/artifactregistry.writer" \
  --quiet

echo "Granting Logs Writer role to Compute SA..."
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$COMPUTE_SA" \
  --role="roles/logging.logWriter" \
  --quiet

echo "Granting Cloud Run Admin role to Compute SA..."
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$COMPUTE_SA" \
  --role="roles/run.admin" \
  --quiet

echo "Granting Service Account User role to Compute SA (required to deploy as itself)..."
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$COMPUTE_SA" \
  --role="roles/iam.serviceAccountUser" \
  --quiet

echo "Granting Pub/Sub Publisher and Subscriber roles to Compute SA..."
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$COMPUTE_SA" \
  --role="roles/pubsub.publisher" \
  --quiet

gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$COMPUTE_SA" \
  --role="roles/pubsub.subscriber" \
  --quiet

# --- 7. Trigger Cloud Build ---
echo "Starting Cloud Build..."
gcloud builds submit --config=cloudbuild.yaml --substitutions=_REGION="$REGION" .