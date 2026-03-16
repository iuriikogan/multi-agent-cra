#!/bin/bash
set -e

# Configuration
PROJECT_ID=$(gcloud config get-value project)
REGION="europe-west1"
REPO_NAME="multi-agent-compliance"

# Parse arguments
DESTROY=false
while [[ "$#" -gt 0 ]]; do
    case $1 in
        -d|--destroy) DESTROY=true ;;
        *) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
    shift
done

if [ "$DESTROY" = true ]; then
  echo "Triggering Destruction of Resources for Project ID: $PROJECT_ID in Region: $REGION"

  # Cloud Armor
  echo "Deleting Cloud Armor Policies..."
  gcloud compute backend-services update compliance-backend --global --security-policy="" --project=$PROJECT_ID 2>/dev/null || true
  gcloud compute security-policies delete compliance-security-policy --quiet --project=$PROJECT_ID 2>/dev/null || true
  gcloud compute security-policies delete agent-armor-policy --quiet --project=$PROJECT_ID 2>/dev/null || true

  # Load Balancer & Network
  echo "Deleting Forwarding Rule..."
  gcloud compute forwarding-rules delete compliance-frontend-rule --global --quiet --project=$PROJECT_ID 2>/dev/null || true
  
  echo "Deleting Target HTTP Proxy..."
  gcloud compute target-http-proxies delete compliance-http-proxy --quiet --project=$PROJECT_ID 2>/dev/null || true

  echo "Deleting URL Map..."
  gcloud compute url-maps delete compliance-url-map --quiet --project=$PROJECT_ID 2>/dev/null || true

  echo "Deleting Backend Service..."
  gcloud compute backend-services delete compliance-backend --global --quiet --project=$PROJECT_ID 2>/dev/null || true

  echo "Deleting Network Endpoint Group..."
  gcloud compute network-endpoint-groups delete compliance-server-neg --region=$REGION --quiet --project=$PROJECT_ID 2>/dev/null || true

  echo "Deleting Global IP..."
  gcloud compute addresses delete compliance-dashboard-ip --global --quiet --project=$PROJECT_ID 2>/dev/null || true

  # Pub/Sub
  echo "Deleting Pub/Sub Subscription..."
  gcloud pubsub subscriptions delete scan-requests-sub --quiet --project=$PROJECT_ID 2>/dev/null || true

  echo "Deleting Pub/Sub Topics..."
  for topic in scan-requests aggregator-topic modeler-topic validator-topic reviewer-topic tagger-topic reporter-topic monitoring-topic; do
    gcloud pubsub topics delete $topic --quiet --project=$PROJECT_ID 2>/dev/null || true
  done

  # Cloud Run
  echo "Deleting Cloud Run Services..."
  gcloud run services delete compliance-server --region=$REGION --quiet --project=$PROJECT_ID 2>/dev/null || true
  gcloud run services delete compliance-worker --region=$REGION --quiet --project=$PROJECT_ID 2>/dev/null || true

  # Cloud SQL
  echo "Deleting Cloud SQL Instance (this may take a while)..."
  gcloud sql instances delete compliance-mysql-instance --quiet --project=$PROJECT_ID 2>/dev/null || true

  # Networking / VPC
  echo "Deleting Private IP Allocation and Peering..."
  gcloud services vpc-peerings delete --network=compliance-vpc --service=servicenetworking.googleapis.com --project=$PROJECT_ID --quiet 2>/dev/null || true
  gcloud compute addresses delete private-ip-for-sql --global --quiet --project=$PROJECT_ID 2>/dev/null || true

  echo "Deleting VPC Access Connector..."
  gcloud compute networks vpc-access connectors delete compliance-connector --region=$REGION --quiet --project=$PROJECT_ID 2>/dev/null || true

  echo "Deleting VPC Subnet..."
  gcloud compute networks subnets delete compliance-subnet --region=$REGION --quiet --project=$PROJECT_ID 2>/dev/null || true

  echo "Deleting VPC Network..."
  gcloud compute networks delete compliance-vpc --quiet --project=$PROJECT_ID 2>/dev/null || true

  # Secrets & IAM
  echo "Deleting Secret Manager Secret..."
  gcloud secrets delete GEMINI_API_KEY --quiet --project=$PROJECT_ID 2>/dev/null || true

  echo "Deleting Service Accounts..."
  for sa in compliance-server-sa compliance-worker-sa compliance-build-sa sa-classifier sa-auditor sa-vuln sa-reporter; do
    gcloud iam service-accounts delete $sa@$PROJECT_ID.iam.gserviceaccount.com --quiet --project=$PROJECT_ID 2>/dev/null || true
  done

  # Artifact Registry
  echo "Deleting Artifact Registry Repository..."
  gcloud artifacts repositories delete $REPO_NAME --location=$REGION --quiet --project=$PROJECT_ID 2>/dev/null || true
  
  echo "Destruction complete."
  exit 0
