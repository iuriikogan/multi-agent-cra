# System Architecture & Security Design

This document describes the technical architecture of the Multi-Agent Regulatory Compliance System (supporting CRA and DORA).

### [Deployment Instructions](https://github.com/iuriikogan/Audit-Agent/blob/main/DEPLOY.md)

## High-Level System Architecture

The system is deployed as a single compiled Go binary that adapts its behavior based on the `ROLE` environment variable, enabling independent scaling of the API/UI and background processing workloads on Google Cloud Run.

```mermaid
graph TD
    User([Security Engineer]) -->|HTTPS| CloudArmor[Cloud Armor WAF]
    CloudArmor --> Server[Cloud Run: API/UI Server]
    
    Server -->|Read/Write| DB[(Cloud SQL / SQLite)]
    Server -->|Publish Scan| PubSub_Scan[Pub/Sub: scan-requests]
    Server -.->|SSE Stream| User
    
    WorkerFleet[Cloud Run: Worker Fleet] -->|Subscribe| PubSub_Scan
    WorkerFleet <-->|Multi-stage Pipeline| PubSub_Internal[Pub/Sub: Internal Queues]
    WorkerFleet -->|Read/Write| DB
    WorkerFleet -->|Broadcast Events| PubSub_Monitoring[Pub/Sub: monitoring-events]
    
    PubSub_Monitoring -->|Consume| Server
    
    WorkerFleet <-->|Agent Reasoning| Gemini[Google Gemini API]
    WorkerFleet -->|Embedded Context| KB[(Regulatory KB: CRA & DORA)]
    WorkerFleet <-->|Discover/Tag| GCP_API[GCP Asset Inventory API]

    %% Observability
    Server -.->|Export Spans| Trace[Cloud Trace]
    WorkerFleet -.->|Export Spans| Trace
    Server -.->|Structured Logs| Logging[Cloud Logging]
    WorkerFleet -.->|Structured Logs| Logging

    classDef gcp fill:#e8f0fe,stroke:#1a73e8,stroke-width:2px;
    class Server,WorkerFleet,CloudArmor,PubSub_Scan,PubSub_Internal,PubSub_Monitoring,DB,Gemini,GCP_API,KB,Trace,Logging gcp;
```

## Observability & Distributed Tracing

The system implements full-stack observability using OpenTelemetry to provide end-to-end visibility into the asynchronous, multi-agent workflows.

### Distributed Tracing Flow

Trace context is propagated across service boundaries (including Pub/Sub) to maintain a single trace for each compliance assessment.

```mermaid
sequenceDiagram
    participant Browser as Next.js Frontend
    participant Server as Go API Server
    participant PubSub as Google Cloud Pub/Sub
    participant Worker as Go Worker (Agents)
    participant Trace as Google Cloud Trace

    Browser->>Server: POST /api/scan (Start Span)
    Server->>Server: Create Scan Record
    Server->>PubSub: Publish Scan Request (Inject Context)
    Server-->>Browser: 202 Accepted
    Server->>Trace: Export Server Span

    PubSub-->>Worker: Pull Message (Extract Context)
    Worker->>Worker: Start Processing Span
    Worker->>Worker: Agent Reasoning (Gemini)
    Worker->>Trace: Export Worker Spans
```

### Log-Trace Correlation

By injecting `logging.googleapis.com/trace` and `logging.googleapis.com/spanId` into structured JSON logs, the system enables seamless navigation between logs and traces in the Google Cloud Console. This is critical for debugging the asynchronous behavior of the multi-agent pipeline.

## Agent Pipeline & Data Flow

The compliance process is a multi-stage, event-driven pipeline where autonomous AI agents perform specific roles.

