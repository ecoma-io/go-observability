# gRPC Service Example

This example demonstrates how to use the Ecoma Go Observability library with gRPC servers.

## Features Demonstrated

- ✅ Automatic request/response logging for unary and streaming RPCs
- ✅ Panic recovery with gRPC status errors
- ✅ Trace ID propagation in response metadata
- ✅ OpenTelemetry tracing integration
- ✅ Prometheus metrics collection
- ✅ Health check endpoint for readiness probes
- ✅ gRPC reflection for testing

## Running the Example

### 1. Set environment variables

```bash
export SERVICE_NAME=grpc-service
export LOG_LEVEL=info
export OTEL_ENDPOINT=localhost:4318
export METRICS_PORT=9090
export PORT=50051
export HEALTH_PORT=8080
```

If running a local (non-TLS) collector:

```bash
export OTEL_INSECURE=true
export METRICS_INSECURE=true
```

### 2. Run the service

```bash
cd examples/grpc-service
go run main.go
```

### 3. Test the service with grpcurl

**Install grpcurl:**

```bash
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

**List available services:**

```bash
grpcurl -plaintext localhost:50051 list
```

**Unary RPC - SayHello:**

```bash
grpcurl -plaintext -d '{"name": "World"}' localhost:50051 hello.HelloService/SayHello
```

**Server Streaming RPC - SayHelloStream:**

```bash
grpcurl -plaintext -d '{"name": "Stream"}' localhost:50051 hello.HelloService/SayHelloStream
```

**Trigger panic (recovery test):**

```bash
grpcurl -plaintext -d '{"name": "panic"}' localhost:50051 hello.HelloService/SayHello
```

**Test validation error:**

```bash
grpcurl -plaintext -d '{"name": ""}' localhost:50051 hello.HelloService/SayHello
```

**Health check:**

```bash
curl http://localhost:8080/health
```

## Expected Logs

### Success Request (Unary)

```json
{
  "level": "info",
  "timestamp": "2026-01-09T10:00:00.123Z",
  "msg": "gRPC Request",
  "service": "grpc-service",
  "method": "/hello.HelloService/SayHello",
  "grpc_code": "OK",
  "latency_ms": 12,
  "trace_id": "a1b2c3d4e5f6...",
  "span_id": "1234567890ab..."
}
```

### Streaming Request

```json
{
  "level": "info",
  "timestamp": "2026-01-09T10:00:00.456Z",
  "msg": "gRPC Stream Request",
  "service": "grpc-service",
  "method": "/hello.HelloService/SayHelloStream",
  "grpc_code": "OK",
  "latency_ms": 150,
  "is_client_stream": false,
  "is_server_stream": true,
  "trace_id": "a1b2c3d4e5f6...",
  "span_id": "1234567890ab..."
}
```

### Panic Recovery

```json
{
  "level": "error",
  "timestamp": "2026-01-09T10:00:00.789Z",
  "msg": "Panic recovered in gRPC handler",
  "service": "grpc-service",
  "error": "intentional panic for e2e testing",
  "trace_id": "a1b2c3d4e5f6...",
  "method": "/hello.HelloService/SayHello",
  "stack": "goroutine 42 [running]:\n..."
}
```

**Client receives:**

```
ERROR:
  Code: Internal
  Message: Internal server error occurred
```

### Client Error (Validation)

```json
{
  "level": "warn",
  "timestamp": "2026-01-09T10:00:01.000Z",
  "msg": "gRPC Client Error",
  "service": "grpc-service",
  "method": "/hello.HelloService/SayHello",
  "grpc_code": "InvalidArgument",
  "latency_ms": 1,
  "error": "rpc error: code = InvalidArgument desc = name cannot be empty"
}
```

## Integration with Full Stack

To see traces in Jaeger and metrics in Prometheus:

1. Start the E2E infrastructure:

```bash
cd ../../e2e
docker-compose up -d otel-collector jaeger prometheus
```

2. Run this example with proper OTEL endpoint:

```bash
export OTEL_ENDPOINT=localhost:4318
go run main.go
```

3. Access the UIs:

- Jaeger: http://localhost:16687
- Prometheus: http://localhost:9099
- Metrics endpoint: http://localhost:9090/metrics

## Code Structure

```go
// Create gRPC server with observability interceptors
grpcServer := grpc.NewServer(
    grpc.ChainUnaryInterceptor(
        observability.GrpcUnaryInterceptors(logger)...,
    ),
    grpc.ChainStreamInterceptor(
        observability.GrpcStreamInterceptors(logger)...,
    ),
)

// Or apply individually:
// grpc.NewServer(
//     grpc.UnaryInterceptor(observability.GrpcUnaryRecoveryInterceptor(logger)),
//     grpc.UnaryInterceptor(observability.GrpcUnaryServerInterceptor(logger)),
//     grpc.StreamInterceptor(observability.GrpcStreamRecoveryInterceptor(logger)),
//     grpc.StreamInterceptor(observability.GrpcStreamServerInterceptor(logger)),
// )
```

## Interceptor Order

The interceptors are applied in this order:

1. **GrpcUnaryRecoveryInterceptor / GrpcStreamRecoveryInterceptor** - Catches panics first
2. **GrpcUnaryServerInterceptor / GrpcStreamServerInterceptor** - Logs all requests (including
   recovered ones)

This ensures that even panic-recovered requests are properly logged.

## Protocol Buffers

The service uses Protocol Buffers for defining the API:

- **Definition**: [proto/hello.proto](proto/hello.proto)
- **Generated code**: `proto/hello.pb.go`, `proto/hello_grpc.pb.go`

To regenerate proto files:

```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       proto/hello.proto
```
