#!/bin/bash
set -e

# Deployment Script for Multi-Agent CRA System
# Usage: ./DEPLOY.sh [cloudrun]

TARGET=${1:-cloudrun}
PROJECT_ID=${PROJECT_ID:-$(gcloud config get-value project)}
REGION=${REGION:-us-central1}
IMAGE_REPO="gcr.io/${PROJECT_ID}/multi-agent-cra"
TAG="latest"
SERVICE_ACCOUNT="cra-cloudrun-invoker@${PROJECT_ID}.iam.gserviceaccount.com"

if [ -z "$PROJECT_ID" ]; then
    echo "Error: PROJECT_ID is not set."
    exit 1
fi

echo "🚀 Deploying to target: $TARGET"
echo "   Project: $PROJECT_ID"
echo "   Region:  $REGION"

# 1. Build Containers
echo "📦 Building and Pushing Containers..."
gcloud builds submit --tag "${IMAGE_REPO}/server:${TAG}" -f Dockerfile --target server .
gcloud builds submit --tag "${IMAGE_REPO}/worker:${TAG}" -f Dockerfile --target worker .

# 2. Setup Secure OIDC & Service Accounts
echo "🔒 Configuring Security..."
# Create invoker SA if not exists
if ! gcloud iam service-accounts describe "$SERVICE_ACCOUNT" --project "$PROJECT_ID" >/dev/null 2>&1; then
    gcloud iam service-accounts create cra-cloudrun-invoker --display-name "Cloud Run Invoker"
fi

# 3. Application Deployment
if [ "$TARGET" == "cloudrun" ]; then
    echo "🚀 Deploying to Cloud Run..."
    
    # Deploy Server (Authenticated)
    gcloud run deploy cra-server \
        --image "${IMAGE_REPO}/server:${TAG}" \
        --region "$REGION" \
        --no-allow-unauthenticated \
        --service-account "$SERVICE_ACCOUNT" \
        --set-env-vars="PROJECT_ID=${PROJECT_ID},GEMINI_API_KEY=${GEMINI_API_KEY},LOG_LEVEL=INFO" \
        --set-secrets="GEMINI_API_KEY=gemini-api-key:latest"

    # Grant Invoker Permission (Secure OIDC)
    # Allows the deployed service account to invoke itself (or specific users)
    gcloud run services add-iam-policy-binding cra-server \
        --region "$REGION" \
        --member="serviceAccount:${SERVICE_ACCOUNT}" \
        --role="roles/run.invoker"

    # Deploy Worker (Background)
    gcloud run deploy cra-worker \
        --image "${IMAGE_REPO}/worker:${TAG}" \
        --region "$REGION" \
        --no-allow-unauthenticated \
        --service-account "$SERVICE_ACCOUNT" \
        --set-env-vars="PROJECT_ID=${PROJECT_ID},GEMINI_API_KEY=${GEMINI_API_KEY},LOG_LEVEL=INFO" \
        --set-secrets="GEMINI_API_KEY=gemini-api-key:latest" \
        --min-instances=1

elif [ "$TARGET" == "gke" ]; then
    echo "⚠️  For GKE deployment, please use Terraform:"
    echo "   cd terraform && terraform apply"
fi

echo "✅ Deployment Finished!"
echo "   Run 'gcloud run services list' to see your endpoints."