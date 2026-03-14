#!/bin/bash
set -ou -pipefail

# --- Configuration ---
PROJECT_ID=$(gcloud config get-value project)
PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')
REGION=$(gcloud config get-value compute/region 2>/dev/null)
REGION=${REGION:-europe-west1}
REPO_NAME="multi-agent-cra"

echo "Using Project: $PROJECT_ID ($PROJECT_NUMBER)"
echo "Using Region: $REGION"

# --- Configuration Defaults ---
DESTROY=0

# --- Parse Arguments ---
while [[ "$#" -gt 0 ]]; do
  case $1 in
    --destroy|-d) DESTROY=1; shift ;;
    *) echo "Unknown parameter: $1"; exit 1 ;;
  esac
done

# --- Handle Destroy Flag ---
if [[ "$DESTROY" == "1" ]]; then
  echo "Destroy flag detected. Tearing down resources..."
  # Tearing down services manually as requested for the script path
  gcloud run services delete cra-server --region="$REGION" --quiet || true
  gcloud run services delete cra-worker --region="$REGION" --quiet || true
  echo "Resource teardown complete."
  exit 0
fi

# --- 1. Enable Required Services ---
echo "Enabling core GCP services..."
gcloud services enable \
  artifactregistry.googleapis.com \
  cloudbuild.googleapis.com \
  run.googleapis.com \
  secretmanager.googleapis.com \
  sqladmin.googleapis.com \
  compute.googleapis.com \
  servicenetworking.googleapis.com \
  pubsub.googleapis.com

# --- 2. Setup Bootstrapping Secret ---
if ! gcloud secrets describe GEMINI_API_KEY &>/dev/null; then
  echo "GEMINI_API_KEY secret not found."
  if [ -z "${GEMINI_API_KEY:-}" ]; then
     echo "Error: GEMINI_API_KEY environment variable is not set."
     exit 1
  fi
  printf "%s" "$GEMINI_API_KEY" | gcloud secrets create GEMINI_API_KEY --data-file=-
fi

# --- 3. Setup Artifact Registry ---
if ! gcloud artifacts repositories describe "$REPO_NAME" --location="$REGION" &>/dev/null; then
  echo "Creating Artifact Registry repository: $REPO_NAME..."
  gcloud artifacts repositories create "$REPO_NAME" \
    --repository-format=docker \
    --location="$REGION"
fi

# --- 4. Fetch Cloud SQL IP (if instance exists) ---
echo "Fetching Cloud SQL private IP..."
DB_PRIVATE_IP=$(gcloud sql instances describe cra-db-instance --format='value(ipAddresses.filter(type=PRIVATE).ipAddress)' 2>/dev/null || echo "10.0.0.3")
echo "Using DB IP: $DB_PRIVATE_IP"

# --- 5. Build and Deploy via Cloud Build ---
echo "Triggering full build and deployment via Cloud Build..."
gcloud builds submit --config=cloudbuild.yaml \
  --substitutions=_REGION="$REGION",_REPO_NAME="$REPO_NAME",_DB_IP="$DB_PRIVATE_IP" .

echo "Deployment complete! Your Multi-Agent CRA system is live."
