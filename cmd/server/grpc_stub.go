//go:build !grpc

package main

import "github.com/lboevset/backend-challenge/internal/application"

// startGRPC is a no-op when the binary is built without -tags grpc.
// Build with: go build -tags grpc ./cmd/server
func startGRPC(_ *application.UserService, _ string) {}
