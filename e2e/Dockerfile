# Build stage
FROM golang:alpine AS builder

# Build arguments for version info (can be overridden at build time)
ARG VERSION=dev
ARG BUILD_TIME
ARG SERVICE_NAME=simple-service

WORKDIR /workspace

# Copy entire workspace (for local replace directive to work)
COPY . .

# Build the simple-service with LDFlags
WORKDIR /workspace/e2e
RUN go mod download
ENV CGO_ENABLED=0
ENV GOOS=linux
RUN go build \
    -ldflags="-X 'github.com/ecoma-io/observability.Version=${VERSION}' \
              -X 'github.com/ecoma-io/observability.BuildTime=${BUILD_TIME}' \
              -X 'github.com/ecoma-io/observability.ServiceName=${SERVICE_NAME}'" \
    -o simple-service .

# Runtime stage

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /workspace/e2e .

# Expose ports
EXPOSE 8080
EXPOSE 9090

# Run the service
CMD ["./simple-service"]
