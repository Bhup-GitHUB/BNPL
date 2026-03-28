package api_test

import (
	"context"
	"io"
	"log"
	"net"
	"testing"
	"time"

	"bnpl/internal/api"
	"bnpl/internal/decision"
	"bnpl/internal/domain"
	"bnpl/internal/features"
	"bnpl/internal/rules"
	"bnpl/internal/scoring"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

type testRepo struct {
	applications map[string]domain.ApplicationRecord
	keys         map[string]string
}

func newRepo() *testRepo {
	return &testRepo{
		applications: map[string]domain.ApplicationRecord{},
		keys:         map[string]string{},
	}
}

func (r *testRepo) CreateApplication(_ context.Context, application domain.Application) (domain.Application, error) {
	r.keys[application.IdempotencyKey] = application.ID
	r.applications[application.ID] = domain.ApplicationRecord{Application: application}
	return application, nil
}

func (r *testRepo) GetByIdempotencyKey(_ context.Context, key string) (domain.ApplicationRecord, bool, error) {
	id, ok := r.keys[key]
	if !ok {
		return domain.ApplicationRecord{}, false, nil
	}
	record := r.applications[id]
	return record, true, nil
}

func (r *testRepo) SaveDecision(_ context.Context, decision domain.Decision, snapshots []domain.ProviderSnapshot) error {
	record := r.applications[decision.ApplicationID]
	record.Decision = decision
	record.Application.Status = decision.Status
	record.Application.UpdatedAt = decision.UpdatedAt
	record.Snapshots = snapshots
	r.applications[decision.ApplicationID] = record
	return nil
}

func (r *testRepo) GetApplication(_ context.Context, applicationID string) (domain.ApplicationRecord, error) {
	record, ok := r.applications[applicationID]
	if !ok {
		return domain.ApplicationRecord{}, net.ErrClosed
	}
	return record, nil
}

func (r *testRepo) GetDecisionTrace(_ context.Context, applicationID string) (domain.DecisionTrace, error) {
	record, err := r.GetApplication(context.Background(), applicationID)
	if err != nil {
		return domain.DecisionTrace{}, err
	}
	trace := domain.DecisionTrace{
		ApplicationID:   record.Application.ID,
		TraceID:         record.Application.TraceID,
		Status:          record.Decision.Status,
		Score:           record.Decision.Score,
		ApprovedLimit:   record.Decision.ApprovedLimit,
		Reasons:         record.Decision.Reasons,
		RuleHits:        record.Decision.RuleHits,
		ScoreBreakdown:  record.Decision.ScoreBreakdown,
		FeatureSnapshot: record.Decision.FeatureSummary,
		ProviderState:   map[string]string{},
		ProviderLatency: map[string]int64{},
		CreatedAt:       record.Application.CreatedAt,
		UpdatedAt:       record.Application.UpdatedAt,
	}
	for _, snapshot := range record.Snapshots {
		trace.ProviderState[snapshot.ProviderName] = snapshot.Status
		trace.ProviderLatency[snapshot.ProviderName] = snapshot.LatencyMs
	}
	return trace, nil
}

func (r *testRepo) Ping(context.Context) error {
	return nil
}

func (r *testRepo) AppendAuditEvent(context.Context, domain.AuditEvent) error {
	return nil
}

type bureauStub struct{}

func (bureauStub) Fetch(context.Context, domain.Application) (domain.BureauReport, error) {
	return domain.BureauReport{ScoreBand: "excellent", Provider: "stub", Raw: map[string]any{"score_band": "excellent"}}, nil
}

type accountStub struct{}

func (accountStub) Fetch(context.Context, domain.Application) (domain.AccountAggregatorReport, error) {
	return domain.AccountAggregatorReport{IncomeStability: 0.93, BalanceTrend: "up", BounceCount: 0, Raw: map[string]any{"income_stability": 0.93}}, nil
}

type fraudStub struct{}

func (fraudStub) Fetch(context.Context, domain.Application) (domain.FraudReport, error) {
	return domain.FraudReport{Raw: map[string]any{"fraud_hit": false}}, nil
}

func TestDecisionServiceOverGRPC(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	repo := newRepo()
	server := grpc.NewServer()

	service := decision.NewService(decision.Options{
		Repo:           repo,
		Audit:          repo,
		Bureau:         bureauStub{},
		Account:        accountStub{},
		Fraud:          fraudStub{},
		Features:       features.Builder{Latency: 0},
		Scoring:        scoring.Engine{},
		Rules:          rules.Engine{ApproveScore: 72, RejectScore: 48},
		Logger:         log.New(io.Discard, "", 0),
		RequestTimeout: time.Second,
		BureauTimeout:  time.Second,
		AccountTimeout: time.Second,
		FraudTimeout:   time.Second,
	})

	api.RegisterDecisionServiceServer(server, service)

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
		IdempotencyKey:    "grpc-1",
		FullName:          "Aarav Mehta",
		PAN:               "ABCDE1234F",
		Mobile:            "9000000000",
		Email:             "aarav@example.com",
		EmploymentType:    "salaried",
		MonthlyIncome:     100000,
		RequestedAmount:   12000,
		ConsentHandle:     "consent-1",
		BankAccountRef:    "bank-1",
		DeviceFingerprint: "device-1",
		TypingCadenceMs:   120,
		FormFillSeconds:   65,
		AddressPincode:    "560001",
		EmployerName:      "Tech Labs",
	})
	if err != nil {
		t.Fatalf("submit over grpc failed: %v", err)
	}
	if response.Status != string(domain.StatusApproved) {
		t.Fatalf("expected approved status, got %s", response.Status)
	}

	trace, err := client.GetDecisionTrace(context.Background(), &api.GetDecisionTraceRequest{
		ApplicationID: response.ApplicationID,
	})
	if err != nil {
		t.Fatalf("trace over grpc failed: %v", err)
	}
	if trace.ApplicationID != response.ApplicationID {
		t.Fatalf("expected trace application id %s, got %s", response.ApplicationID, trace.ApplicationID)
	}
}
