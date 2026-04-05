package grpc

import (
	"context"
	"net"
	"time"

	"github.com/yourusername/kvstore/pkg/metrics"
	"github.com/yourusername/kvstore/pkg/storage"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Server wraps a gRPC server and its listener.
type Server struct {
	grpcServer *grpc.Server
	addr       string
	logger     *zap.Logger
}

// NewServer creates a gRPC server with logging and metrics interceptors.
func NewServer(addr string, engine storage.Engine, m *metrics.Metrics, logger *zap.Logger) *Server {
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			loggingInterceptor(logger),
			metricsInterceptor(m),
		),
	}

	s := grpc.NewServer(opts...)
	RegisterKVStoreServer(s, newHandlers(engine, logger))
	reflection.Register(s) // enables grpcurl without proto files

	return &Server{grpcServer: s, addr: addr, logger: logger}
}

// Start begins accepting connections. Blocks until the server is stopped.
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.logger.Info("gRPC server listening", zap.String("addr", s.addr))
	return s.grpcServer.Serve(lis)
}

// Stop gracefully drains in-flight RPCs and shuts down.
func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
}

// loggingInterceptor logs each RPC call with method name and duration.
func loggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		logger.Info("grpc",
			zap.String("method", info.FullMethod),
			zap.Duration("duration", time.Since(start)),
			zap.Error(err),
		)
		return resp, err
	}
}

// metricsInterceptor records per-RPC operation durations.
func metricsInterceptor(m *metrics.Metrics) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		m.OperationDuration.WithLabelValues("grpc").Observe(time.Since(start).Seconds())
		return resp, err
	}
}
