# Build stage
FROM golang:alpine AS builder

ARG VERSION=dev
ARG BUILD_TIME
ARG SERVICE_NAME=simple-service
ARG SERVICE_DIR=simple-service

WORKDIR /workspace

# 1. Chỉ copy files định nghĩa dependency trước
COPY go.mod go.sum ./
# Nếu dự án có sử dụng 'replace' tới local modules, bạn cần copy cả go.mod của chúng
# Ví dụ: COPY internal/pkg/go.mod ./internal/pkg/

# 2. Download dependencies (Layer này sẽ được cached nếu go.mod/go.sum không đổi)
RUN go mod download

# 3. Copy toàn bộ source code sau khi đã download xong deps
COPY . .

# 4. Sử dụng BuildKit Cache Mount để tăng tốc compile
WORKDIR /workspace/examples/${SERVICE_DIR}
ENV CGO_ENABLED=0
ENV GOOS=linux

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build \
    -ldflags="-X 'github.com/ecoma-io/go-observability.Version=${VERSION}' \
              -X 'github.com/ecoma-io/go-observability.BuildTime=${BUILD_TIME}' \
              -X 'github.com/ecoma-io/go-observability.ServiceName=${SERVICE_NAME}'" \
    -o /app/service .

# Runtime stage
FROM gcr.io/distroless/static-debian12:debug
WORKDIR /app
COPY --from=builder /app/service .
EXPOSE 8080 9090
ENTRYPOINT ["/app/service"]
CMD []