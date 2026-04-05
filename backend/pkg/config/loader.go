package config

import (
	"strings"

	"github.com/spf13/viper"
)

// Load reads config from a YAML file (optional) and environment variables.
// Environment variables use the KVS_ prefix, e.g. KVS_DATA_DIR overrides data_dir.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults from DefaultConfig
	defaults := DefaultConfig()
	v.SetDefault("node_id", defaults.NodeID)
	v.SetDefault("listen_grpc", defaults.ListenGRPC)
	v.SetDefault("listen_http", defaults.ListenHTTP)
	v.SetDefault("data_dir", defaults.DataDir)
	v.SetDefault("snapshot_interval", defaults.SnapshotInterval)
	v.SetDefault("snapshot_threshold", defaults.SnapshotThreshold)
	v.SetDefault("wal_max_size_bytes", defaults.WALMaxSizeBytes)
	v.SetDefault("log_level", defaults.LogLevel)
	v.SetDefault("gossip_bind", defaults.GossipBind)
	v.SetDefault("seed_nodes", defaults.SeedNodes)
	v.SetDefault("replication_n", defaults.ReplicationN)
	v.SetDefault("replication_r", defaults.ReplicationR)
	v.SetDefault("replication_w", defaults.ReplicationW)

	// Environment variable support
	v.SetEnvPrefix("KVS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Config file (optional)
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
	}

	if err := v.ReadInConfig(); err != nil {
		// Config file is optional — only fail on real errors
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
