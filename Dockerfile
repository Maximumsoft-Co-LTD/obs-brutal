# Working Dockerfile for OBS-Brutal Simple Monitoring
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git curl

# Copy go files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the simple working demo
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o monitoring-demo simple_demo_working.go

# Production stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates curl tzdata

# Set timezone
ENV TZ=Asia/Bangkok

# Create user
RUN addgroup -g 1001 obsuser && \
    adduser -D -s /bin/sh -u 1001 -G obsuser obsuser

WORKDIR /app

# Copy binary
COPY --from=builder /app/monitoring-demo .

# Switch to non-root user
USER obsuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the full monitoring demo
CMD ["./monitoring-demo"]
