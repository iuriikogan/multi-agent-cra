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
  gcloud run services delete cra-dashboard --region="$REGION" --quiet || true

  echo "Deleting Pub/Sub subscriptions and topics..."
  SUBS=("scan-requests-sub" "aggregator-tasks-sub" "modeler-tasks-sub" "validator-tasks-sub" "reviewer-tasks-sub" "tagger-tasks-sub" "monitoring-events-sub")
  TOPICS=("scan-requests" "aggregator-tasks" "modeler-tasks" "validator-tasks" "reviewer-tasks" "tagger-tasks" "monitoring-events")
  
  for SUB in "${SUBS[@]}"; do
    gcloud pubsub subscriptions delete "$SUB" --quiet || true
  done

  for TOPIC in "${TOPICS[@]}"; do
    gcloud pubsub topics delete "$TOPIC" --quiet || true
  done
  
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
TOPICS=("scan-requests" "aggregator-tasks" "modeler-tasks" "validator-tasks" "reviewer-tasks" "tagger-tasks" "monitoring-events")
SUBS=("scan-requests-sub" "aggregator-tasks-sub" "modeler-tasks-sub" "validator-tasks-sub" "reviewer-tasks-sub" "tagger-tasks-sub" "monitoring-events-sub")

for TOPIC in "${TOPICS[@]}"; do
  if ! gcloud pubsub topics describe "$TOPIC" &>/dev/null; then
    echo "Creating Pub/Sub topic: $TOPIC..."
    gcloud pubsub topics create "$TOPIC"
  else
    echo "Pub/Sub topic '$TOPIC' already exists."
  fi
done

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

echo "Granting Cloud Asset Viewer role to Compute SA (required for list_gcp_assets tool)..."
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$COMPUTE_SA" \
  --role="roles/cloudasset.viewer" \
  --quiet

# --- 7. Trigger Cloud Build ---
echo "Starting Cloud Build to build and deploy services..."
gcloud builds submit --config=cloudbuild.yaml --substitutions=_REGION="$REGION" .

# --- 8. Setup Pub/Sub Push Subscriptions ---
echo "Getting Worker Service URL..."
WORKER_URL=$(gcloud run services describe cra-worker --region="$REGION" --format="value(status.url)")

if [ -z "$WORKER_URL" ]; then
  echo "Error: Could not retrieve cra-worker URL. Deploy may have failed."
  exit 1
fi

echo "Worker URL is: $WORKER_URL"

for i in "${!SUBS[@]}"; do
  SUB="${SUBS[$i]}"
  TOPIC="${TOPICS[$i]}"
  
  # For the push endpoint, we append the topic name to match the Mux handlers
  # EXCEPT for the topic "scan-requests", wait--
  # In worker.go, the routes are:
  # /pubsub/aggregator
  # /pubsub/modeler
  # /pubsub/validator
  # /pubsub/reviewer
  # /pubsub/tagger
  # /pubsub/scan-requests
  # Let's map TOPICS to endpoints manually or standardize them.
  # The topics are: "scan-requests" "aggregator-tasks" "modeler-tasks" "validator-tasks" "reviewer-tasks" "tagger-tasks" "monitoring-events"
  ENDPOINT_PATH=""
  case "$TOPIC" in
    "scan-requests") ENDPOINT_PATH="/pubsub/scan-requests" ;;
    "aggregator-tasks") ENDPOINT_PATH="/pubsub/aggregator" ;;
    "modeler-tasks") ENDPOINT_PATH="/pubsub/modeler" ;;
    "validator-tasks") ENDPOINT_PATH="/pubsub/validator" ;;
    "reviewer-tasks") ENDPOINT_PATH="/pubsub/reviewer" ;;
    "tagger-tasks") ENDPOINT_PATH="/pubsub/tagger" ;;
    "monitoring-events") 
       # Monitoring is broadcast to SSE via the server, worker doesn't consume it.
       # The Server actually consumed this in the previous Pull model!
       # We should not create a worker Push sub for monitoring.
       continue ;;
  esac

  PUSH_ENDPOINT="${WORKER_URL}${ENDPOINT_PATH}"

  if ! gcloud pubsub subscriptions describe "$SUB" &>/dev/null; then
    echo "Creating Pub/Sub Push subscription: $SUB attached to $TOPIC -> $PUSH_ENDPOINT"
    gcloud pubsub subscriptions create "$SUB" --topic="$TOPIC" --push-endpoint="$PUSH_ENDPOINT"
  else
    echo "Updating Pub/Sub Push subscription: $SUB to $PUSH_ENDPOINT"
    gcloud pubsub subscriptions update "$SUB" --push-endpoint="$PUSH_ENDPOINT"
  fi
done

echo "Deployment complete! Event-Driven Push Services are active."