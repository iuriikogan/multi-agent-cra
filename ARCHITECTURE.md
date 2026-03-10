# System Architecture & Security Design

This document describes the technical architecture of the Multi-Agent Cyber Resilience Act (CRA) Compliance System.

## High-Level Deployment Architecture

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
    WorkerFleet <-->|Discover/Tag| GCP_API[GCP Asset Inventory API]

    classDef gcp fill:#e8f0fe,stroke:#1a73e8,stroke-width:2px;
    class Server,WorkerFleet,CloudArmor,PubSub_Scan,PubSub_Internal,PubSub_Monitoring,DB,Gemini,GCP_API gcp;
```

## Agent Pipeline & Data Flow

The compliance process is a multi-stage, event-driven pipeline where autonomous AI agents perform specific roles.

```mermaid
sequenceDiagram
    participant UI as CRA Dashboard
    participant API as API Server
    participant PS as Pub/Sub
    participant Agg as Agent: Aggregator
    participant Mod as Agent: Modeler
    participant Val as Agent: Validator
    participant Rev as Agent: Reviewer
    participant Tag as Agent: Tagger
    participant DB as Cloud SQL
    participant GCP as GCP APIs

    UI->>API: POST /api/scan {scope: "org/123"}
    API->>DB: CreateScan(job_id, "running")
    API->>PS: Publish(scan-requests, job_id)
    
    PS-->>Agg: Consume(scan-requests)
    Agg->>GCP: ListAssets(scope)
    GCP-->>Agg: [Asset1, Asset2...]
    loop For each Asset
        Agg->>PS: Publish(aggregator-tasks, AssetN)
    end
    
    PS-->>Mod: Consume(aggregator-tasks)
    Mod->>Mod: Gemini Reasoning: Map asset to CRA framework
    Mod->>PS: Publish(modeler-tasks, CRA_Model)
    
    PS-->>Val: Consume(modeler-tasks)
    Val->>Val: Gemini Reasoning: Evaluate compliance rules
    Val->>DB: AddFinding(job_id, Finding)
    Val->>PS: Publish(validator-tasks, Validation_Result)
    
    PS-->>Rev: Consume(validator-tasks)
    Rev->>Rev: Gemini Reasoning: Peer review validation logic
    Rev->>PS: Publish(reviewer-tasks, Reviewed_Result)
    
    PS-->>Tag: Consume(reviewer-tasks)
    Tag->>GCP: Apply compliance tags/labels
    
    Note over Agg,Tag: All agents continuously publish telemetry to 'monitoring-events'
    PS-->>API: Consume(monitoring-events)
    API-->>UI: Server-Sent Events (SSE) Live Update
```

## Security Controls

1.  **Strict 12-Factor Configuration:** No secrets (API keys, DB credentials) are stored in code or configuration files. They are injected exclusively via environment variables at runtime, sourced from Google Secret Manager.
2.  **Least Privilege Execution:**
    *   The `server` role requires only database access and Pub/Sub publish rights.
    *   The `worker` role operates under a dedicated Service Account with specific permissions to read Cloud Asset Inventory and apply Resource Tags. It does not expose any inbound network ports.
3.  **Network Isolation:** 
    *   The Cloud SQL database is deployed with a private IP within a Virtual Private Cloud (VPC), inaccessible from the public internet.
    *   Serverless VPC Access connectors route traffic from Cloud Run to the private database.
4.  **Ingress Protection:**
    *   Google Cloud Armor sits in front of the API server, providing WAF and DDoS protection.
    *   **Model Armor** integration inspects incoming requests for prompt injection or jailbreak attempts before they reach the Gemini AI agents.

## State Management

The system abstracts state management through a `Store` interface, allowing flexibility based on deployment needs:

*   **Cloud SQL (PostgreSQL):** Used for production. Provides robust, concurrent transaction support and complex querying capabilities for the CRA Dashboard.
*   **SQLite (In-Memory):** Used for local development and CI/CD pipelines. It provides a zero-dependency, ephemeral database that perfectly mimics the relational structure of Cloud SQL without requiring a running database server.

The frontend dashboard queries this state via the `/api/findings` endpoint, pulling historical compliance data independently of the real-time Pub/Sub pipeline.