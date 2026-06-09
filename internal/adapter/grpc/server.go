//go:build grpc

// Package grpcadapter is the gRPC delivery adapter.
// It is only compiled when building with -tags grpc (e.g. the Docker build).
//
// Local development:
//  1. Install protoc:          brew install protobuf
//  2. Install plugins:         make proto-tools
//  3. Generate stubs:          make proto
//  4. Run with gRPC enabled:   go run -tags grpc ./cmd/server
package grpcadapter

import (
	"context"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/lboevset/backend-challenge/internal/application"
	pb "github.com/lboevset/backend-challenge/proto/user"
)

// userServiceServer wraps the application service to satisfy the gRPC interface.
type userServiceServer struct {
	pb.UnimplementedUserServiceServer
	svc *application.UserService
}

// CreateUser registers a new user and returns the created record.
func (s *userServiceServer) CreateUser(
	ctx context.Context,
	req *pb.CreateUserRequest,
) (*pb.CreateUserResponse, error) {
	user, err := s.svc.Register(ctx, application.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return nil, status.Error(codes.AlreadyExists, err.Error())
	}
	return &pb.CreateUserResponse{
		Id:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	}, nil
}

// GetUser retrieves a user by ID.
func (s *userServiceServer) GetUser(
	ctx context.Context,
	req *pb.GetUserRequest,
) (*pb.GetUserResponse, error) {
	user, err := s.svc.GetUser(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if user == nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return &pb.GetUserResponse{
		Id:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	}, nil
}

// New builds a gRPC server with UserService registered and reflection enabled
// (so grpcurl / Postman can discover the service without a .proto file).
func New(svc *application.UserService) *grpc.Server {
	s := grpc.NewServer()
	pb.RegisterUserServiceServer(s, &userServiceServer{svc: svc})
	reflection.Register(s)
	return s
}

// ListenAndServe starts the gRPC server on addr (e.g. ":9090").
// Blocks until the server stops.
func ListenAndServe(svc *application.UserService, addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s := New(svc)
	log.Printf("[gRPC] server listening on %s", addr)
	return s.Serve(lis)
}