fi

echo "Triggering Option A: Cloud Build (Automated Deployment)"
echo "Using Project ID: $PROJECT_ID"
echo "Region: $REGION"

echo "Enabling required APIs..."
gcloud services enable \
    run.googleapis.com \
    iam.googleapis.com \
    cloudresourcemanager.googleapis.com \
    artifactregistry.googleapis.com \
    cloudbuild.googleapis.com \
    sqladmin.googleapis.com \
    servicenetworking.googleapis.com \
    pubsub.googleapis.com \
    secretmanager.googleapis.com \
    vpcaccess.googleapis.com \
    compute.googleapis.com

echo "Ensuring Artifact Registry repository exists..."
if ! gcloud artifacts repositories describe $REPO_NAME --location=$REGION --project=$PROJECT_ID &>/dev/null; then
  gcloud artifacts repositories create $REPO_NAME \
      --repository-format=docker \
      --location=$REGION \
      --description="Docker repository for Multi-Agent Compliance Platform" \
      --project=$PROJECT_ID
fi

echo "Ensuring VPC network exists..."
if ! gcloud compute networks describe compliance-vpc --project=$PROJECT_ID &>/dev/null; then
  gcloud compute networks create compliance-vpc --subnet-mode=custom --project=$PROJECT_ID
fi

echo "Ensuring VPC subnet exists..."
if ! gcloud compute networks subnets describe compliance-subnet --region=$REGION --project=$PROJECT_ID &>/dev/null; then
  gcloud compute networks subnets create compliance-subnet \
      --network=compliance-vpc \
      --range=10.0.0.0/24 \
      --region=$REGION \
      --project=$PROJECT_ID
fi

echo "Ensuring VPC access connector exists..."
if ! gcloud compute networks vpc-access connectors describe compliance-connector --region=$REGION --project=$PROJECT_ID &>/dev/null; then
  gcloud compute networks vpc-access connectors create compliance-connector \
      --network=compliance-vpc \
      --region=$REGION \
      --range=10.8.0.0/28 \
      --project=$PROJECT_ID
fi

echo "Ensuring Service Accounts exist..."
for sa in compliance-server-sa compliance-worker-sa compliance-build-sa sa-classifier sa-auditor sa-vuln sa-reporter; do
  if ! gcloud iam service-accounts describe $sa@$PROJECT_ID.iam.gserviceaccount.com --project=$PROJECT_ID &>/dev/null; then
    gcloud iam service-accounts create $sa \
        --display-name="Service Account for $sa" \
        --project=$PROJECT_ID
  fi
done

echo "Configuring IAM bindings..."
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-server-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor" --condition=None >/dev/null || true
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-server-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/cloudsql.client" --condition=None >/dev/null || true
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-server-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/cloudtrace.agent" --condition=None >/dev/null || true
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-server-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/pubsub.publisher" --condition=None >/dev/null || true

gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-worker-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor" --condition=None >/dev/null || true
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-worker-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/cloudsql.client" --condition=None >/dev/null || true
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-worker-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/aiplatform.user" --condition=None >/dev/null || true
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-worker-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/cloudasset.viewer" --condition=None >/dev/null || true
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-worker-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/cloudtrace.agent" --condition=None >/dev/null || true
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-worker-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/pubsub.publisher" --condition=None >/dev/null || true

gcloud run services add-iam-policy-binding compliance-worker \
    --region=$REGION \
    --member="serviceAccount:compliance-worker-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/run.invoker" \
    --project=$PROJECT_ID >/dev/null || true

for sa in sa-classifier sa-auditor sa-vuln sa-reporter; do
  gcloud projects add-iam-policy-binding $PROJECT_ID \
      --member="serviceAccount:$sa@$PROJECT_ID.iam.gserviceaccount.com" \
      --role="roles/aiplatform.user" --condition=None >/dev/null || true
done

