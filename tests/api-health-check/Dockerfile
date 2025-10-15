# Multi-stage build for minimal image size
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Copy go module files
COPY go.mod go.sum* ./

# Download dependencies (if any)
RUN go mod download || true

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api-health-check .

# Final stage - minimal runtime image
FROM alpine:latest

# Add ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/api-health-check .

# Create directory for reports
RUN mkdir -p /tmp/reports

# Run as non-root user
RUN adduser -D -u 1000 healthcheck
USER healthcheck

ENTRYPOINT ["/app/api-health-check"]
