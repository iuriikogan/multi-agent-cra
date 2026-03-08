# System Architecture

This document describes the technical architecture of the Multi-Agent Cyber Resilience Act (CRA) Compliance System.

## Overview

The system is designed as an event-driven, microservices-based application running on Google Cloud Platform. It leverages **Google Cloud Pub/Sub** for asynchronous communication between agents, **Cloud Run** (or **GKE**) for compute, and **Firestore** for state management. **Google Gemini 1.5 Pro** serves as the reasoning engine for the AI agents.

## Core Components

### 1. API Server (`cmd/server`)
*   **Role:** Entry point for user requests.
*   **Functionality:**
    *   Exposes HTTP endpoints (`POST /api/scan`).
    *   Authenticates users (mocked/IAP).
    *   Publishes scan jobs to the `scan-requests` Pub/Sub topic.
    *   Provides health checks (`/healthz`).

### 2. Worker Agents (`cmd/worker`)
*   **Role:** Autonomous units performing specific compliance tasks.
*   **Scalability:** Horizontal scaling based on Pub/Sub queue depth.
*   **Agent Roles:**
    *   **Aggregator:** Listens to `scan-requests`. Queries Google Cloud Asset Inventory to discover resources. Publishes to `assets-found`.
    *   **Modeler:** Listens to `assets-found`. Uses Gemini to map technical configurations to CRA requirements. Publishes to `models-generated`.
    *   **Validator:** Listens to `models-generated`. Uses Gemini to validate compliance against specific rules. Publishes to `validation-results`.
    *   **Reviewer:** Listens to `validation-results`. Aggregates findings and generates a final report.
    *   **Tagger:** Applies labels to GCP resources based on compliance status.
    *   **Visual Reporter:** Generates compliance dashboard images.

### 3. Messaging Infrastructure (Pub/Sub)
*   **Topics:**
    *   `scan-requests`: Triggers a new compliance scan.
    *   `assets-found`: Contains raw asset data.
    *   `models-generated`: Contains CRA compliance models.
    *   `validation-results`: Contains pass/fail results per resource.
    *   `final-reports`: Aggregated reports.

### 4. Data Storage
*   **Firestore (Optional/Planned):** Stores persistent state of scans, audit logs, and historical compliance data.
*   **Cloud Storage:** Stores generated reports (PDF/CSV) and dashboard images.

### 5. AI Engine
*   **Google Gemini API:**
    *   `gemini-3.1-flash-lite-preview`: Used for high-volume, low-latency tasks (Aggregator, Tagger).
    *   `gemini-3-pro-preview`: Used for complex reasoning and validation (Modeler, Validator, Reviewer).

### 6. Security (Cloud Armor)
*   **WAF & DDoS Protection:** Google Cloud Armor protects the public endpoints.
*   **Model Armor:** Specific AI/LLM protection rules (e.g., prompt injection, jailbreak detection) are applied to the ingress traffic.
*   **Management:** Security policies are managed via the Google Cloud Console to leverage Adaptive Protection and fine-grained rule tuning.

## Data Flow

1.  **User** submits a scan request for a Project/Folder via the API.
2.  **API Server** validates the request and publishes a message to `scan-requests`.
3.  **Aggregator Worker** picks up the message, lists all GCP assets in the scope, and publishes individual asset messages to `assets-found`.
4.  **Modeler Worker** processes each asset, generating a compliance model JSON, and publishes to `models-generated`.
5.  **Validator Worker** checks the model against hardcoded or retrieved CRA rules, publishing results to `validation-results`.
6.  **Reviewer/Tagger/Reporter** act on the results to finalize the workflow.

## Infrastructure as Code (Terraform)

The `terraform/` directory contains the definition for:
*   GKE Cluster (Autopilot)
*   Pub/Sub Topics and Subscriptions.
*   IAM Service Accounts and Bindings.
*   VPC Network and Security configurations.
