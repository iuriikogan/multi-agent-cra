# Multi-Agent CRA Security Platform Gemini Agent Configuration

This document provides instructions and context for interacting with the Multi-Agent CRA Security Platform using the Gemini CLI.

## Project Overview

This is a scalable, event-driven multi-agent system designed to assess Google Cloud infrastructure against the EU Cyber Resilience Act (CRA). The system is built with a Go backend and a Next.js frontend. It uses Google Cloud services like Pub/Sub for its event-driven architecture and can be configured to use either Cloud SQL (PostgreSQL) for production or an in-memory SQLite database for local development.

The project is structured into several Go packages and a Next.js web application. The Go backend consists of three main entry points: a server, a worker, and a batch process. The server handles API requests and serves the frontend, the worker processes background tasks from a Pub/Sub queue, and the batch process is for batch execution. The Next.js application provides a real-time dashboard for monitoring and interacting with the system.

The infrastructure for the project is defined using Terraform, and it can be deployed to Google Cloud Run.

## Key Technologies

*   **Backend:** Go
*   **Frontend:** Next.js, React, TypeScript
*   **Database:** PostgreSQL (Cloud SQL), SQLite
*   **Messaging:** Google Cloud Pub/Sub
*   **Infrastructure:** Google Cloud Run, Terraform, Docker

## Building and Running

The project includes a `Makefile` that simplifies the build, test, and run processes.

### Build

To build the entire application (Go backend and Next.js frontend), run:

```bash
make build
```

This will compile the Go binaries and build the Next.js application, placing the static assets in a directory that will be embedded into the server binary.

To build the individual components:

*   **Go server:** `make build-server`
*   **Go worker:** `make build-worker`
*   **Go batch:** `make build-batch`
*   **Next.js frontend:** `make build-web`

### Running Locally

To run the application locally for development, you need to set the following environment variables:

```bash
export GEMINI_API_KEY="your_actual_api_key_here"
export PROJECT_ID="your-gcp-project-id"
export ROLE="all"
export DATABASE_TYPE="SQLITE_MEM"
```

Then, you can run the server using:

```bash
go run ./cmd/server
```

### Running with Docker Compose

To run the application using Docker Compose:

```bash
make start
```

To stop the services:

```bash
make stop
```

### Cloud Run Deployment

To deploy the application to Google Cloud Run, use the provided build script:

```bash
./build.sh
```

## Testing and Linting

### Testing

To run all tests (Go and web):

```bash
make test
```

To run only the Go tests:

```bash
make test-go
```

### Linting

To lint all code (Go and web):

```bash
make lint
```

To run only the Go linter:

```bash
make lint-go
```

To run only the web linter:

```bash
make lint-web
```

## Development Conventions

*   The Go backend and Next.js frontend are decoupled.
*   The Go backend follows a modular structure, with code organized into `cmd`, `internal`, and `pkg` directories.
*   The system is designed to be event-driven, using Pub/Sub for communication between services.
*   The `ROLE` environment variable is used to determine the behavior of the Go binary (server, worker, or all).
*   For local development, an in-memory SQLite database is used for convenience.
*   Infrastructure is managed using Terraform.
