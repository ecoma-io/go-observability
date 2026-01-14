# Build stage
FROM golang:alpine AS builder

ARG VERSION=dev
ARG BUILD_TIME
ARG SERVICE_NAME=simple-service
ARG SERVICE_DIR=simple-service

WORKDIR /workspace

# 1. Only copy dependency definition files first
COPY go.mod go.sum ./
# If the project uses 'replace' to local modules, you need to copy their go.mod as well
# Example: COPY internal/pkg/go.mod ./internal/pkg/

# 2. Download dependencies (this layer will be cached if go.mod/go.sum do not change)
RUN go mod download

# 3. Copy the full source code after dependencies have been downloaded
COPY . .

# 4. Use BuildKit Cache Mount to speed up compilation
WORKDIR /workspace/examples/${SERVICE_DIR}
ENV CGO_ENABLED=0
ENV GOOS=linux

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build \
    -ldflags="-X github.com/ecoma-io/go-observability.Version=${VERSION} \
              -X github.com/ecoma-io/go-observability.BuildTime=${BUILD_TIME} \
              -X github.com/ecoma-io/go-observability.ServiceName=${SERVICE_NAME}" \
    -o /app/service .

# Runtime stage
FROM gcr.io/distroless/static-debian12:debug
WORKDIR /app
COPY --from=builder /app/service .
EXPOSE 8080 9090
ENTRYPOINT ["/app/service"]
CMD []