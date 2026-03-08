#!/bin/bash
set -e

PROJECT_ID=$(gcloud config get-value project)
echo "Fixing Terraform State for Project: $PROJECT_ID"

# 1. Pub/Sub Topics
for topic in "scan-requests" "assets-found" "models-generated" "validation-results" "final-reports"; do
    if gcloud pubsub topics describe "$topic" --project="$PROJECT_ID" >/dev/null 2>&1; then
        echo "Importing topic: $topic"
        terraform import google_pubsub_topic.$(echo $topic | tr '-' '_') projects/$PROJECT_ID/topics/$topic || true
    fi
done

# 2. Pub/Sub Subscriptions
for topic in "scan-requests" "assets-found" "models-generated" "validation-results" "final-reports"; do
    sub="${topic}-sub"
    if gcloud pubsub subscriptions describe "$sub" --project="$PROJECT_ID" >/dev/null 2>&1; then
        echo "Importing subscription: $sub"
        terraform import google_pubsub_subscription.$(echo $sub | tr '-' '_') projects/$PROJECT_ID/subscriptions/$sub || true
    fi
done

# 3. Secret Manager
if gcloud secrets describe gemini-api-key --project="$PROJECT_ID" >/dev/null 2>&1; then
    echo "Importing Secret: gemini-api-key"
    terraform import google_secret_manager_secret.gemini_api_key projects/$PROJECT_ID/secrets/gemini-api-key || true
fi

echo "State import attempt complete. You can now run 'terraform apply'."
