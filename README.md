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

### Quick Start (Local)

1.  **Set Environment Variables:**
    ```bash
    export GEMINI_API_KEY="your_key"
    export PROJECT_ID="your_project_id"
    export GOOGLE_APPLICATION_CREDENTIALS="path/to/creds.json"
    ```

2.  **Run the Server:**
    ```bash
    go run cmd/server/main.go
    ```

3.  **Run a Worker:**
    ```bash
    go run cmd/worker/main.go
    ```

4.  **Trigger a Scan:**
    ```bash
    curl -X POST http://localhost:8080/api/scan -d '{"scope": "projects/my-project"}'
    ```

### Deployment (GKE/Cloud Run)

Use the provided `DEPLOY.sh` script to deploy the infrastructure and application.

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
