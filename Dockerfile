# Use a multi-stage build for production efficiency
# Build stage
FROM golang:1.25 AS builder

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build argument to specify which binary to build (server or worker)
ARG TARGET=server

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/app ./cmd/${TARGET}/main.go

# Run stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for external API calls
RUN apk --no-cache add ca-certificates

# Copy the binary from the builder stage
COPY --from=builder /app/bin/app ./app

# Expose port (default for server)
EXPOSE 8080

# Set entrypoint
CMD ["./app"]