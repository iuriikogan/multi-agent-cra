# Multi-Agent CRA Compliance System

![Architecture Status](https://img.shields.io/badge/Architecture-Event--Driven-blue)
![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8)
![AI Model](https://img.shields.io/badge/AI-Gemini%201.5%20Pro-8E75B2)

A scalable, event-driven multi-agent system designed to assess Google Cloud infrastructure against the EU Cyber Resilience Act (CRA).

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

### Quick Start (Local Development)

We provide a `Makefile` and `docker-compose` setup for easy local development with emulators.

1.  **Configure Environment:**
    ```bash
    cp .env.example .env
    # Edit .env and set your GEMINI_API_KEY and PROJECT_ID
    ```

2.  **Start Services:**
    ```bash
    make start
    ```
    This will spin up:
    *   **Backend API:** http://localhost:8080
    *   **Frontend:** http://localhost:3000
    *   **Pub/Sub Emulator:** http://localhost:8085
    *   **Firestore Emulator:** http://localhost:8081

3.  **Other Commands:**
    *   `make build`: Compile all binaries.
    *   `make test`: Run unit tests.
    *   `make lint`: Run linters.
    *   `make stop`: Stop all local services.
    *   `make clean`: Cleanup artifacts.

### Production Deployment (GKE/Cloud Run)

Use the provided `DEPLOY.sh` script to deploy the infrastructure and application to Google Cloud.

```bash
# Deploy to Cloud Run (Serverless)
./DEPLOY.sh cloudrun

# Deploy to GKE (Kubernetes)
./DEPLOY.sh gke
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for details on the system design.

## 🧪 Testing

Run unit tests:
```bash
go test ./...
```

Run security scans (requires Snyk):
```bash
snyk test
snyk code test
```

## 📜 License

Apache 2.0
