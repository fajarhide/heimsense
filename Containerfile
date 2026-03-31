# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod first for caching
COPY go.mod ./
RUN go mod download || true

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /heimsense ./cmd/server/

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' appuser

# Copy binary from builder
COPY --from=builder /heimsense /app/heimsense

# Copy env example as reference
COPY env.example /app/env.example

# Set ownership
RUN chown -R appuser:appuser /app

USER appuser

# Environment defaults
ENV LISTEN_ADDR=:8080
ENV REQUEST_TIMEOUT_MS=120000
ENV MAX_RETRIES=3

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/heimsense"]
