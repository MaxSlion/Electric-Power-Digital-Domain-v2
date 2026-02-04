package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/electric-power/backend-service/internal/config"
	"github.com/electric-power/backend-service/internal/grpcclient"
	"github.com/electric-power/backend-service/internal/grpcserver"
	httpHandler "github.com/electric-power/backend-service/internal/http"
	"github.com/electric-power/backend-service/internal/scheduler"
	"github.com/electric-power/backend-service/internal/services"
	"github.com/electric-power/backend-service/internal/storage"
	"github.com/electric-power/backend-service/internal/ws"
	pb "github.com/electric-power/backend-service/proto"

	_ "github.com/electric-power/backend-service/docs" // Import swagger docs

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// @title           Electric Power Digital Domain Backend API
// @version         1.0
// @description     Backend service API for algorithm orchestration, job management, and real-time progress tracking.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@example.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	cfg := config.Load()

	// Initialize MySQL store
	store, err := storage.NewMySQLStore(cfg.MySQLDSN)
	if err != nil {
		logger.Fatal("MySQL connect failed", zap.Error(err))
	}
	defer store.Close()

	if err := store.InitSchema(context.Background()); err != nil {
		logger.Fatal("MySQL init schema failed", zap.Error(err))
	}
	logger.Info("MySQL connected and schema initialized")

	// Initialize Redis cache
	cache := storage.NewRedisCache(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err := cache.Ping(context.Background()); err != nil {
		logger.Warn("Redis ping failed, continuing without cache", zap.Error(err))
	} else {
		logger.Info("Redis connected")
	}
	defer cache.Close()

	// Initialize WebSocket hub
	hub := ws.NewHubWithLogger(logger)
	defer hub.Close()

	// Initialize job service
	jobs := services.NewJobService(store, cache, hub, cfg.SchemeCacheKey, cfg.ProgressCacheKeyNS)

	// Initialize algorithm gRPC client with resilience
	algoClientCfg := grpcclient.DefaultAlgoClientConfig(cfg.GRPCAlgoAddr)
	algoClient, err := grpcclient.NewAlgoClientWithConfig(algoClientCfg, logger)
	if err != nil {
		logger.Fatal("Algorithm gRPC client connect failed", zap.Error(err))
	}
	defer algoClient.Close()
	logger.Info("Algorithm gRPC client connected", zap.String("addr", cfg.GRPCAlgoAddr))

	// Pre-cache algorithm schemes
	if schemes, err := algoClient.GetSchemes(context.Background()); err == nil {
		_ = jobs.CacheSchemes(context.Background(), schemes)
		logger.Info("Cached algorithm schemes", zap.Int("count", len(schemes)))
	}

	// Initialize scheduler for background tasks
	sched := scheduler.NewScheduler(store, cache, algoClient, logger)
	sched.Start()
	defer sched.Stop()
	logger.Info("Background scheduler started")

	// Start gRPC callback server for receiving results from algorithm service
	grpcLis, err := net.Listen("tcp", cfg.GRPCResultAddr)
	if err != nil {
		logger.Fatal("gRPC listen failed", zap.Error(err))
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(100*1024*1024), // 100MB for large results
		grpc.MaxSendMsgSize(100*1024*1024),
	)
	pb.RegisterResultReceiverServiceServer(grpcServer, grpcserver.NewResultServer(jobs))

	go func() {
		logger.Info("gRPC result server starting", zap.String("addr", cfg.GRPCResultAddr))
		if err := grpcServer.Serve(grpcLis); err != nil {
			logger.Error("gRPC serve failed", zap.Error(err))
		}
	}()

	// Initialize HTTP handler and router
	h := httpHandler.NewHandler(jobs, algoClient, store, cache)
	routerCfg := httpHandler.RouterConfig{
		EnableSwagger:  true,
		RateLimitRPS:   cfg.RateLimitRPS,
		RequestTimeout: time.Duration(cfg.RequestTimeoutSec) * time.Second,
	}
	r := httpHandler.NewRouterWithConfig(h, hub, cache, logger, routerCfg)

	// Start HTTP server in goroutine
	go func() {
		logger.Info("HTTP server starting", zap.String("addr", cfg.HTTPAddr))
		if err := r.Run(cfg.HTTPAddr); err != nil {
			logger.Error("HTTP serve failed", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")

	// Stop accepting new connections
	grpcServer.GracefulStop()

	// Wait for in-flight requests
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop scheduler
	<-sched.Stop().Done()

	// Close hub
	hub.Close()

	logger.Info("Server shutdown complete")
	_ = ctx
}
