package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"bnpl/internal/api"
	"bnpl/internal/config"
	"bnpl/internal/decision"
	"bnpl/internal/features"
	"bnpl/internal/persistence/postgres"
	"bnpl/internal/providers/mock"
	"bnpl/internal/rules"
	"bnpl/internal/scoring"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("config load failed err=%v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("postgres pool failed err=%v", err)
	}
	defer pool.Close()

	migrationsDir, err := filepath.Abs("migrations")
	if err != nil {
		logger.Fatalf("migrations path failed err=%v", err)
	}

	if err := postgres.RunMigrations(ctx, pool, migrationsDir); err != nil {
		logger.Fatalf("migration failed err=%v", err)
	}

	store := postgres.NewStore(pool)
	if err := store.Ping(ctx); err != nil {
		logger.Fatalf("postgres ping failed err=%v", err)
	}

	service := decision.NewService(decision.Options{
		Repo:           store,
		Audit:          store,
		Bureau:         mock.BureauClient{Latency: cfg.BureauLatency},
		Account:        mock.AccountAggregatorClient{Latency: cfg.AccountLatency},
		Fraud:          mock.FraudClient{Latency: cfg.FraudLatency},
		Features:       features.Builder{Latency: cfg.FeatureLatency},
		Scoring:        scoring.Engine{},
		Rules:          rules.Engine{ApproveScore: cfg.ApproveScore, RejectScore: cfg.RejectScore},
		Logger:         logger,
		RequestTimeout: cfg.RequestTimeout,
		BureauTimeout:  cfg.BureauTimeout,
		AccountTimeout: cfg.AccountTimeout,
		FraudTimeout:   cfg.FraudTimeout,
	})

	server := grpc.NewServer()
	api.RegisterDecisionServiceServer(server, service)

	healthServer := health.NewServer()
	healthServer.SetServingStatus(api.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, healthServer)

	reflection.Register(server)

	listener, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		logger.Fatalf("listen failed err=%v", err)
	}

	go func() {
		<-ctx.Done()
		logger.Printf("shutdown start")
		healthServer.SetServingStatus(api.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		stopped := make(chan struct{})
		go func() {
			server.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			server.Stop()
		}
	}()

	logger.Printf("grpc server listening port=%s", cfg.Port)
	if err := server.Serve(listener); err != nil {
		logger.Fatalf("server stopped err=%v", err)
	}
}
