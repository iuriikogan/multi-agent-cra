# Use a multi-stage build for production efficiency

# --- Stage 1: Frontend Builder ---
FROM node:20-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# --- Stage 2: Backend Builder ---
FROM golang:1.25 AS backend-builder
WORKDIR /app
ENV GOTOOLCHAIN=auto

# Copy Go dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments
ARG TARGET=server

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/app ./cmd/${TARGET}/main.go

# --- Stage 3: Final Runtime Image ---
FROM alpine:latest
WORKDIR /app

# Install ca-certificates for external API calls
RUN apk --no-cache add ca-certificates

# Copy the binary from the backend builder
COPY --from=backend-builder /app/bin/app ./app

# Copy the static Next.js export
COPY --from=frontend-builder /app/web/out ./web/out

# Expose port
EXPOSE 8080

# Set entrypoint
CMD ["./app"]