# Cloud Build SA (Least Privilege replacement for Default Compute SA)
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-build-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/logging.logWriter" --condition=None >/dev/null || true
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-build-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/artifactregistry.writer" --condition=None >/dev/null || true
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-build-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/run.admin" --condition=None >/dev/null || true
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-build-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/iam.serviceAccountUser" --condition=None >/dev/null || true
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-build-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor" --condition=None >/dev/null || true
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:compliance-build-sa@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/storage.admin" --condition=None >/dev/null || true

echo "Ensuring Secret Manager secret exists..."
if ! gcloud secrets describe GEMINI_API_KEY --project=$PROJECT_ID &>/dev/null; then
  gcloud secrets create GEMINI_API_KEY --replication-policy=automatic --project=$PROJECT_ID
fi

if ! gcloud secrets versions describe latest --secret=GEMINI_API_KEY --project=$PROJECT_ID &>/dev/null; then
  echo -n "${GEMINI_API_KEY:-dummy_key_for_build}" | gcloud secrets versions add GEMINI_API_KEY --data-file=- --project=$PROJECT_ID
fi

echo "Configuring Private Service Connection..."
if ! gcloud compute addresses describe private-ip-for-sql --global --project=$PROJECT_ID &>/dev/null; then
  gcloud compute addresses create private-ip-for-sql \
      --global \
      --purpose=VPC_PEERING \
      --prefix-length=16 \
      --network=compliance-vpc \
      --project=$PROJECT_ID
fi

if ! gcloud services vpc-peerings describe --network=compliance-vpc --service=servicenetworking.googleapis.com --project=$PROJECT_ID &>/dev/null; then
  gcloud services vpc-peerings connect \
      --service=servicenetworking.googleapis.com \
      --ranges=private-ip-for-sql \
      --network=compliance-vpc \
      --project=$PROJECT_ID
fi

echo "Ensuring Cloud SQL instance exists..."
if ! gcloud sql instances describe compliance-mysql-instance --project=$PROJECT_ID &>/dev/null; then
  gcloud sql instances create compliance-mysql-instance \
      --database-version=MYSQL_8_0 \
      --tier=db-f1-micro \
      --region=$REGION \
      --network=compliance-vpc \
      --no-assign-ip \
      --project=$PROJECT_ID
fi

if ! gcloud sql databases describe compliance_db --instance=compliance-mysql-instance --project=$PROJECT_ID &>/dev/null; then
  gcloud sql databases create compliance_db --instance=compliance-mysql-instance --project=$PROJECT_ID
fi

if ! gcloud sql users describe compliance_user --instance=compliance-mysql-instance --host="%" --project=$PROJECT_ID &>/dev/null; then
  gcloud sql users create compliance_user \
      --instance=compliance-mysql-instance \
      --password="change_me_in_production" \
      --host="%" \
      --project=$PROJECT_ID
fi

echo "Ensuring Pub/Sub topics exist..."
for topic in scan-requests aggregator-topic modeler-topic validator-topic reviewer-topic tagger-topic reporter-topic monitoring-topic; do
  if ! gcloud pubsub topics describe $topic --project=$PROJECT_ID &>/dev/null; then
    gcloud pubsub topics create $topic --project=$PROJECT_ID
  fi
done

# Fetch dynamic values for Cloud Build substitutions
DB_IP=$(gcloud sql instances describe compliance-mysql-instance --format='value(ipAddresses[0].ipAddress)' --project=$PROJECT_ID)

# Submit Cloud Build using Dedicated Service Account
echo "Submitting Cloud Build..."
gcloud beta builds submit --project=$PROJECT_ID \
  --service-account="projects/${PROJECT_ID}/serviceAccounts/compliance-build-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --substitutions=_DB_IP=$DB_IP

# Setup Pub/Sub Subscription for worker
echo "Ensuring Pub/Sub subscription exists..."
if ! gcloud pubsub subscriptions describe scan-requests-sub --project=$PROJECT_ID &>/dev/null; then
  WORKER_URL=$(gcloud run services describe compliance-worker --region=$REGION --format='value(status.url)' --project=$PROJECT_ID)
  if [ -n "$WORKER_URL" ]; then
    gcloud pubsub subscriptions create scan-requests-sub \
        --topic=scan-requests \
        --push-endpoint="${WORKER_URL}/pubsub/scan-requests" \
        --push-auth-service-account=compliance-worker-sa@$PROJECT_ID.iam.gserviceaccount.com \
        --project=$PROJECT_ID
  else
    echo "Warning: Could not fetch worker URL. Subscription scan-requests-sub not created."
  fi
fi

