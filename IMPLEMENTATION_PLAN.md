# Implementation Plan: Multi-Agent CRA Security Platform

## Phase 1: Core Refactoring & Infrastructure
- [x] **Directory Structure**: Created `cmd/server`, `cmd/worker`, `pkg/core`, `pkg/config`.
- [x] **Core Domain**: Refactored `pkg/domain` into `pkg/core` with cleaner interfaces.
- [x] **Configuration**: Implemented `pkg/config` for centralized env var management.
- [x] **Terraform**: Updated infrastructure for Pub/Sub, Firestore, and Network Policies.

## Phase 2: Backend & Agents
- [x] **Agent Engine**: Refactored `pkg/agent` to be a reusable library using `tools.Executor`.
- [x] **Pub/Sub Integration**: Implemented producer/consumer logic in `pkg/queue`.
- [x] **Worker**: Created `cmd/worker` to consume tasks and run agents.
- [x] **API Server**: Created `cmd/server` to expose REST endpoints for the frontend.

## Phase 3: Frontend Dashboard
- [x] **Scaffold**: Initialized Next.js application in `web/` directory.
- [x] **UI Components**: Built Dashboard, Asset List, and Agent Activity views.
- [ ] **Integration**: Connect Frontend to API Server (pending deployment).

## Phase 4: Security & Integration
- [x] **Audit**: Added structured audit logging middleware to stdout.
- [x] **RBAC**: Implemented role-based access control middleware stubs in the API Server.
- [ ] **IAP**: Configuration requires OAuth setup (manual step).
