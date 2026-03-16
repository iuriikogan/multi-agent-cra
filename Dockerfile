# --- Stage 1: Frontend Builder ---
FROM node:22-alpine AS frontend-builder
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- Stage 2: Go Dependencies ---
FROM golang:1.25 AS go-deps
WORKDIR /app
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download

# --- Stage 3: Backend Source ---
FROM go-deps AS backend-source
# Copy ONLY Go source code, preventing frontend changes from busting the Go cache
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/

# --- Stage 4: Worker Builder ---
FROM backend-source AS worker-builder
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/app ./cmd/worker/main.go

# --- Stage 5: Server Builder ---
FROM backend-source AS server-builder
# Copy frontend assets into the server's embed directory
COPY --from=frontend-builder /web/out ./cmd/server/out
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/app ./cmd/server/main.go

# --- Stage 6: Base Runtime Image ---
FROM alpine:latest AS runtime
WORKDIR /app
RUN apk --no-cache add ca-certificates
EXPOSE 8080
CMD ["./app"]

# --- Final image for Worker ---
FROM runtime AS worker
COPY --from=worker-builder /app/bin/app ./app

# --- Final image for Server ---
FROM runtime AS server
COPY --from=server-builder /app/bin/app ./app