```mermaid
sequenceDiagram
    participant UI as Compliance Dashboard
    participant API as API Server
    participant PS as Pub/Sub
    participant Agg as Agent: Aggregator
    participant Mod as Agent: Modeler
    participant Val as Agent: Validator
    participant Rev as Agent: Reviewer
    participant Tag as Agent: Tagger
    participant DB as Cloud SQL
    participant GCP as GCP APIs

    UI->>API: POST /api/scan {scope: "org/123", regulation: "DORA"}
    API->>DB: CreateScan(job_id, scope, regulation)
    API->>PS: Publish(scan-requests, job_id)
    
    PS-->>Agg: Consume(scan-requests)
    Agg->>GCP: SearchAllResources(scope)
    GCP-->>Agg: [Asset1, Asset2...]
    loop For each Asset
        Agg->>PS: Publish(aggregator-tasks, AssetN)
    end
    
    PS-->>Mod: Consume(aggregator-tasks)
    Mod->>Mod: Gemini Reasoning: Map asset to regulatory framework
    Mod->>PS: Publish(modeler-tasks, Model)
    
    PS-->>Val: Consume(modeler-tasks)
    Val->>Val: Gemini Reasoning: Evaluate compliance rules
    Val->>KB: Semantic Search (search_knowledge_base)
    KB-->>Val: Relevant Regulatory Excerpts
    Val->>DB: AddFinding(job_id, Finding)
    Val->>PS: Publish(validator-tasks, Validation_Result)
    
    PS-->>Rev: Consume(validator-tasks)
    Rev->>Rev: Gemini Reasoning: Peer review validation logic
    Rev->>PS: Publish(reviewer-tasks, Reviewed_Result)
    
    PS-->>Tag: Consume(reviewer-tasks)
    Tag->>GCP: Apply compliance tags/labels (e.g., dora_status=compliant)
    
    Note over Agg,Tag: All agents continuously publish telemetry to 'monitoring-events'
    PS-->>API: Consume(monitoring-events)
    API-->>UI: Server-Sent Events (SSE) Live Update
```

## Security Controls

1.  **Secure Configuration Management:** No secrets (API keys, DB credentials) are stored in code or configuration files. They are injected exclusively via environment variables at runtime, sourced from Google Secret Manager.
2.  **Least Privilege Execution (Identity-Based Security):**
    *   The system uses dedicated Google Service Accounts (defined in `iam.tf`) for different stages:
        *   `compliance-server-sa`: Used by the API/UI server with access to Cloud SQL and Secrets.
        *   `compliance-worker-sa`: Used by the background worker with access to Vertex AI, Cloud Asset API, Cloud SQL, and Secrets.
        *   Agent-Specific Accounts (`sa-classifier`, `sa-auditor`, `sa-vuln`, `sa-reporter`): Available for fine-grained execution where individual agents run in isolated contexts.
    *   **Pub/Sub Push Authentication**: All internal agent communication uses Pub/Sub **Push Subscriptions** with OIDC token authentication. The worker services validate these tokens, ensuring that only the authorized Pub/Sub service can trigger agent logic.
3.  **Network Isolation:** 
    *   **Private Cloud SQL**: The mySQL database is deployed with a private IP within a Virtual Private Cloud (VPC), inaccessible from the public internet.
    *   **Serverless VPC Access**: Cloud Run services use a dedicated VPC Connector to securely reach the private database.
4.  **Ingress Protection:**
    *   The **Worker Fleet** is configured with `ingress = internal`, preventing direct access from the public internet.
    *   The **Server** is accessible publicly but protected by Google Cloud Armor (WAF and DDoS protection).
    *   **Model Armor** integration inspects incoming requests for prompt injection or jailbreak attempts before they reach the Gemini AI agents.

## State Management

The system abstracts state management through a `Store` interface, allowing flexibility based on deployment needs:

*   **Cloud SQL** Used for production. Provides robust, concurrent transaction support and complex querying capabilities for the CRA Dashboard. Database connections are secured via SSL and private IP.
*   **SQLite (In-Memory):** Used for local development and CI/CD pipelines. It provides a zero-dependency, ephemeral database that perfectly mimics the relational structure of Cloud SQL.

The frontend dashboard queries this state via the `/api/findings` endpoint, pulling historical compliance data independently of the real-time Pub/Sub pipeline.
