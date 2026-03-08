# Multi-Agent CRA Compliance System

![Architecture Status](https://img.shields.io/badge/Architecture-Event--Driven-blue)
![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8)
![AI Model](https://img.shields.io/badge/AI-Gemini%201.5%20Pro-8E75B2)

A scalable, event-driven multi-agent system designed to assess Google Cloud infrastructure against the EU Cyber Resilience Act (CRA). The goal is to provide Security Engineers with a real-time, dashboard-driven tool to monitor, audit, and enforce CRA compliance across their GCP estate.

## 🚀 Features

*   **Autonomous Agents:** Specialized AI agents for Discovery, Modeling, Validation, and Reporting.
*   **Event-Driven:** Decoupled architecture using Google Cloud Pub/Sub.
*   **Scalable:** Deploys on Google Kubernetes Engine (GKE) or Cloud Run.
*   **AI-Powered:** Leverages Gemini 1.5 Pro for deep reasoning and compliance mapping.
*   **Infrastructure as Code:** Full Terraform setup included.

## 🏗️ System Architecture

![System Architecture](architecture.png)

The system is composed of the following key components:

1.  **Frontend (Next.js):** A responsive web dashboard for triggering scans, viewing results, and managing compliance reports.
2.  **Backend API (Go):** A RESTful API server that handles user requests, initiates scans via Pub/Sub, and queries Firestore for data.
3.  **Worker (Go):** An autonomous worker service that consumes scan requests from Pub/Sub, orchestrates the AI agents, and performs the actual compliance assessments.
4.  **Pub/Sub:** Acts as the asynchronous message bus, decoupling the API server from the heavy processing in the workers.
5.  **Firestore:** Stores scan results, compliance reports, and audit logs.
6.  **Gemini AI:** The reasoning engine used by the agents to analyze infrastructure and determine compliance.

### Agent Workflow

![Agent Workflow](agent_workflow.png)

The compliance process is driven by a chain of specialized agents:

*   **Resource Aggregator:** Discovers and ingests GCP assets.
*   **CRA Modeler:** Applies the CRA compliance framework to the data.
*   **Compliance Validator:** Validates the model against regulatory rules.
*   **Reviewer:** Provides final approval and summary of the report.
*   **Resource Tagger:** Tags resources with compliance status and remediation steps.

## 📂 Project Structure

```
├── cmd/
│   ├── server/      # HTTP API Entrypoint
│   ├── worker/      # Event-driven Worker Agents
│   └── batch/       # Legacy CLI/Batch mode
├── pkg/
│   ├── agent/       # Gemini Agent implementation
│   ├── core/        # Domain types
│   ├── workflow/    # Orchestration logic
│   └── tools/       # Agent tools (GCP API, etc.)
├── terraform/       # Infrastructure definitions (GKE, PubSub, IAM)
└── web/             # Frontend Dashboard (Next.js)
```

## 🛠️ Setup & Deployment

### Prerequisites
*   Go 1.25+
*   Google Cloud Project with Billing enabled
*   `gcloud` CLI installed and authenticated
*   `terraform` installed
*   Gemini API Key
*   Docker & Docker Compose

# Deployment Instructions

This document provides detailed instructions for deploying the Multi-Agent CRA System locally, to Google Kubernetes Engine (GKE) using Terraform, and to Cloud Run using the provided shell script.

## 1. Local Deployment

Run the application locally for development and testing.

### Prerequisites
*   **Go**: Version 1.25 or higher ([Download](https://go.dev/dl/))
*   **Google Cloud Project with Billing enabled**
*   **`gcloud` CLI installed and authenticated**
*   **`terraform` installed**
*   **Gemini API Key**

### Steps
1.  **Clone the repository** (if not already done):
    ```bash
    git clone <repository-url>
    cd multi-agent-cra
    ```

2.  **Set Environment Variables**:
    ```bash
    # Linux/macOS
    export GEMINI_API_KEY="your_actual_api_key_here"

    # Windows (PowerShell)
    $env:GEMINI_API_KEY="your_actual_api_key_here"
    ```

3.  **Run the Application**:
    ```bash
    go run cmd/main.go
    ```
    *Note: The current local execution runs all agents within a single process via the coordinator.*

---

## 2. Google Kubernetes Engine (GKE) Deployment

This method uses **Terraform** to provision a GKE Autopilot cluster, secure secrets, and deploy the agents as separate Kubernetes workloads.

### Prerequisites
*   **Google Cloud Project**: With billing enabled.
*   **Terraform**: Installed.
*   **gcloud CLI**: Installed and authenticated (`gcloud auth login`, `gcloud auth application-default login`).
*   **Docker**: For building the image.

### Step 1: Build and Push Docker Image
Before running Terraform, the container image must exist in a registry (e.g., Google Artifact Registry or Container Registry).

1.  **Set Variables**:
    ```bash
    export PROJECT_ID="your-project-id"
    export IMAGE_NAME="gcr.io/${PROJECT_ID}/agent-cra:latest"
    ```

2.  **Build and Push**:
    ```bash
    # Enable Container Registry API if needed, or use Artifact Registry
    gcloud services enable containerregistry.googleapis.com

    # Build
    docker build -t $IMAGE_NAME .

    # Configure Docker to push to GCR
    gcloud auth configure-docker

    # Push
    docker push $IMAGE_NAME
    ```

### Step 2: Deploy Infrastructure with Terraform
1.  **Navigate to the Terraform directory**:
    ```bash
    cd terraform
    ```

2.  **Create a `terraform.tfvars` file**:
    Create a file named `terraform.tfvars` with your specific configuration. **Do not commit this file.**
    ```hcl
    project_id       = "your-project-id"
    region           = "us-central1"
    cluster_name     = "agent-engine-cluster"
    image_repository = "gcr.io/your-project-id/agent-cra:latest" # Must match the image pushed in Step 1
    gemini_api_key   = "your-actual-gemini-api-key"
    ```

3.  **Initialize and Apply**:
    ```bash
    terraform init
    terraform apply
    ```
    *Confirm the action by typing `yes` when prompted.*

    **What this does:**
    *   Creates a VPC Network and Subnet.
    *   Provisions a GKE Autopilot Cluster.
    *   Creates a Secret in Google Secret Manager for the API Key.
    *   Sets up Workload Identity (IAM binding between K8s Service Accounts and Google Service Accounts).
    *   Deploys 4 microservices (`agent-classifier`, `agent-auditor`, `agent-vuln`, `agent-reporter`).

### Step 3: Verify Deployment
1.  **Get Cluster Credentials**:
    ```bash
    gcloud container clusters get-credentials agent-engine-cluster --region us-central1
    ```

2.  **Check Pods**:
    ```bash
    kubectl get pods
    ```
    You should see pods for each agent (classifier, auditor, vuln, reporter) running.

---

## 3. Cloud Run Deployment

This method uses the `deploy.sh` script to deploy the agents as serverless Cloud Run services.

### Prerequisites
*   **Google Cloud SDK**: `gcloud` installed and authenticated.
*   **Project ID**: Set your active project (`gcloud config set project YOUR_PROJECT_ID`).

### Step 1: Create the API Key Secret
The deployment script expects a secret named `gemini-api-key` to exist in Secret Manager.

```bash
# Replace YOUR_API_KEY with your actual key
echo -n "YOUR_API_KEY" | gcloud secrets create gemini-api-key --data-file=-
```

### Step 2: Run the Deployment Script
1.  **Make the script executable**:
    ```bash
    chmod +x deploy.sh
    ```

2.  **Run the script**:
    ```bash
    ./deploy.sh
    ```

    **What this script does:**
    *   Enables necessary Google Cloud APIs.
    *   Creates a dedicated Service Account.
    *   Creates an Artifact Registry repository.
    *   Builds the Docker image using Cloud Build (no local Docker required).
    *   Deploys 4 Cloud Run services, injecting the API Key secret and setting the `AGENT_ROLE` environment variable.

### Step 3: Verify
The script will output the URLs of the deployed services. You can also list them:
```bash
gcloud run services list
```
