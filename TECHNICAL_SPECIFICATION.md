# Technical Specification: Multi-Agent CRA Security Platform

## 1. Executive Summary
This document outlines the re-architecture of the `multi-agent-cra` project into a comprehensive **Cyber Resilience Act (CRA) Compliance Platform**. 
The goal is to provide Security Engineers with a real-time, dashboard-driven tool to monitor, audit, and enforce CRA compliance across their GCP estate. 
The system will leverage a microservices architecture with a dedicated frontend, an API Gateway for secure access, and granular RBAC for agent actions.

## 2. Architecture Overview

The system will transition from a monolithic CLI tool to a distributed platform with the following core components:

### 2.1. Component Diagram (Logical)
```
[User (Security Engineer)] -> [IAP / Load Balancer] -> [Frontend (Next.js)]
                                         |
                                         v
                                  [API Gateway]
                                         |
            +----------------------------+-----------------------------+
            |                            |                             |
      [Agent Engine API]        [Compliance API]               [Audit Service]
            |                            |                             |
    +-------+-------+           +--------+--------+                    v
    |   Orchestrator|           |   Dashboard API |            [Cloud Logging]
    +-------+-------+           +-----------------+
            |
    [Agent Workers (Pub/Sub)]
      |--> Aggregator
      |--> Modeler
      |--> Validator
      |--> Reviewer
      |--> Tagger
```

### 2.2. Key Technologies
*   **Frontend:** Next.js (React) + Material UI.
*   **Backend:** Go (Chi router or Gin) microservices.
*   **Agent Engine:** Custom Go-based agent framework (evolution of existing `pkg/agent`).
*   **Infrastructure:** GKE Autopilot (or Cloud Run).
*   **Identity:** 
    *   **Users:** Google Cloud Identity via IAP (Identity-Aware Proxy).
    *   **Services:** Workload Identity.
*   **Database:** Firestore (for real-time updates and flexibility) or Cloud SQL (PostgreSQL).
*   **Messaging:** Pub/Sub for asynchronous agent communication.

## 3. Detailed Components

### 3.1. Frontend Dashboard (New)
*   **Technology:** Next.js, deployed on Cloud Run or GKE (behind IAP).
*   **Features:**
    *   **Compliance Overview:** Real-time graphs of compliant vs. non-compliant assets.
    *   **Asset Inventory:** Searchable list of GCP assets with CRA status.
    *   **Agent Activity:** Live feed of agent actions (e.g., "Aggregator scanning...", "Validator found issue...").
    *   **Remediation:** Buttons to trigger Tagger/Remediation agents (protected by RBAC).
*   **Security:** Authenticated via IAP. Headers passed to backend for user identity.

### 3.2. API Gateway & Identity
*   **Gateway:** Google Cloud API Gateway or Apigee (or simpler: Ingress with IAP).
*   **Authentication:** 
    *   Enforce IAP for all external access.
    *   Validate JWT headers in backend services.
*   **Authorization (RBAC):**
    *   Define roles: `cra.viewer`, `cra.admin`, `cra.auditor`.
    *   Middleware in Go services to check user roles against required permissions.

### 3.3. Agent Engine & Workers
The monolithic `Coordinator` will be broken down:
*   **Orchestrator Service:** Receives scan requests, creates "Jobs", and publishes messages to Pub/Sub.
*   **Agent Workers:** Independent pods/services subscribing to Pub/Sub topics.
    *   `topic: scan-requests` -> **Aggregator Agent** -> `topic: assets-found`
    *   `topic: assets-found` -> **Modeler Agent** -> `topic: models-generated`
    *   `topic: models-generated` -> **Validator Agent** -> `topic: validation-results`
    *   `topic: validation-results` -> **Reviewer Agent** -> `topic: final-reports`
*   **Scaling:** KEDA (Kubernetes Event-driven Autoscaling) to scale workers based on Pub/Sub queue depth.

### 3.4. Audit Logging
*   **Audit Logging:**
    *   Middleware in all services to structured log every API call and Agent action to **stdout** (for ingestion by Cloud Logging).
    *   Logs include: `actor_identity`, `resource_acted_upon`, `action_type`, `status`.

## 4. Security Specification

### 4.1. Network Policies
*   **Default Deny:** All pods deny ingress by default.
*   **Allow Rules:**
    *   Frontend -> API Gateway/Backend.
    *   Backend -> Google APIs (Gemini, Asset Inventory).
    *   Backend -> Database (Firestore/SQL).
*   **VPC Service Controls:** Enforce a perimeter around the project to prevent data exfiltration.

### 4.2. Service Accounts & IAM (Least Privilege)
*   **Frontend:** `roles/run.invoker` (if Cloud Run).
*   **Aggregator:** `roles/cloudasset.viewer`.
*   **Tagger:** `roles/resourcemanager.tagUser` (conditional on specific tags).
*   **All Agents:** `roles/aiplatform.user` (Vertex AI).

### 4.3. Secret Management
*   **Secret Manager:** Store any non-GCP API keys (if needed) and database credentials.
*   **Mounting:** Use Secret Manager CSI Driver for GKE.

## 5. Implementation Plan

### Phase 1: Foundation & API (Current Refactor focus)
1.  **Modularize Code:** Split `main.go` into `cmd/server/` (API) and `cmd/worker/` (Agents).
2.  **Containerize:** Update `Dockerfile` for multi-stage builds.
3.  **Infrastructure:** Update Terraform for GKE Autopilot, Pub/Sub, and Firestore.

### Phase 2: Agent Event Loop
1.  **Refactor Coordinator:** Implement Pub/Sub logic in `pkg/workflow`.
2.  **Deploy Workers:** Deploy separate Deployments for Aggregator, Modeler, etc.

### Phase 3: Frontend & Security
1.  **Build Dashboard:** Initialize Next.js app in `web/`.
2.  **Configure IAP:** Set up OAuth consent screen and IAP in Terraform.

## 6. Testing Strategy
*   **Unit:** Mock Pub/Sub and Gemini for agent logic.
*   **Integration:** Test full flow from API trigger -> Pub/Sub -> Agent -> Database.
*   **E2E:** Playwright tests for the Frontend dashboard.