# Load Balancer and Cloud Armor
echo "Ensuring Cloud Armor policy exists..."
if ! gcloud compute security-policies describe compliance-security-policy --project=$PROJECT_ID &>/dev/null; then
  gcloud compute security-policies create compliance-security-policy \
      --description="Basic security policy for Compliance Dashboard" \
      --project=$PROJECT_ID
  
  # Allow my IP
  MY_IP=$(curl -s https://ipv4.icanhazip.com)
  if [ -n "$MY_IP" ]; then
    gcloud compute security-policies rules create 1000 \
        --security-policy=compliance-security-policy \
        --src-ip-ranges="${MY_IP}/32" \
        --action="allow" \
        --description="Allow my IP" \
        --project=$PROJECT_ID
  fi

  # Update default rule to deny
  gcloud compute security-policies rules update 2147483647 \
      --security-policy=compliance-security-policy \
      --action="deny-403" \
      --project=$PROJECT_ID || true
fi

echo "Ensuring Cloud Armor agent policy exists..."
if ! gcloud compute security-policies describe agent-armor-policy --project=$PROJECT_ID &>/dev/null; then
  gcloud compute security-policies create agent-armor-policy \
      --project=$PROJECT_ID
  
  gcloud compute security-policies rules create 1000 \
      --security-policy=agent-armor-policy \
      --src-ip-ranges="*" \
      --action="allow" \
      --description="Allow access" \
      --project=$PROJECT_ID

  gcloud compute security-policies rules create 900 \
      --security-policy=agent-armor-policy \
      --expression="evaluatePreconfiguredExpr('sqli-v33-stable')" \
      --action="deny-403" \
      --description="Block SQL Injection" \
      --project=$PROJECT_ID || true

  gcloud compute security-policies rules create 901 \
      --security-policy=agent-armor-policy \
      --expression="evaluatePreconfiguredExpr('xss-v33-stable')" \
      --action="deny-403" \
      --description="Block XSS" \
      --project=$PROJECT_ID || true

  # Default deny all
  gcloud compute security-policies rules update 2147483647 \
      --security-policy=agent-armor-policy \
      --action="deny-403" \
      --project=$PROJECT_ID || true
fi

echo "Ensuring Global External Application Load Balancer exists..."
if ! gcloud compute addresses describe compliance-dashboard-ip --global --project=$PROJECT_ID &>/dev/null; then
  gcloud compute addresses create compliance-dashboard-ip --global --project=$PROJECT_ID
fi

if ! gcloud compute network-endpoint-groups describe compliance-server-neg --region=$REGION --project=$PROJECT_ID &>/dev/null; then
  gcloud compute network-endpoint-groups create compliance-server-neg \
      --region=$REGION \
      --network-endpoint-type=serverless \
      --cloud-run-service=compliance-server \
      --project=$PROJECT_ID
fi

if ! gcloud compute backend-services describe compliance-backend --global --project=$PROJECT_ID &>/dev/null; then
  gcloud compute backend-services create compliance-backend \
      --global \
      --protocol=HTTPS \
      --port-name=http \
      --load-balancing-scheme=EXTERNAL_MANAGED \
      --project=$PROJECT_ID

  gcloud compute backend-services add-backend compliance-backend \
      --global \
      --network-endpoint-group=compliance-server-neg \
      --network-endpoint-group-region=$REGION \
      --project=$PROJECT_ID

  gcloud compute backend-services update compliance-backend \
      --global \
      --security-policy=compliance-security-policy \
      --project=$PROJECT_ID
fi

if ! gcloud compute url-maps describe compliance-url-map --project=$PROJECT_ID &>/dev/null; then
  gcloud compute url-maps create compliance-url-map \
      --default-service=compliance-backend \
      --project=$PROJECT_ID
fi

if ! gcloud compute target-http-proxies describe compliance-http-proxy --project=$PROJECT_ID &>/dev/null; then
  gcloud compute target-http-proxies create compliance-http-proxy \
      --url-map=compliance-url-map \
      --project=$PROJECT_ID
fi

if ! gcloud compute forwarding-rules describe compliance-frontend-rule --global --project=$PROJECT_ID &>/dev/null; then
  gcloud compute forwarding-rules create compliance-frontend-rule \
      --global \
      --target-http-proxy=compliance-http-proxy \
      --ports=80 \
      --address=compliance-dashboard-ip \
      --load-balancing-scheme=EXTERNAL_MANAGED \
      --project=$PROJECT_ID
fi

echo "Done provisioning resources and deploying."
