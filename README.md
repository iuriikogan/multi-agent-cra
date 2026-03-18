# Multi-Agent Compliance Security Platform (CRA & DORA)

A scalable, event-driven multi-agent system designed to autonomously assess Google Cloud infrastructure against the EU Cyber Resilience Act (CRA) and the Digital Operational Resilience Act (DORA). The goal is to provide Security Engineers with a real-time, dashboard-driven tool to monitor, audit, and enforce regulatory compliance across their GCP estate.

## Key Features

*   **Autonomous Agent Swarm**: Specialized AI agents (Aggregator, Modeler, Validator, Reviewer, and Tagger) handling decoupled workflow stages.
*   **Integrated Regulatory Knowledge**: Vector-searchable knowledge bases for both the EU CRA and DORA, allowing the Validator agent to reference exact legislative clauses for high-fidelity compliance reasoning.
*   **Real-time Next.js Dashboard**: A TypeScript Next.js frontend embedded directly into the Go binary. It features live Server-Sent Events (SSE) log streaming, interactive compliance charts, and framework-specific filtering.
*   **Event-Driven Pipeline**: Deeply decoupled architecture leveraging Google Cloud Pub/Sub for resilient, asynchronous agent pipelines.

## High-Level System Architecture and Data Flow

Detailed Architecture can be found in [ARCHITECTURE.md](./ARCHITECTURE.md)

The system enforces a strict separation of concerns via a producer-consumer model:

1.  **Frontend (UI)**: Users interact with the embedded Next.js dashboard to dispatch compliance scans (CRA or DORA) or visualize historical findings.
2.  **API Server (`ROLE=server`)**: Ingests HTTP scan requests, publishes them to Pub/Sub, and serves historical context from the state store. It multiplexes internal monitoring events via persistent SSE connections to the browser.
3.  **Message Broker (Pub/Sub)**: Manages resilient, discrete topics orchestrating the complete lifecycle (scan-requests -> aggregator -> modeler -> validator -> reviewer -> tagger).
4.  **Worker Fleet (`ROLE=worker`)**: Stateless background processes (powered by Google's latest GenAI SDK) that consume Pub/Sub tasks, evaluate GCP Assets, query the Knowledge Base, and commit findings.
5.  **State Store**: 
    *   **Cloud SQL (Production)**: Relational persistent storage of scan metadata and assessment results.
    *   **SQLite (Local)**: In-memory ephemeral storage for isolated rapid testing.

## Security Controls
*   **Least Privilege**: The system executes using dedicated Google Service Accounts tailored strictly for Asset Inventory read-access and Pub/Sub interactions.
*   **Secret Management**: Hardcoded secrets are explicitly avoided. API keys and Database URLs are resolved at runtime via Google Secret Manager.
*   **Network Isolation**: Production configurations restrict Cloud SQL instances to private IPs, shielded behind Serverless VPC Access connectors.

## Project Structure

```text
├── cmd/             # Application entrypoints
│   ├── batch/       # Batch execution mode
│   ├── server/      # Core API and Next.js static server
│   └── worker/      # Pub/Sub background agent worker
├── internal/        # Internal application logic
├── pkg/             # Core library and domain logic
│   ├── agent/       # Gemini AI Agent logic (Google GenAI SDK)
│   ├── core/        # Domain entities (GCPResource, AssessmentResult)
│   ├── knowledge/   # RAG implementations and embedded JSON Knowledge Bases
│   ├── queue/       # Pub/Sub publish/subscribe semantics
│   ├── store/       # Persistent storage (Cloud SQL, SQLite)
│   ├── tools/       # Agent-callable function definitions
│   └── workflow/    # Pub/Sub pipeline orchestrator
├── web/             # Next.js Frontend Dashboard (Typescript, React)
└── terraform/       # IaC definitions for GCP deployment
```

## Knowledge Base & Embeddings

The system uses vector embeddings to provide specialized regulatory knowledge to the agents. These are stored as JSON files in `pkg/knowledge/`.

### Adding New Regulation Embeddings

To add a new regulation (e.g., "NIST"), follow these steps:

1.  **Prepare the Text**: Create a plain text file (e.g., `nist_text.txt`) containing the regulation's clauses.
2.  **Generate Embeddings**: Use the unified embedding script:
    ```bash
    export GEMINI_API_KEY="your-api-key"
    export EMBEDDING_INPUT_FILE="nist_text.txt"
    export EMBEDDING_OUTPUT_FILE="pkg/knowledge/nist_kb.json"
    go run scripts/embeddings/main.go
    ```
3.  **Update Domain Logic**: If necessary, update the Validator agent to reference the new knowledge base in `pkg/knowledge/knowledge.go`.

## Prerequisites

Ensure the following prerequisites are met:
*   Google Cloud Platform project with billing enabled.
*   Valid Google Cloud credentials configured (`gcloud auth application-default login`).
*   Go 1.25 or higher installed.
*   Node.js & npm (for building the Next.js UI).
*   Terraform installed.
*   A valid Gemini API Key.
*   Google Cloud services enabled: `run.googleapis.com`, `cloudbuild.googleapis.com`, `artifactregistry.googleapis.com`, `secretmanager.googleapis.com`, `sqladmin.googleapis.com`.

For instructions on deployment, refer to [DEPLOY.md](./DEPLOY.md).
*   (Production) Google Cloud services enabled: run.googleapis.com, cloudbuild.googleapis.com, artifactregistry.googleapis.com, secretmanager.googleapis.com, sqladmin.googleapis.com, cloudtrace.googleapis.com.

### [   Architecture](https://github.com/iuriikogan/Audit-Agent/blob/main/ARCHITECTURE.md)
