package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcapi "github.com/yourusername/kvstore/pkg/api/grpc"
	httpapi "github.com/yourusername/kvstore/pkg/api/http"
	"github.com/yourusername/kvstore/pkg/cluster"
	"github.com/yourusername/kvstore/pkg/config"
	"github.com/yourusername/kvstore/pkg/consistency"
	"github.com/yourusername/kvstore/pkg/hash"
	"github.com/yourusername/kvstore/pkg/metrics"
	"github.com/yourusername/kvstore/pkg/replication"
	"github.com/yourusername/kvstore/pkg/storage"
	syncs "github.com/yourusername/kvstore/pkg/sync"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "", "path to config.yaml (optional)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	mustOK(err, "load config")

	logger, err := buildLogger(cfg.LogLevel)
	mustOK(err, "build logger")
	defer logger.Sync()

	logger.Info("kvstore starting",
		zap.String("node_id", cfg.NodeID),
		zap.String("data_dir", cfg.DataDir),
	)

	m := metrics.NewMetrics()

	// Storage engine
	engine, err := storage.Open(cfg, m, logger)
	mustOK(err, "open storage engine")

	// Consistent hash ring + cluster membership
	ring := hash.NewRing(hash.DefaultVirtualNodes)
	clusterMgr, err := cluster.NewManager(cluster.Config{
		NodeID:      cfg.NodeID,
		BindAddr:    cfg.GossipBind,
		GRPCAddress: cfg.ListenGRPC,
		HTTPAddress: cfg.ListenHTTP,
	}, ring, logger)
	mustOK(err, "create cluster manager")

	if err := clusterMgr.Join(cfg.SeedNodeList()); err != nil {
		logger.Warn("cluster join failed, continuing as single node", zap.Error(err))
	}

	// Replication coordinator
	replCfg := replication.Config{
		N:               cfg.ReplicationN,
		R:               cfg.ReplicationR,
		W:               cfg.ReplicationW,
		Timeout:         2 * time.Second,
		NodeID:          cfg.NodeID,
		NodeHTTPAddress: cfg.ListenHTTP,
	}
	coordinator := replication.NewCoordinator(replCfg, ring, engine, logger)

	coordinator.HintedHandoff().Start(func(nodeID string) string {
		for _, n := range ring.Nodes() {
			if n.ID == nodeID {
				return n.Address
			}
		}
		return ""
	})

	// Session manager (read-your-writes + monotonic reads)
	sessionMgr := consistency.NewManager()
	defer sessionMgr.Stop()

	// Anti-entropy (Merkle-tree-based background sync)
	getPeers := func() []syncs.PeerInfo {
		var peers []syncs.PeerInfo
		for _, n := range ring.Nodes() {
			peers = append(peers, syncs.PeerInfo{NodeID: n.ID, HTTPAddress: n.Address})
		}
		return peers
	}
	antiEntropy := syncs.NewAntiEntropy(cfg.NodeID, engine, getPeers, logger)
	antiEntropy.Start()
	defer antiEntropy.Stop()

	// Merkle builder — closure over local engine for the admin endpoint
	merkleBuilder := func() *syncs.MerkleTree {
		vals, _ := engine.Scan("", "\xff\xff\xff\xff")
		entries := make([]syncs.Entry, 0, len(vals))
		for _, v := range vals {
			entries = append(entries, syncs.Entry{Key: string(v.Data), Value: v.Data})
		}
		return syncs.Build(entries)
	}

	// gRPC server (uses local engine directly for internal RPCs)
	grpcSrv := grpcapi.NewServer(cfg.ListenGRPC, engine, m, logger)
	go func() {
		if err := grpcSrv.Start(); err != nil {
			logger.Error("gRPC server stopped", zap.Error(err))
		}
	}()

	// HTTP server
	httpSrv := httpapi.NewServer(cfg.ListenHTTP, coordinator, sessionMgr, merkleBuilder, m, cfg.NodeID, logger)
	go func() {
		if err := httpSrv.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server stopped", zap.Error(err))
		}
	}()

	logger.Info("kvstore ready",
		zap.String("grpc", cfg.ListenGRPC),
		zap.String("http", cfg.ListenHTTP),
		zap.Int("replication_n", cfg.ReplicationN),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clusterMgr.Leave(5 * time.Second)
	grpcSrv.Stop()
	httpSrv.Shutdown(ctx)
	engine.Close()

	logger.Info("kvstore stopped cleanly")
}

func buildLogger(level string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	switch level {
	case "debug":
		cfg.Level.SetLevel(zap.DebugLevel)
	case "warn":
		cfg.Level.SetLevel(zap.WarnLevel)
	case "error":
		cfg.Level.SetLevel(zap.ErrorLevel)
	default:
		cfg.Level.SetLevel(zap.InfoLevel)
	}
	return cfg.Build()
}

func mustOK(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL %s: %v\n", msg, err)
		os.Exit(1)
	}
}
