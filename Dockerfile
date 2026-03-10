# Use a multi-stage build for production efficiency
FROM golang:1.24 AS backend-builder
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
