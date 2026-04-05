# ---- build stage ----
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /kvstore-server ./cmd/server
RUN go build -o /kvctl         ./cmd/cli

# ---- runtime stage ----
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /kvstore-server /app/kvstore-server
COPY --from=builder /kvctl          /app/kvctl

EXPOSE 8080 50051 7946

ENTRYPOINT ["/app/kvstore-server"]
