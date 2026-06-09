.PHONY: run run-grpc test build build-grpc proto proto-tools docker-up docker-down

# Run the HTTP-only server locally (no gRPC, no protoc required)
run:
	go run ./cmd/server

# Run with gRPC enabled (requires `make proto` first)
run-grpc:
	go run -tags grpc ./cmd/server

# Run all unit tests
test:
	go test ./... -v -count=1

# Build HTTP-only binary
build:
	CGO_ENABLED=0 go build -o bin/server ./cmd/server

# Build with gRPC enabled (requires `make proto` first)
build-grpc:
	CGO_ENABLED=0 go build -tags grpc -o bin/server ./cmd/server

# Install protoc compiler plugins (one-time setup)
#   Prerequisites: brew install protobuf  (macOS)
#                  sudo apt install -y protobuf-compiler  (Ubuntu)
proto-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate Go code from the proto file
proto: proto-tools
	protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/user/user.proto

# Start the full stack (API + MongoDB) with Docker Compose
docker-up:
	docker compose up --build

docker-down:
	docker compose down -v
