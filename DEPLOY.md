# Deployment Instructions

This document provides detailed instructions for deploying the Multi-Agent Regulatory Compliance System to Google Cloud Run using the unified `build.sh` script.

The system deploys as two simplified Cloud Run services:
1.  compliance-server: Houses the API and the React Frontend (static export).
2.  compliance-worker: Handles the background AI agent tasks.

## Prerequisites

Ensure the following prerequisites are met before deploying:

*   Google Cloud Project: With billing enabled.
*   Go Environment: Go 1.25+ is required for building the backend (as specified in the Dockerfile).
*   APIs Enabled: run.googleapis.com, cloudbuild.googleapis.com, artifactregistry.googleapis.com, secretmanager.googleapis.com.
*   Permissions: You need the Owner or Editor role on the project to run the setup script initially.
*   Local tools: `make`, `terraform`, and `gcloud` CLI installed.

## Steps

### Local Deployment (Development)

Run the application locally for development and testing using Docker Compose.

1.  Configure Environment:
    Copy `.env.example` to `.env` and fill in your values (specifically `GEMINI_API_KEY`). By default, local execution via `docker-compose.yml` configures `DATABASE_TYPE=SQLITE_MEM`.
    ```bash
    cp .env.example .env
    ```

2.  Start Services:
    ```bash
    make start
    ```
    This will build the frontend, embed it into the server, and start the Server, Worker, and Emulators.

### Deployment Options

The Multi-Agent compliance system provides two distinct paths for production deployment on Google Cloud.

---

### Option A: Quick Start (Automated Cloud Build)

Use this option for a fully automated build and deployment process handled by Google Cloud Build. The `build.sh` script orchestrates the bootstrapping and triggers the deployment.

1.  **Configure Environment**:
    ```bash
    export GEMINI_API_KEY="your-key"
    ```

2.  **Run the Build Script**:
    ```bash
    ./build.sh
    ```
    This script will:
    - Enable required APIs.
    - Setup Secret Manager for the Gemini API Key.
    - Create the Artifact Registry repository.
    - Trigger `cloudbuild.yaml` to build Docker images and deploy the Cloud Run services.

---

### Option B: Infrastructure-as-Code (Standalone Terraform)

Use this option if you prefer managing the lifecycle of your infrastructure and services using Terraform. This path assumes container images have already been built and pushed to the registry.

1.  **Build and Push Images**:
    If not already in the registry, build and push your images:
    ```bash
    gcloud builds submit --config=cloudbuild.yaml --substitutions=_REGION="europe-west1",_REPO_NAME="multi-agent-compliance"
    ```

2.  **Deploy via Terraform**:
    Create a `terraform.tfvars` file in the `terraform/` directory or pass variables directly using `-var` flags:
    ```bash
    cd terraform
    ./setup_backend.sh
    terraform init
    terraform apply \
      -var="project_id=your-project-id" \
      -var="region=europe-west1" \
      -var="gemini_api_key=your-key" \
      -var="server_image=europe-west1-docker.pkg.dev/your-project-id/multi-agent-compliance/server:latest" \
      -var="worker_image=europe-west1-docker.pkg.dev/your-project-id/multi-agent-compliance/worker:latest"
    ```

    > [!IMPORTANT]
    > Ensure that the `server_image` and `worker_image` variables point to the full container image paths in your Artifact Registry.

### Dashboard Authentication & Access

The Compliance Dashboard is private by default and requires IAM-based authentication.

#### 1. Configure Authorized Users
Specify who can access the dashboard by setting the `authorized_users` variable in Terraform:
```hcl
authorized_users = [
  "user:you@example.com",
  "group:security-team@example.com",
  "serviceAccount:some-sa@your-project-id.iam.gserviceaccount.com"
]
```

#### 2. Accessing the Dashboard Securely

**Option A: Local Proxy (Recommended)**
Use the Google Cloud SDK to create a local proxy that handles authentication for you:
```bash
gcloud run services proxy compliance-server --region europe-west1
```
Then navigate to `http://localhost:8080` in your browser.

**Option B: OIDC Token (CLI/API)**
To access via the direct Cloud Run URL, you must include an OIDC ID Token:
```bash
# Get the service URL
SERVICE_URL=$(gcloud run services describe compliance-server --format='value(status.url)')

# Access with ID Token
curl -H "Authorization: Bearer $(gcloud auth print-identity-token)" $SERVICE_URL
```

---

## Post-Deployment Security (Cloud Armor)
    After deployment, configure Google Cloud Armor to protect your public endpoint.
    *   Go to Console: Navigate to Network Security > Cloud Armor.
    *   Create Policy: Create a policy named `agent-armor-policy`.
    *   Configure Rules: Enable Model Armor to filter malicious LLM prompts. Enable Adaptive Protection for DDoS mitigation. Restrict access to your corporate IP range or VPN if required.
    *   Attach Policy: Go to Cloud Run > compliance-server > Integrations (or Security). Attach the Cloud Armor policy/load balancer configuration.

## Verification

To verify the deployment:

1.  Local: Access the dashboard at `http://localhost:8080`.
2.  Production: Follow the steps in the [Dashboard Authentication & Access](#dashboard-authentication--access) section to securely connect to the `compliance-server` URL. Using `gcloud run services proxy` is the recommended method for browser access.
3.  Check Cloud Run logs to ensure both `compliance-server` and `compliance-worker` are running without initialization errors.

## Rollback

To tear down the deployed resources:

1.  Production Rollback:
    Run the build script with the destroy flag to bring down all resources:
    ```bash
    ./build.sh --destroy
    # or
    ./build.sh -d
    ```
2.  Local Rollback:
    Stop the services using:
    ```bash
    make stop
    ```

## Architecture Notes

*   Frontend: Built as a static site (`next build` with `output: export`) and embedded into the Go binary (`//go:embed`). This allows a single container to serve the UI and API.
*   Scaling:
    *   `compliance-server`: Auto-scales based on HTTP traffic.
    *   `compliance-worker`: Auto-scales based on CPU/Memory usage (or custom Pub/Sub metrics if configured). Can scale to 0 to save costs.
