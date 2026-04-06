# DistributedKV

A distributed key-value store built from scratch in Go — inspired by the internals of DynamoDB and Cassandra.

---

## Why I Built This

I was taking a distributed systems course and kept hitting the same wall: the papers made sense, but I had no intuition for *why* any of it was hard. CAP theorem is easy to recite. It's much harder to experience what happens when your cluster partitions mid-write and two nodes diverge.

So I stopped reading and started building. The goal was to encounter — and solve — the real problems.

---

## Problems I Had to Solve

**Crash recovery** — If the server dies mid-write, how do you know what was committed? I built a Write-Ahead Log that fsyncs every record before touching the memtable. On restart, the engine replays only from the last snapshot's byte offset — O(recent writes), not O(all history).

**Data distribution** — Traditional modulo hashing reshuffles most keys when a node joins or leaves. I implemented a consistent hash ring with 150 virtual nodes per physical node. Only ~1/N keys move on topology changes. Nodes discover each other automatically via gossip.

**Stale replicas** — A write that reaches 2 of 3 replicas leaves the third stale. Three things keep it in sync: hinted handoff (coordinator stores missed writes and replays them when the node recovers), read repair (every read pushes the latest value to stale replicas in the background), and Merkle anti-entropy (background goroutine compares 1024-bucket trees between peers and syncs only divergent buckets).

**Conflict resolution** — Wall clocks lie. I implemented vector clocks (`map[nodeID]uint64`) so the system can determine whether two versions have a causal relationship or are genuinely concurrent — the same approach DynamoDB uses.

**Session guarantees** — In a leaderless system, a write to node-1 might not be visible yet on node-3. A session manager tracks each client's last write and read timestamps. Every GET response includes `read_your_write` and `monotonic` flags so clients know exactly what consistency they got.

---

## Stack

**Backend** — Go, gRPC + Protobuf, Hashicorp memberlist (gossip), Prometheus, Uber Zap, google/btree

**Frontend** — React 18, TypeScript, Vite, TanStack Query, Zustand, Recharts, Tailwind CSS

**Infra** — Docker Compose (3-node cluster), Prometheus, Grafana, GitHub Actions CI

---

## Running It

```bash
# Single node
cd backend && go run ./cmd/server
cd frontend && npm install && npm run dev   # http://localhost:5173

# 3-node cluster (Docker)
cd deployment/docker && docker compose up --build -d
# nodes: :8080 / :8081 / :8082 · Grafana: :3000 · Prometheus: :9090

# Tests
cd backend && go test ./... -race -timeout 120s
```

---

## API

```
POST   /api/v1/kv          {"key":"...","value":"...","consistency":"quorum"}
GET    /api/v1/kv/{key}
DELETE /api/v1/kv/{key}
GET    /api/v1/kv?prefix=user:
GET    /api/v1/health
GET    /metrics
```

Consistency levels: `one` · `quorum` (default) · `all`

---

## What I Learned

Eventual consistency is *more* work than strong consistency — you have to build every mechanism that makes convergence actually happen. Strong consistency is simple: one leader, everyone follows. Eventual consistency requires versioning, anti-entropy, conflict detection, and session tracking. Each one is a real engineering problem, not a footnote.

---

*~6,000 lines of Go and TypeScript. Every component written from scratch.*
