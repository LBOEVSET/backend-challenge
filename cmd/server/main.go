package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/lboevset/backend-challenge/internal/adapter/mongodb"
	httpadapter "github.com/lboevset/backend-challenge/internal/adapter/http"
	"github.com/lboevset/backend-challenge/internal/application"
)

func main() {
	// ── Config ────────────────────────────────────────────────────────────────
	mongoURI  := getEnv("MONGODB_URI",  "mongodb://localhost:27017")
	dbName    := getEnv("MONGODB_DB",   "assignment")
	jwtSecret := getEnv("JWT_SECRET",   "change-me-in-production")
	port      := getEnv("PORT",         "8080")
	grpcPort  := getEnv("GRPC_PORT",    "9090")

	// ── MongoDB ───────────────────────────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("MongoDB connect: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("MongoDB ping: %v", err)
	}
	log.Println("Connected to MongoDB")

	db   := client.Database(dbName)
	repo, err := mongodb.NewUserRepository(db)
	if err != nil {
		log.Fatalf("NewUserRepository: %v", err)
	}

	// ── Application layer ────────────────────────────────────────────────────
	svc := application.NewUserService(repo, jwtSecret)

	// ── Background goroutine: log user count every 10 s ───────────────────
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			count, err := svc.CountUsers(context.Background())
			if err != nil {
				log.Printf("[background] count error: %v", err)
				continue
			}
			log.Printf("[background] total users in DB: %d", count)
		}
	}()

	// ── gRPC server (enabled with -tags grpc) ────────────────────────────────
	startGRPC(svc, grpcPort)

	// ── HTTP server ───────────────────────────────────────────────────────────
	router := httpadapter.NewRouter(svc, jwtSecret)
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Start in background so we can wait for signals
	go func() {
		log.Printf("HTTP server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down…")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}
	if err := client.Disconnect(shutCtx); err != nil {
		log.Printf("MongoDB disconnect error: %v", err)
	}
	log.Println("Server stopped")
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
