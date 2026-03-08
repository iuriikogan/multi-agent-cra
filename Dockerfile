# Use a multi-stage build for production efficiency

# Stage 1: Build Frontend (Node.js) ---
FROM node:20-alpine AS frontend-builder
WORKDIR /app/web
# Copy package.json and lock files
COPY web/package.json web/package-lock.json* ./
# Install dependencies
RUN npm install
# Copy source code
COPY web/ .
# Build Next.js app (static export)
RUN npm run build

# --- Stage 2: Build Backend (Go) ---
FROM golang:1.25 AS backend-builder
WORKDIR /app

# Copy Go dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend assets to the expected embed location
# Note: Ensure cmd/server/out exists or is created
COPY --from=frontend-builder /app/web/out ./cmd/server/out

# Build arguments
ARG TARGET=server

# Build the binary
# We only build the target requested. 
# For 'server', it will include the embedded assets.
# For 'worker', it ignores them.
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/app ./cmd/${TARGET}/main.go

# --- Stage 3: Final Runtime Image ---
FROM alpine:latest
WORKDIR /app

# Install ca-certificates for external API calls
RUN apk --no-cache add ca-certificates

# Copy the binary from the backend builder
COPY --from=backend-builder /app/bin/app ./app

# Expose port
EXPOSE 8080

# Set entrypoint
CMD ["./app"]
