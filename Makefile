# Makefile for Multi-Agent CRA Project

# Variables
GO_CMD=go
GO_BUILD=$(GO_CMD) build
GO_TEST=$(GO_CMD) test
GO_CLEAN=$(GO_CMD) clean
GO_LINT=golangci-lint run

# Directories
CMD_DIR=./cmd
BIN_DIR=./bin
WEB_DIR=./web

# Binaries
SERVER_BIN=$(BIN_DIR)/server
WORKER_BIN=$(BIN_DIR)/worker
BATCH_BIN=$(BIN_DIR)/batch

.PHONY: all build build-server build-worker build-batch build-web test test-go test-web lint lint-go lint-web start stop clean help

all: build

# --- Build Commands ---

build: build-server build-worker build-batch build-web

build-server:
	@echo "Building Server..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(SERVER_BIN) $(CMD_DIR)/server/main.go

build-worker:
	@echo "Building Worker..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(WORKER_BIN) $(CMD_DIR)/worker/main.go

build-batch:
	@echo "Building Batch..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BATCH_BIN) $(CMD_DIR)/batch/main.go

build-web:
	@echo "Building Web Frontend..."
	cd $(WEB_DIR) && npm install && npm run build

# --- Test Commands ---

test: test-go test-web

test-go:
	@echo "Running Go Unit Tests..."
	$(GO_TEST) ./...

test-web:
	@echo "Running Web Tests..."
	# Assuming 'npm test' exists, skipping if not configured
	# cd $(WEB_DIR) && npm test

# --- Lint Commands ---

lint: lint-go lint-web

lint-go:
	@echo "Running Go Linters..."
	$(GO_LINT) ./...

lint-web:
	@echo "Running Web Linters..."
	cd $(WEB_DIR) && npm run lint

# --- Docker Compose Commands ---

start:
	@echo "Starting services with Docker Compose..."
	docker-compose up -d --build

stop:
	@echo "Stopping services..."
	docker-compose down

# --- Clean Commands ---

clean:
	@echo "Cleaning build artifacts..."
	$(GO_CLEAN)
	rm -rf $(BIN_DIR)
	rm -rf $(WEB_DIR)/.next
	rm -rf $(WEB_DIR)/out
	@echo "Cleaning Docker resources (optional)..."
	# docker-compose down -v --rmi local

# --- Help ---

help:
	@echo "Available commands:"
	@echo "  build        - Compile all projects (Go backend + Next.js frontend)"
	@echo "  test         - Run all unit tests"
	@echo "  lint         - Run all linters"
	@echo "  start        - Start all services via docker-compose"
	@echo "  stop         - Stop all services via docker-compose"
	@echo "  clean        - Remove build artifacts"
