package grpc

import (
	"context"
	"errors"

	"github.com/yourusername/kvstore/pkg/storage"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---- Minimal inline proto types (replaced by generated code after `make proto`) ----
// These allow the project to compile before protoc is run.

type ConsistencyLevel int32

const (
	ConsistencyLevel_ONE    ConsistencyLevel = 1
	ConsistencyLevel_QUORUM ConsistencyLevel = 2
	ConsistencyLevel_ALL    ConsistencyLevel = 3
)

type PutRequest  struct{ Key string; Value []byte; Consistency ConsistencyLevel }
type PutResponse struct{ Timestamp int64 }

type GetRequest  struct{ Key string; Consistency ConsistencyLevel }
type GetResponse struct{ Key string; Value []byte; Timestamp int64 }

type DeleteRequest  struct{ Key string; Consistency ConsistencyLevel }
type DeleteResponse struct{ Success bool }

type ScanRequest  struct{ StartKey, EndKey string; Limit int32 }
type ScanResponse struct{ Entries []*GetResponse }

// KVStoreServer is the interface all handler implementations must satisfy.
type KVStoreServer interface {
	Put(context.Context, *PutRequest) (*PutResponse, error)
	Get(context.Context, *GetRequest) (*GetResponse, error)
	Delete(context.Context, *DeleteRequest) (*DeleteResponse, error)
	Scan(context.Context, *ScanRequest) (*ScanResponse, error)
}

// RegisterKVStoreServer is a stub until generated code is in place.
// After `make proto` this file is replaced by the generated registration function.
func RegisterKVStoreServer(_ *grpc.Server, _ KVStoreServer) {}

// ---- Handler implementation ----

type handlers struct {
	engine storage.Engine
	logger *zap.Logger
}

func newHandlers(engine storage.Engine, logger *zap.Logger) *handlers {
	return &handlers{engine: engine, logger: logger}
}

func storageErrToGRPC(err error) error {
	switch {
	case errors.Is(err, storage.ErrKeyNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, storage.ErrEmptyKey):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, storage.ErrEngineClosed):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func (h *handlers) Put(ctx context.Context, req *PutRequest) (*PutResponse, error) {
	if err := h.engine.Put(req.Key, req.Value); err != nil {
		return nil, storageErrToGRPC(err)
	}
	return &PutResponse{}, nil
}

func (h *handlers) Get(ctx context.Context, req *GetRequest) (*GetResponse, error) {
	val, err := h.engine.Get(req.Key)
	if err != nil {
		return nil, storageErrToGRPC(err)
	}
	return &GetResponse{
		Key:       req.Key,
		Value:     val.Data,
		Timestamp: val.Timestamp,
	}, nil
}

func (h *handlers) Delete(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error) {
	if err := h.engine.Delete(req.Key); err != nil {
		return nil, storageErrToGRPC(err)
	}
	return &DeleteResponse{Success: true}, nil
}

func (h *handlers) Scan(ctx context.Context, req *ScanRequest) (*ScanResponse, error) {
	end := req.EndKey
	if end == "" {
		end = req.StartKey + "\xff"
	}
	vals, err := h.engine.Scan(req.StartKey, end)
	if err != nil {
		return nil, storageErrToGRPC(err)
	}
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 1000
	}
	if len(vals) > limit {
		vals = vals[:limit]
	}
	resp := &ScanResponse{}
	for _, v := range vals {
		resp.Entries = append(resp.Entries, &GetResponse{Value: v.Data, Timestamp: v.Timestamp})
	}
	return resp, nil
}
