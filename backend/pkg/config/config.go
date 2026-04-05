package config

import (
	"strings"
	"time"
)

// Config holds all configuration for the kvstore node.
type Config struct {
	NodeID            string        `mapstructure:"node_id"            yaml:"node_id"`
	ListenGRPC        string        `mapstructure:"listen_grpc"        yaml:"listen_grpc"`
	ListenHTTP        string        `mapstructure:"listen_http"        yaml:"listen_http"`
	DataDir           string        `mapstructure:"data_dir"           yaml:"data_dir"`
	SnapshotInterval  time.Duration `mapstructure:"snapshot_interval"  yaml:"snapshot_interval"`
	SnapshotThreshold int           `mapstructure:"snapshot_threshold" yaml:"snapshot_threshold"`
	WALMaxSizeBytes   int64         `mapstructure:"wal_max_size_bytes" yaml:"wal_max_size_bytes"`
	LogLevel          string        `mapstructure:"log_level"          yaml:"log_level"`

	// Cluster / gossip
	GossipBind string `mapstructure:"gossip_bind" yaml:"gossip_bind"` // host:port for memberlist
	SeedNodes  string `mapstructure:"seed_nodes"  yaml:"seed_nodes"`  // comma-separated host:port list

	// Replication
	ReplicationN int `mapstructure:"replication_n" yaml:"replication_n"`
	ReplicationR int `mapstructure:"replication_r" yaml:"replication_r"`
	ReplicationW int `mapstructure:"replication_w" yaml:"replication_w"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		NodeID:            "node-1",
		ListenGRPC:        ":50051",
		ListenHTTP:        ":8080",
		DataDir:           "./data",
		SnapshotInterval:  time.Minute,
		SnapshotThreshold: 10_000,
		WALMaxSizeBytes:   64 * 1024 * 1024, // 64 MB
		LogLevel:          "info",
		GossipBind:        "0.0.0.0:7946",
		ReplicationN:      3,
		ReplicationR:      2,
		ReplicationW:      2,
	}
}

// SeedNodeList parses the comma-separated SeedNodes string into a slice.
func (c *Config) SeedNodeList() []string {
	if c.SeedNodes == "" {
		return nil
	}
	var out []string
	for _, s := range strings.Split(c.SeedNodes, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
