GOBIN     := $(shell go env GOPATH)/bin
GOOS      := $(shell go env GOOS)
BINARY_SERVER := bin/kvstore-server
BINARY_CLI    := bin/kvctl
PROTO_FILE    := proto/kvstore.proto

ifeq ($(GOOS),windows)
BINARY_SERVER := bin/kvstore-server.exe
BINARY_CLI    := bin/kvctl.exe
endif

.PHONY: all build proto test bench run clean

all: build

build:
	@mkdir -p bin
	go build -o $(BINARY_SERVER) ./cmd/server
	go build -o $(BINARY_CLI)    ./cmd/cli

proto:
	protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(PROTO_FILE)

test:
	go test ./... -v -race -count=1

bench:
	go test ./pkg/storage/... -bench=. -benchmem -benchtime=10s

run: build
	./$(BINARY_SERVER) --config config.yaml

clean:
	rm -rf bin/ data/

tidy:
	go mod tidy
