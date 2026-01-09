package observability

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GrpcUnaryServerInterceptor logs gRPC unary requests with OpenTelemetry trace context
func GrpcUnaryServerInterceptor(logger *Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		// Extract trace context if available
		span := trace.SpanFromContext(ctx)
		spanContext := span.SpanContext()
		traceID := spanContext.TraceID().String()
		spanID := spanContext.SpanID().String()

		// Call the handler
		resp, err := handler(ctx, req)

		// Calculate latency
		latency := time.Since(start)

		// Extract gRPC status
		grpcStatus := status.Code(err)

		// Build log fields
		fields := []interface{}{
			"method", info.FullMethod,
			"grpc_code", grpcStatus.String(),
			"latency_ms", latency.Milliseconds(),
		}

		// Add trace context if present
		if traceID != "" && traceID != "00000000000000000000000000000000" {
			fields = append(fields, "trace_id", traceID)
		}
		if spanID != "" && spanID != "0000000000000000" {
			fields = append(fields, "span_id", spanID)
		}

		// Add error if present
		if err != nil {
			fields = append(fields, "error", err.Error())
		}

		// Log based on gRPC status code
		switch grpcStatus {
		case codes.OK:
			logger.Info("gRPC Request", fields...)
		case codes.Canceled, codes.InvalidArgument, codes.NotFound, codes.AlreadyExists,
			codes.PermissionDenied, codes.Unauthenticated, codes.FailedPrecondition,
			codes.OutOfRange:
			logger.Warn("gRPC Client Error", fields...)
		default:
			logger.Error("gRPC Server Error", fields...)
		}

		return resp, err
	}
}

// GrpcStreamServerInterceptor logs gRPC streaming requests with OpenTelemetry trace context
func GrpcStreamServerInterceptor(logger *Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		ctx := stream.Context()

		// Extract trace context if available
		span := trace.SpanFromContext(ctx)
		spanContext := span.SpanContext()
		traceID := spanContext.TraceID().String()
		spanID := spanContext.SpanID().String()

		// Call the handler
		err := handler(srv, stream)

		// Calculate latency
		latency := time.Since(start)

		// Extract gRPC status
		grpcStatus := status.Code(err)

		// Build log fields
		fields := []interface{}{
			"method", info.FullMethod,
			"grpc_code", grpcStatus.String(),
			"latency_ms", latency.Milliseconds(),
			"is_client_stream", info.IsClientStream,
			"is_server_stream", info.IsServerStream,
		}

		// Add trace context if present
		if traceID != "" && traceID != "00000000000000000000000000000000" {
			fields = append(fields, "trace_id", traceID)
		}
		if spanID != "" && spanID != "0000000000000000" {
			fields = append(fields, "span_id", spanID)
		}

		// Add error if present
		if err != nil {
			fields = append(fields, "error", err.Error())
		}

		// Log based on gRPC status code
		switch grpcStatus {
		case codes.OK:
			logger.Info("gRPC Stream Request", fields...)
		case codes.Canceled, codes.InvalidArgument, codes.NotFound, codes.AlreadyExists,
			codes.PermissionDenied, codes.Unauthenticated, codes.FailedPrecondition,
			codes.OutOfRange:
			logger.Warn("gRPC Stream Client Error", fields...)
		default:
			logger.Error("gRPC Stream Server Error", fields...)
		}

		return err
	}
}

// GrpcUnaryRecoveryInterceptor recovers from panics in gRPC unary handlers
func GrpcUnaryRecoveryInterceptor(logger *Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				// Extract trace context
				span := trace.SpanFromContext(ctx)
				spanContext := span.SpanContext()
				traceID := spanContext.TraceID().String()

				// Get stack trace
				stack := string(debug.Stack())

				// Log the panic with full context
				logger.Error("Panic recovered in gRPC handler",
					"error", fmt.Sprintf("%v", r),
					"trace_id", traceID,
					"method", info.FullMethod,
					"stack", stack,
				)

				// Inject trace_id into response metadata if available
				if traceID != "" && traceID != "00000000000000000000000000000000" {
					md := metadata.Pairs("trace_id", traceID)
					if err := setTrailer(ctx, md); err != nil {
						logger.Warn("failed to set trailer", "error", err)
					}
				}

				// Return Internal error
				err = status.Errorf(codes.Internal, "Internal server error occurred")
			}
		}()

		return handler(ctx, req)
	}
}

// setTrailer is a thin wrapper around grpc.SetTrailer to satisfy linters
func setTrailer(ctx context.Context, md metadata.MD) error {
	// grpc.SetTrailer does not return an error; suppress errcheck here
	// nolint:errcheck
	grpc.SetTrailer(ctx, md)
	return nil
}

// GrpcStreamRecoveryInterceptor recovers from panics in gRPC streaming handlers
func GrpcStreamRecoveryInterceptor(logger *Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if r := recover(); r != nil {
				ctx := stream.Context()

				// Extract trace context
				span := trace.SpanFromContext(ctx)
				spanContext := span.SpanContext()
				traceID := spanContext.TraceID().String()

				// Get stack trace
				stack := string(debug.Stack())

				// Log the panic with full context
				logger.Error("Panic recovered in gRPC stream handler",
					"error", fmt.Sprintf("%v", r),
					"trace_id", traceID,
					"method", info.FullMethod,
					"is_client_stream", info.IsClientStream,
					"is_server_stream", info.IsServerStream,
					"stack", stack,
				)

				// Inject trace_id into response metadata if available
				if traceID != "" && traceID != "00000000000000000000000000000000" {
					md := metadata.Pairs("trace_id", traceID)
					stream.SetTrailer(md)
				}

				// Return Internal error
				err = status.Errorf(codes.Internal, "Internal server error occurred")
			}
		}()

		return handler(srv, stream)
	}
}

// GrpcUnaryInterceptors returns a chain of unary interceptors (recovery + logging)
// Usage: grpc.NewServer(grpc.ChainUnaryInterceptor(observability.GrpcUnaryInterceptors(logger)...))
func GrpcUnaryInterceptors(logger *Logger) []grpc.UnaryServerInterceptor {
	return []grpc.UnaryServerInterceptor{
		GrpcUnaryRecoveryInterceptor(logger),
		GrpcUnaryServerInterceptor(logger),
	}
}

// GrpcStreamInterceptors returns a chain of stream interceptors (recovery + logging)
// Usage: grpc.NewServer(grpc.ChainStreamInterceptor(observability.GrpcStreamInterceptors(logger)...))
func GrpcStreamInterceptors(logger *Logger) []grpc.StreamServerInterceptor {
	return []grpc.StreamServerInterceptor{
		GrpcStreamRecoveryInterceptor(logger),
		GrpcStreamServerInterceptor(logger),
	}
}
