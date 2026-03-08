# Deployment Instructions

This document provides detailed instructions for deploying the Multi-Agent CRA System to Google Cloud Run using the unified `build.sh` script.

## 1. Local Deployment (Development)

Run the application locally for development and testing using Docker Compose.

1.  **Configure Environment:**
    Copy `.env.example` to `.env` and fill in your values (specifically `GEMINI_API_KEY`).
    ```bash
    cp .env.example .env
    ```

2.  **Start Services:**
    ```bash
    make start
    ```
    This will build the frontend, embed it into the server, and start the Server, Worker, and Emulators.
    *   **Dashboard:** http://localhost:8080

3.  **Stop Services:**
    ```bash
    make stop
    ```

## 2. Production Deployment (Cloud Run)

The system deploys as two simplified Cloud Run services:
1.  **cra-server**: Houses the API and the React Frontend (static export).
2.  **cra-worker**: Handles the background AI agent tasks.

### Prerequisites
*   **Google Cloud Project**: With billing enabled.
*   **APIs Enabled**: `run.googleapis.com`, `cloudbuild.googleapis.com`, `artifactregistry.googleapis.com`, `secretmanager.googleapis.com`.
*   **Permissions**: You need Owner or Editor role on the project to run the setup script initially.

### Step 1: Run the Build Script
The `build.sh` script automates the entire process:
*   Enables APIs.
*   Creates Artifact Registry.
*   Creates/Updates the `GEMINI_API_KEY` secret.
*   Sets up Pub/Sub topics.
*   Fixes IAM permissions for Cloud Build.
*   Triggers the build and deployment.

```bash
./build.sh
```

### Step 2: Access the Application
Once the script completes, it will output the URL for `cra-server`.
Example: `https://cra-server-578241461072.us-central1.run.app`

Open this URL in your browser to access the Dashboard.

### Step 3: Security (Cloud Armor)

**Crucial Step:** After deployment, configure **Google Cloud Armor** to protect your public endpoint.

1.  **Go to Console**: Navigate to **Network Security** > **Cloud Armor**.
2.  **Create Policy**: Create a policy named `agent-armor-policy`.
3.  **Configure Rules**:
    *   Enable **Model Armor** to filter malicious LLM prompts.
    *   Enable **Adaptive Protection** for DDoS mitigation.
    *   (Optional) Restrict access to your corporate IP range or VPN if required.
4.  **Attach Policy**:
    *   Go to **Cloud Run** > **cra-server** > **Integrations** (or Security).
    *   Attach the Cloud Armor policy/load balancer configuration.

## 3. Architecture Notes

*   **Frontend**: Built as a static site (`next build` with `output: export`) and embedded into the Go binary (`//go:embed`). This allows a single container to serve the UI and API.
*   **Scaling**: 
    *   `cra-server`: Auto-scales based on HTTP traffic.
    *   `cra-worker`: Auto-scales based on CPU/Memory usage (or custom Pub/Sub metrics if configured). Can scale to 0 to save costs.