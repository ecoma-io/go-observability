package observability

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Mock request/response types
type mockRequest struct {
	Message string
}

type mockResponse struct {
	Message string
}

// Mock ServerStream for testing stream interceptors
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func (m *mockServerStream) SetTrailer(md metadata.MD) {
	// Mock implementation
}

func TestGrpcUnaryServerInterceptor(t *testing.T) {
	cfg := &BaseConfig{
		ServiceName: "test-grpc-service",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	tests := []struct {
		name           string
		handler        grpc.UnaryHandler
		expectedErr    bool
		expectedStatus codes.Code
	}{
		{
			name: "Success_Request",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return &mockResponse{Message: "success"}, nil
			},
			expectedErr:    false,
			expectedStatus: codes.OK,
		},
		{
			name: "NotFound_Error",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, status.Error(codes.NotFound, "resource not found")
			},
			expectedErr:    true,
			expectedStatus: codes.NotFound,
		},
		{
			name: "Internal_Error",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, status.Error(codes.Internal, "internal server error")
			},
			expectedErr:    true,
			expectedStatus: codes.Internal,
		},
		{
			name: "InvalidArgument_Error",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, status.Error(codes.InvalidArgument, "invalid argument")
			},
			expectedErr:    true,
			expectedStatus: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := GrpcUnaryServerInterceptor(logger)

			info := &grpc.UnaryServerInfo{
				FullMethod: "/test.Service/TestMethod",
			}

			ctx := context.Background()
			req := &mockRequest{Message: "test"}

			resp, err := interceptor(ctx, req, info, tt.handler)

			if tt.expectedErr && err == nil {
				t.Errorf("Expected error but got nil")
			}

			if !tt.expectedErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if err != nil {
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("Expected gRPC status error")
				}
				if st.Code() != tt.expectedStatus {
					t.Errorf("Expected status %v, got %v", tt.expectedStatus, st.Code())
				}
			}

			if !tt.expectedErr && resp == nil {
				t.Errorf("Expected response but got nil")
			}
		})
	}
}

func TestGrpcStreamServerInterceptor(t *testing.T) {
	cfg := &BaseConfig{
		ServiceName: "test-grpc-service",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	tests := []struct {
		name           string
		handler        grpc.StreamHandler
		expectedErr    bool
		expectedStatus codes.Code
	}{
		{
			name: "Success_Stream",
			handler: func(srv interface{}, stream grpc.ServerStream) error {
				return nil
			},
			expectedErr:    false,
			expectedStatus: codes.OK,
		},
		{
			name: "Canceled_Stream",
			handler: func(srv interface{}, stream grpc.ServerStream) error {
				return status.Error(codes.Canceled, "stream canceled")
			},
			expectedErr:    true,
			expectedStatus: codes.Canceled,
		},
		{
			name: "Internal_Error_Stream",
			handler: func(srv interface{}, stream grpc.ServerStream) error {
				return status.Error(codes.Internal, "internal error")
			},
			expectedErr:    true,
			expectedStatus: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := GrpcStreamServerInterceptor(logger)

			info := &grpc.StreamServerInfo{
				FullMethod:     "/test.Service/TestStreamMethod",
				IsClientStream: true,
				IsServerStream: true,
			}

			ctx := context.Background()
			stream := &mockServerStream{ctx: ctx}

			err := interceptor(nil, stream, info, tt.handler)

			if tt.expectedErr && err == nil {
				t.Errorf("Expected error but got nil")
			}

			if !tt.expectedErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if err != nil {
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("Expected gRPC status error")
				}
				if st.Code() != tt.expectedStatus {
					t.Errorf("Expected status %v, got %v", tt.expectedStatus, st.Code())
				}
			}
		})
	}
}

func TestGrpcUnaryRecoveryInterceptor(t *testing.T) {
	cfg := &BaseConfig{
		ServiceName: "test-grpc-service",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	tests := []struct {
		name        string
		handler     grpc.UnaryHandler
		shouldPanic bool
	}{
		{
			name: "Normal_Request_No_Panic",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return &mockResponse{Message: "success"}, nil
			},
			shouldPanic: false,
		},
		{
			name: "Panic_In_Handler",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				panic("something went wrong!")
			},
			shouldPanic: true,
		},
		{
			name: "Panic_With_Error",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				panic(errors.New("error panic"))
			},
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := GrpcUnaryRecoveryInterceptor(logger)

			info := &grpc.UnaryServerInfo{
				FullMethod: "/test.Service/TestMethod",
			}

			ctx := context.Background()
			req := &mockRequest{Message: "test"}

			resp, err := interceptor(ctx, req, info, tt.handler)

			if tt.shouldPanic {
				if err == nil {
					t.Errorf("Expected error from panic recovery but got nil")
				}

				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("Expected gRPC status error")
				}

				if st.Code() != codes.Internal {
					t.Errorf("Expected Internal status code, got %v", st.Code())
				}

				if resp != nil {
					t.Errorf("Expected nil response after panic, got %v", resp)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				if resp == nil {
					t.Errorf("Expected response but got nil")
				}
			}
		})
	}
}

func TestGrpcStreamRecoveryInterceptor(t *testing.T) {
	cfg := &BaseConfig{
		ServiceName: "test-grpc-service",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	tests := []struct {
		name        string
		handler     grpc.StreamHandler
		shouldPanic bool
	}{
		{
			name: "Normal_Stream_No_Panic",
			handler: func(srv interface{}, stream grpc.ServerStream) error {
				return nil
			},
			shouldPanic: false,
		},
		{
			name: "Panic_In_Stream_Handler",
			handler: func(srv interface{}, stream grpc.ServerStream) error {
				panic("stream panic!")
			},
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := GrpcStreamRecoveryInterceptor(logger)

			info := &grpc.StreamServerInfo{
				FullMethod:     "/test.Service/TestStreamMethod",
				IsClientStream: true,
				IsServerStream: true,
			}

			ctx := context.Background()
			stream := &mockServerStream{ctx: ctx}

			err := interceptor(nil, stream, info, tt.handler)

			if tt.shouldPanic {
				if err == nil {
					t.Errorf("Expected error from panic recovery but got nil")
				}

				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("Expected gRPC status error")
				}

				if st.Code() != codes.Internal {
					t.Errorf("Expected Internal status code, got %v", st.Code())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestGrpcUnaryInterceptors(t *testing.T) {
	cfg := &BaseConfig{
		ServiceName: "test-grpc-service",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	interceptors := GrpcUnaryInterceptors(logger)

	if len(interceptors) != 2 {
		t.Errorf("Expected 2 interceptors, got %d", len(interceptors))
	}

	// Test that interceptors are not nil
	for i, interceptor := range interceptors {
		if interceptor == nil {
			t.Errorf("Interceptor at index %d is nil", i)
		}
	}
}

func TestGrpcStreamInterceptors(t *testing.T) {
	cfg := &BaseConfig{
		ServiceName: "test-grpc-service",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	interceptors := GrpcStreamInterceptors(logger)

	if len(interceptors) != 2 {
		t.Errorf("Expected 2 interceptors, got %d", len(interceptors))
	}

	// Test that interceptors are not nil
	for i, interceptor := range interceptors {
		if interceptor == nil {
			t.Errorf("Interceptor at index %d is nil", i)
		}
	}
}
