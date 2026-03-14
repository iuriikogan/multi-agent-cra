# Multi-Agent CRA Security Platform

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A scalable, event-driven multi-agent system designed to assess Google Cloud infrastructure against the EU Cyber Resilience Act (CRA). The goal is to provide Security Engineers with a real-time, dashboard-driven tool to monitor, audit, and enforce CRA compliance across their GCP estate.

## Key Features

*   Autonomous Agents: Specialized AI agents for Discovery (Aggregator), Modeling, Validation, Review, and Tagging.
*   Real-time Dashboard: A Next.js frontend embedded in the Go binary featuring live Server-Sent Events (SSE) log streaming and interactive compliance charts.
*   Event-Driven: Decoupled architecture using Google Cloud Pub/Sub for resilient, multi-stage agent pipelines.
  
## High-Level System Architecture and Data Flow
#####    (Detailed Architecture can be found in ARCHITECTURE.md)

The system uses a strictly decoupled producer-consumer model:

1.  Frontend (UI): Users interact with the embedded React dashboard to initiate scans or view historical CRA findings.
2.  API Server (ROLE=server): Receives HTTP scan requests, publishes them to Pub/Sub, and serves historical data from the database. It also maintains long-lived SSE connections to broadcast internal monitoring events to the browser.
3.  Message Broker (Pub/Sub): Manages discrete topics for every stage of the agent pipeline (scan-requests -> aggregator -> modeler -> validator -> reviewer -> tagger).
4.  Worker Fleet (ROLE=worker): Stateless background processes that consume Pub/Sub messages, execute Gemini agent logic, interact with GCP APIs (like Cloud Asset Inventory), and write findings to the database.
5.  State Store: 
    *   Cloud SQL (Production): Persistent storage of scan metadata and compliance findings.
    *   SQLite (Local): In-memory ephemeral storage for rapid testing.

### Security Controls
*   Least Privilege: Workers operate using dedicated Google Service Accounts with minimal permissions required for Asset Inventory and Pub/Sub.
*   No Hardcoded Secrets: API keys and Database URLs are injected securely at runtime via environment variables.
*   Network Isolation: Cloud SQL instances should be deployed with private IPs. The cra-worker does not expose any inbound ports.

## Project Structure

```text
├── cmd/
│   ├── batch/       # Batch execution mode entrypoint
│   ├── server/      # Core API and WebSocket server entrypoint
│   └── worker/      # Pub/Sub background worker entrypoint
├── internal/
│   ├── batch/       # Batch processing and reporting logic
│   ├── server/      # HTTP handlers and SSE Hub logic
│   └── worker/      # Agent initialization and Pub/Sub subscriptions
├── pkg/
│   ├── agent/       # Gemini AI Agent logic
│   ├── config/      # Centralized Configuration
│   ├── queue/       # Pub/Sub client implementations
│   ├── store/       # Cloud SQL and SQLite implementations
│   ├── tools/       # GCP SDK and LLM tool definitions
│   └── workflow/    # Pub/Sub pipeline orchestrator
├── web/             # Next.js Frontend Dashboard (compiled into Go binary)
└── terraform/       # IaC definitions for GCP deployment
```

## Prerequisites

Before deploying the application locally or in production, ensure the following prerequisites are met:

*   Google Cloud Platform project with billing enabled.
*   Valid Google Cloud credentials configured (`gcloud auth application-default login`).
*   Go 1.25 or higher installed.
*   Terraform installed.
*   A valid Gemini API Key.
*   (Production) Google Cloud services enabled: run.googleapis.com, cloudbuild.googleapis.com, artifactregistry.googleapis.com, secretmanager.googleapis.com, sqladmin.googleapis.com.

## [Deployment Options](https://github.com/iuriikogan/multi-agent-cra/DEPLOY.md)

## [Architecture](https://github.com/iuriikogan/multi-agent-cra/blob/main/ARCHITECTURE.md)

## [Contributing](https://github.com/iuriikogan/multi-agent-cra/blob/main/CONTRIBUTING.md)

## [Security](https://github.com/iuriikogan/multi-agent-cra/blob/main/SECURITY.md)
