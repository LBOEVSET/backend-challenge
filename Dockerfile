# ── Stage 1: build ───────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

# protoc + build tools
RUN apk add --no-cache protobuf make git

# protoc plugins (downloaded from Go module proxy)
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.2 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

WORKDIR /app

# Copy source so go mod tidy can resolve all imports including gRPC
COPY go.mod ./
COPY . .

# Generate proto stubs, resolve all deps, build with gRPC enabled
RUN protoc \
      --go_out=. --go_opt=paths=source_relative \
      --go-grpc_out=. --go-grpc_opt=paths=source_relative \
      proto/user/user.proto && \
    go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -tags grpc -o server ./cmd/server

# ── Stage 2: minimal runtime ──────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8080 9090
CMD ["./server"]
