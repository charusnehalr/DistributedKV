package http

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yourusername/kvstore/pkg/consistency"
	"github.com/yourusername/kvstore/pkg/metrics"
	"github.com/yourusername/kvstore/pkg/replication"
	syncs "github.com/yourusername/kvstore/pkg/sync"
	"go.uber.org/zap"
)

// clusterProvider is satisfied by *cluster.Manager.
type clusterProvider interface {
	ClusterMembers
}

// Server wraps the standard library HTTP server.
type Server struct {
	httpServer *http.Server
	logger     *zap.Logger
}

// NewServer wires all routes, middleware, and admin endpoints, then returns
// a ready-to-start Server.
func NewServer(
	addr string,
	coordinator *replication.Coordinator,
	sessions *consistency.Manager,
	merkleBuilder func() *syncs.MerkleTree,
	cluster clusterProvider,
	m *metrics.Metrics,
	nodeID string,
	logger *zap.Logger,
) *Server {
	h := NewHandlers(coordinator, sessions, merkleBuilder, cluster, m, nodeID, logger)

	r := mux.NewRouter()

	// Middleware — applied outermost first.
	r.Use(RecoveryMiddleware(logger))
	r.Use(LoggingMiddleware(logger))
	r.Use(CORSMiddleware([]string{"*"})) // tighten in production

	// Client-facing KV API
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/kv", h.HandlePut).Methods(http.MethodPost, http.MethodOptions)
	api.HandleFunc("/kv/{key}", h.HandleGet).Methods(http.MethodGet)
	api.HandleFunc("/kv/{key}", h.HandleDelete).Methods(http.MethodDelete)
	api.HandleFunc("/kv", h.HandleScan).Methods(http.MethodGet)
	api.HandleFunc("/health", h.HandleHealth).Methods(http.MethodGet)

	// Admin / anti-entropy endpoints (node-to-node)
	admin := api.PathPrefix("/admin").Subrouter()
	admin.HandleFunc("/merkle", h.HandleMerkle).Methods(http.MethodGet)
	admin.HandleFunc("/sync", h.HandleSyncPush).Methods(http.MethodPost)

	// Prometheus metrics
	r.Handle("/metrics", promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{}))

	return &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      r,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		logger: logger,
	}
}

// Start begins accepting HTTP connections. Blocks until the server stops.
func (s *Server) Start() error {
	s.logger.Info("HTTP server listening", zap.String("addr", s.httpServer.Addr))
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully drains connections within the given context deadline.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
