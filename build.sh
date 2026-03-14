#!/bin/bash
set -e

# Configuration
PROJECT_ID=$(gcloud config get-value project)

echo "Triggering Option A: Cloud Build (Automated Deployment)"
echo "Using Project ID: $PROJECT_ID"

# Fetch dynamic values for Cloud Build substitutions if needed
DB_IP=$(gcloud sql instances describe cra-db-instance --format='value(ipAddresses[0].ipAddress)' 2>/dev/null || echo "10.0.0.3")

# Submit Cloud Build
gcloud builds submit --project=$PROJECT_ID \
  --substitutions=_DB_IP=$DB_IP
