package postgres

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bnpl/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStoreRoundTripWithPostgres(t *testing.T) {
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

	migrationsDir, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("migrations path failed: %v", err)
	}
	if err := RunMigrations(ctx, pool, migrationsDir); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	store := NewStore(pool)
	now := time.Now().UTC()
	application := domain.Application{
		ID:                "app-int-1",
		TraceID:           "trace-int-1",
		IdempotencyKey:    "idem-int-1",
		FullName:          "Neha Verma",
		PAN:               "ABCDE1234F",
		Mobile:            "9888888888",
		Email:             "neha@example.com",
		EmploymentType:    "salaried",
		MonthlyIncome:     85000,
		RequestedAmount:   15000,
		ConsentHandle:     "consent-abc",
		BankAccountRef:    "bank-abc",
		DeviceFingerprint: "device-abc",
		TypingCadenceMs:   120,
		FormFillSeconds:   55,
		AddressPincode:    "560001",
		EmployerName:      "Tech Labs",
		Metadata:          map[string]string{"source": "integration"},
		Status:            domain.StatusPending,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if _, err := store.CreateApplication(ctx, application); err != nil {
		t.Fatalf("create application failed: %v", err)
	}

	decision := domain.Decision{
		ApplicationID:  application.ID,
		Status:         domain.StatusApproved,
		Score:          84,
		ApprovedLimit:  15000,
		Reasons:        []string{"score_above_approve_threshold"},
		RuleHits:       []string{"score_approve"},
		ScoreBreakdown: map[string]float64{"bureau_band": 24},
		FeatureSummary: map[string]any{"fraud_hit": false, "dedupe_hit": false},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	snapshots := []domain.ProviderSnapshot{
		{
			ApplicationID: application.ID,
			ProviderName:  "bureau",
			Status:        "ok",
			LatencyMs:     200,
			Payload:       map[string]any{"provider": "mock-cibil"},
			CreatedAt:     now,
		},
	}

	if err := store.SaveDecision(ctx, decision, snapshots); err != nil {
		t.Fatalf("save decision failed: %v", err)
	}

	record, err := store.GetApplication(ctx, application.ID)
	if err != nil {
		t.Fatalf("get application failed: %v", err)
	}
	if record.Decision.Status != domain.StatusApproved {
		t.Fatalf("expected approved decision, got %s", record.Decision.Status)
	}
	if len(record.Snapshots) != 1 {
		t.Fatalf("expected one provider snapshot, got %d", len(record.Snapshots))
	}

	trace, err := store.GetDecisionTrace(ctx, application.ID)
	if err != nil {
		t.Fatalf("get decision trace failed: %v", err)
	}
	if trace.ProviderLatency["bureau"] != 200 {
		t.Fatalf("expected bureau latency 200, got %d", trace.ProviderLatency["bureau"])
	}
}
