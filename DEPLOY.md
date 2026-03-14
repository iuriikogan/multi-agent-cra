# Deployment Instructions

This document provides detailed instructions for deploying the Multi-Agent CRA System to Google Cloud Run using the unified `build.sh` script.

The system deploys as two simplified Cloud Run services:
1.  cra-server: Houses the API and the React Frontend (static export).
2.  cra-worker: Handles the background AI agent tasks.

## Prerequisites

Ensure the following prerequisites are met before deploying:

*   Google Cloud Project: With billing enabled.
*   Go Environment: Go 1.25+ is required for building the backend (as specified in the Dockerfile).
*   APIs Enabled: run.googleapis.com, cloudbuild.googleapis.com, artifactregistry.googleapis.com, secretmanager.googleapis.com.
*   Permissions: You need the Owner or Editor role on the project to run the setup script initially.
*   Local tools: `make`, `docker-compose`, and `gcloud` CLI installed for local execution.

## Steps

### Local Deployment (Development)

Run the application locally for development and testing using Docker Compose.

1.  Configure Environment:
    Copy `.env.example` to `.env` and fill in your values (specifically `GEMINI_API_KEY`). By default, local execution via `docker-compose.yml` configures `DATABASE_TYPE=SQLITE_MEM` for an ephemeral in-memory SQLite database, avoiding the need for a separate database container.
    ```bash
    cp .env.example .env
    ```

2.  Start Services:
    ```bash
    make start
    ```
    This will build the frontend, embed it into the server, and start the Server, Worker, and Emulators.

### Production Deployment (Cloud Run)

The `build.sh` script automates the entire process:
*   Enables APIs.
*   Creates Artifact Registry.
*   Creates/Updates the `GEMINI_API_KEY` secret.
*   Sets up Pub/Sub topics.
*   Configures AI agents to use the Gemini 2.5 series models (`gemini-2.5-flash` for aggregation/tagging and `gemini-2.5-pro` for reasoning).
*   Fixes IAM permissions for Cloud Build.
*   Triggers the build and deployment.

1.  Run the Build Script:
    ```bash
    ./build.sh
    ```

2.  Security (Cloud Armor):
    After deployment, configure Google Cloud Armor to protect your public endpoint.
    *   Go to Console: Navigate to Network Security > Cloud Armor.
    *   Create Policy: Create a policy named `agent-armor-policy`.
    *   Configure Rules: Enable Model Armor to filter malicious LLM prompts. Enable Adaptive Protection for DDoS mitigation. Restrict access to your corporate IP range or VPN if required.
    *   Attach Policy: Go to Cloud Run > cra-server > Integrations (or Security). Attach the Cloud Armor policy/load balancer configuration.

## Verification

To verify the deployment:

1.  Local: Access the dashboard at `http://localhost:8080`.
2.  Production: Once the script completes, it will output the URL for `cra-server` (e.g., `https://cra-server-578241461072.us-central1.run.app`). Open this URL in your browser to access the Dashboard.
3.  Check Cloud Run logs to ensure both `cra-server` and `cra-worker` are running without initialization errors.

## Rollback

To tear down the deployed resources:

1.  Production Rollback:
    Run the build script with the destroy flag to bring down all resources:
    ```bash
    ./build.sh --destroy
    ```
2.  Local Rollback:
    Stop the services using:
    ```bash
    make stop
    ```

## Architecture Notes

*   Frontend: Built as a static site (`next build` with `output: export`) and embedded into the Go binary (`//go:embed`). This allows a single container to serve the UI and API.
*   Scaling:
    *   `cra-server`: Auto-scales based on HTTP traffic.
    *   `cra-worker`: Auto-scales based on CPU/Memory usage (or custom Pub/Sub metrics if configured). Can scale to 0 to save costs.
