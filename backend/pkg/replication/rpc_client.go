// Package replication handles quorum-based reads/writes across the cluster.
package replication

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// RemoteValue is the result of a replica read.
type RemoteValue struct {
	NodeID    string
	Data      []byte
	Timestamp int64
	Found     bool
}

// RPCClient issues HTTP calls to remote kvstore nodes using the existing
// /api/v1/kv REST API. In a production system this would be replaced with gRPC.
type RPCClient struct {
	http    *http.Client
	timeout time.Duration
}

// NewRPCClient creates a client with the given per-call timeout.
func NewRPCClient(timeout time.Duration) *RPCClient {
	return &RPCClient{
		http:    &http.Client{Timeout: timeout},
		timeout: timeout,
	}
}

// Put calls PUT on a remote node.
func (c *RPCClient) Put(ctx context.Context, address, key string, value []byte) error {
	body, _ := json.Marshal(map[string]string{"key": key, "value": string(value)})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address+"/api/v1/kv", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("remote put failed: status %d", resp.StatusCode)
	}
	return nil
}

// Get calls GET on a remote node and returns the value with its timestamp.
func (c *RPCClient) Get(ctx context.Context, address, key string) (*RemoteValue, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/api/v1/kv/"+key, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &RemoteValue{Found: false}, nil
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("remote get failed: status %d", resp.StatusCode)
	}

	var envelope struct {
		Data struct {
			Value     string `json:"value"`
			Timestamp int64  `json:"timestamp"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode get response: %w", err)
	}
	return &RemoteValue{
		Data:      []byte(envelope.Data.Value),
		Timestamp: envelope.Data.Timestamp,
		Found:     true,
	}, nil
}

// Delete calls DELETE on a remote node.
func (c *RPCClient) Delete(ctx context.Context, address, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "http://"+address+"/api/v1/kv/"+key, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("remote delete failed: status %d", resp.StatusCode)
	}
	return nil
}
