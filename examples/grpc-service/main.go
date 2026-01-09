package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/ecoma-io/go-observability"
	pb "github.com/ecoma-io/go-observability/examples/grpc-service/proto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type Config struct {
	observability.BaseConfig
	Port       int `env:"PORT" env-default:"50051"`
	HealthPort int `env:"HEALTH_PORT" env-default:"8080"`
}

var (
	logger       *observability.Logger
	requestCount atomic.Int32
)

// HelloServer implements the HelloService
type HelloServer struct {
	pb.UnimplementedHelloServiceServer
}

func (s *HelloServer) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloResponse, error) {
	// Create span for tracing
	tracer := otel.Tracer("grpc-service")
	ctx, span := tracer.Start(ctx, "SayHello")
	defer span.End()

	span.SetAttributes(attribute.String("request.name", req.Name))

	// Validate input
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name cannot be empty")
	}

	// Simulate panic for testing recovery
	if req.Name == "panic" {
		panic("intentional panic for e2e testing")
	}

	// Increment counter
	count := requestCount.Add(1)

	// Update metrics
	meter := otel.Meter("grpc-service")
	counter, _ := meter.Int64Counter("grpc_request_count_total")
	counter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("method", "SayHello"),
		attribute.String("type", "unary"),
	))

	// Artificial delay for tracing
	time.Sleep(10 * time.Millisecond)

	message := fmt.Sprintf("Hello, %s! (request #%d)", req.Name, count)

	return &pb.HelloResponse{
		Message:      message,
		RequestCount: count,
	}, nil
}

func (s *HelloServer) SayHelloStream(req *pb.HelloRequest, stream pb.HelloService_SayHelloStreamServer) error {
	// Create span for tracing
	tracer := otel.Tracer("grpc-service")
	ctx, span := tracer.Start(stream.Context(), "SayHelloStream")
	defer span.End()

	span.SetAttributes(attribute.String("request.name", req.Name))

	// Validate input
	if req.Name == "" {
		return status.Error(codes.InvalidArgument, "name cannot be empty")
	}

	// Update metrics
	meter := otel.Meter("grpc-service")
	counter, _ := meter.Int64Counter("grpc_stream_count_total")
	counter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("method", "SayHelloStream"),
		attribute.String("type", "server_stream"),
	))

	// Send 3 greetings
	for i := 1; i <= 3; i++ {
		count := requestCount.Add(1)
		message := fmt.Sprintf("Hello #%d, %s! (request #%d)", i, req.Name, count)

		if err := stream.Send(&pb.HelloResponse{
			Message:      message,
			RequestCount: count,
		}); err != nil {
			return err
		}

		time.Sleep(50 * time.Millisecond)
	}

	return nil
}

func main() {
	// 1. Load Config
	var cfg Config
	if err := observability.LoadCfg(&cfg); err != nil {
		panic(err)
	}

	// 2. Init Logger
	logger = observability.NewLogger(&cfg.BaseConfig)
	defer logger.Sync()

	logger.Info("Starting grpc-service",
		"version", cfg.Version,
		"grpc_port", cfg.Port,
		"health_port", cfg.HealthPort,
	)

	// 3. Init Otel
	shutdown, err := observability.InitOtel(cfg.BaseConfig)
	if err != nil {
		logger.Fatal("Failed to init Otel", "error", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			logger.Error("Failed to shutdown Otel", "error", err)
		}
	}()

	// 4. Create gRPC server with observability interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			observability.GrpcUnaryInterceptors(logger)...,
		),
		grpc.ChainStreamInterceptor(
			observability.GrpcStreamInterceptors(logger)...,
		),
	)

	// Register services
	pb.RegisterHelloServiceServer(grpcServer, &HelloServer{})

	// Register health check service
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	// Enable reflection for grpcurl
	reflection.Register(grpcServer)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		logger.Fatal("Failed to listen", "error", err)
	}

	// Start HTTP health check endpoint in a goroutine
	go startHealthServer(cfg.HealthPort)

	logger.Info("gRPC server listening", "addr", lis.Addr())
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("Failed to serve", "error", err)
	}
}

// startHealthServer starts an HTTP server for health checks and readiness probes
func startHealthServer(port int) {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	})

	addr := fmt.Sprintf(":%d", port)
	logger.Info("Health server listening", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Error("Health server error", "error", err)
	}
}
