package api_test

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bnpl/internal/api"
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
	"google.golang.org/grpc/test/bufconn"
)

func TestDecisionServiceWithPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pool init failed: %v", err)
	}
	defer pool.Close()

	migrationsDir, err := filepath.Abs("../../migrations")
	if err != nil {
		t.Fatalf("migrations path failed: %v", err)
	}
	if err := postgres.RunMigrations(ctx, pool, migrationsDir); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	store := postgres.NewStore(pool)

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()

	service := decision.NewService(decision.Options{
		Repo:           store,
		Audit:          store,
		Bureau:         mock.BureauClient{Latency: 20 * time.Millisecond},
		Account:        mock.AccountAggregatorClient{Latency: 20 * time.Millisecond},
		Fraud:          mock.FraudClient{Latency: 20 * time.Millisecond},
		Features:       features.Builder{Latency: 20 * time.Millisecond},
		Scoring:        scoring.Engine{},
		Rules:          rules.Engine{ApproveScore: 72, RejectScore: 48},
		Logger:         log.New(io.Discard, "", 0),
		RequestTimeout: time.Second,
		BureauTimeout:  time.Second,
		AccountTimeout: time.Second,
		FraudTimeout:   time.Second,
	})

	api.RegisterDecisionServiceServer(server, service)
	healthServer := health.NewServer()
	healthServer.SetServingStatus(api.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, healthServer)

	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Stop()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(api.CodecName)),
	)
	if err != nil {
		t.Fatalf("grpc client setup failed: %v", err)
	}
	defer conn.Close()

	client := api.NewDecisionClient(conn)
	response, err := client.SubmitApplication(context.Background(), &api.SubmitApplicationRequest{
		IdempotencyKey:    "postgres-e2e-1",
		FullName:          "Kabir Singh",
		PAN:               "ABCDE1234F",
		Mobile:            "9012345678",
		Email:             "kabir@example.com",
		EmploymentType:    "salaried",
		MonthlyIncome:     90000,
		RequestedAmount:   18000,
		ConsentHandle:     "consent-live-1",
		BankAccountRef:    "bank-live-1",
		DeviceFingerprint: "device-live-1",
		TypingCadenceMs:   110,
		FormFillSeconds:   54,
		AddressPincode:    "560001",
		EmployerName:      "Tech Systems",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	if response.ApplicationID == "" {
		t.Fatalf("expected application id")
	}

	trace, err := client.GetDecisionTrace(context.Background(), &api.GetDecisionTraceRequest{
		ApplicationID: response.ApplicationID,
	})
	if err != nil {
		t.Fatalf("get trace failed: %v", err)
	}
	if trace.ApplicationID != response.ApplicationID {
		t.Fatalf("expected trace id for application %s, got %s", response.ApplicationID, trace.ApplicationID)
	}
	if len(trace.ProviderState) == 0 {
		t.Fatalf("expected provider state to be persisted")
	}
}
