#!/bin/bash
set -e

PROJECT_ID=$(gcloud config get-value project)
REGION=${REGION:-europe-west1}
BUCKET_NAME="tf-state-${PROJECT_ID}"

echo "Configuring Terraform Backend for Project: $PROJECT_ID"

# 1. Create the GCS Bucket if it doesn't exist
if ! gcloud storage buckets describe "gs://${BUCKET_NAME}" &>/dev/null; then
    echo "Creating GCS Bucket: ${BUCKET_NAME}..."
    gcloud storage buckets create "gs://${BUCKET_NAME}" --location="$REGION" --uniform-bucket-level-access
    
    # Enable versioning for state safety
    # gcloud storage buckets update "gs://${BUCKET_NAME}" --versioning
else
    echo "Bucket ${BUCKET_NAME} already exists."
fi

# 2. Initialize Terraform with the bucket
echo "Initializing Terraform with backend..."
terraform init -backend-config="bucket=${BUCKET_NAME}" -migrate-state
