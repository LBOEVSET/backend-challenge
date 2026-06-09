//go:build grpc

package main

import (
	"log"

	grpcadapter "github.com/lboevset/backend-challenge/internal/adapter/grpc"
	"github.com/lboevset/backend-challenge/internal/application"
)

// startGRPC launches the gRPC server in a background goroutine.
// Called from main() when the binary is compiled with -tags grpc.
func startGRPC(svc *application.UserService, port string) {
	go func() {
		if err := grpcadapter.ListenAndServe(svc, ":"+port); err != nil {
			log.Printf("[gRPC] fatal: %v", err)
		}
	}()
}
