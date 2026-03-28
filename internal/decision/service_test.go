package decision

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"bnpl/internal/api"
	"bnpl/internal/domain"
	"bnpl/internal/features"
	"bnpl/internal/rules"
	"bnpl/internal/scoring"
)

type testRepo struct {
	mu          sync.Mutex
	byID        map[string]domain.ApplicationRecord
	byKey       map[string]string
	auditEvents []domain.AuditEvent
}

func newTestRepo() *testRepo {
	return &testRepo{
		byID:  map[string]domain.ApplicationRecord{},
		byKey: map[string]string{},
	}
}

func (r *testRepo) CreateApplication(_ context.Context, application domain.Application) (domain.Application, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byKey[application.IdempotencyKey]; ok {
		return domain.Application{}, errors.New("duplicate idempotency_key")
	}
	r.byKey[application.IdempotencyKey] = application.ID
	r.byID[application.ID] = domain.ApplicationRecord{Application: application}
	return application, nil
}

func (r *testRepo) GetByIdempotencyKey(_ context.Context, key string) (domain.ApplicationRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, ok := r.byKey[key]
	if !ok {
		return domain.ApplicationRecord{}, false, nil
	}
	record, ok := r.byID[id]
	return record, ok, nil
}

func (r *testRepo) SaveDecision(_ context.Context, decision domain.Decision, snapshots []domain.ProviderSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	record := r.byID[decision.ApplicationID]
	record.Decision = decision
	record.Application.Status = decision.Status
	record.Application.UpdatedAt = decision.UpdatedAt
	record.Snapshots = append([]domain.ProviderSnapshot(nil), snapshots...)
	r.byID[decision.ApplicationID] = record
	return nil
}

func (r *testRepo) GetApplication(_ context.Context, applicationID string) (domain.ApplicationRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.byID[applicationID]
	if !ok {
		return domain.ApplicationRecord{}, errors.New("not found")
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
	if fraudValue, ok := record.Decision.FeatureSummary["fraud_hit"].(bool); ok {
		trace.FraudHit = fraudValue
	}
	if dedupeValue, ok := record.Decision.FeatureSummary["dedupe_hit"].(bool); ok {
		trace.DedupeHit = dedupeValue
	}

	return trace, nil
}

func (r *testRepo) Ping(context.Context) error {
	return nil
}

func (r *testRepo) AppendAuditEvent(_ context.Context, event domain.AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auditEvents = append(r.auditEvents, event)
	return nil
}

type bureauStub struct {
	latency time.Duration
	report  domain.BureauReport
	err     error
}

func (s bureauStub) Fetch(ctx context.Context, _ domain.Application) (domain.BureauReport, error) {
	if err := waitStub(ctx, s.latency); err != nil {
		return domain.BureauReport{}, err
	}
	if s.err != nil {
		return domain.BureauReport{}, s.err
	}
	return s.report, nil
}

type accountStub struct {
	latency time.Duration
	report  domain.AccountAggregatorReport
	err     error
}

func (s accountStub) Fetch(ctx context.Context, _ domain.Application) (domain.AccountAggregatorReport, error) {
	if err := waitStub(ctx, s.latency); err != nil {
		return domain.AccountAggregatorReport{}, err
	}
	if s.err != nil {
		return domain.AccountAggregatorReport{}, s.err
	}
	return s.report, nil
}

type fraudStub struct {
	latency time.Duration
	report  domain.FraudReport
	err     error
}

func (s fraudStub) Fetch(ctx context.Context, _ domain.Application) (domain.FraudReport, error) {
	if err := waitStub(ctx, s.latency); err != nil {
		return domain.FraudReport{}, err
	}
	if s.err != nil {
		return domain.FraudReport{}, s.err
	}
	return s.report, nil
}

func waitStub(ctx context.Context, latency time.Duration) error {
	timer := time.NewTimer(latency)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func newTestService(repo *testRepo, bureauLatency, accountLatency, fraudLatency, featureLatency time.Duration) *Service {
	return NewService(Options{
		Repo:           repo,
		Audit:          repo,
		Bureau:         bureauStub{latency: bureauLatency, report: domain.BureauReport{ScoreBand: "excellent", Provider: "stub", Raw: map[string]any{"score_band": "excellent"}}},
		Account:        accountStub{latency: accountLatency, report: domain.AccountAggregatorReport{IncomeStability: 0.92, BalanceTrend: "up", BounceCount: 0, ConsentRef: "ok", Raw: map[string]any{"income_stability": 0.92}}},
		Fraud:          fraudStub{latency: fraudLatency, report: domain.FraudReport{FraudHit: false, DedupeHit: false, DeviceReuseCount: 0, Raw: map[string]any{"fraud_hit": false}}},
		Features:       features.Builder{Latency: featureLatency},
		Scoring:        scoring.Engine{},
		Rules:          rules.Engine{ApproveScore: 72, RejectScore: 48},
		Logger:         log.New(io.Discard, "", 0),
		RequestTimeout: 2 * time.Second,
		BureauTimeout:  800 * time.Millisecond,
		AccountTimeout: 800 * time.Millisecond,
		FraudTimeout:   800 * time.Millisecond,
	})
}

func baselineRequest() *api.SubmitApplicationRequest {
	return &api.SubmitApplicationRequest{
		IdempotencyKey:    "idem-1",
		FullName:          "Riya Sharma",
		PAN:               "ABCDE1234F",
		Mobile:            "9999999999",
		Email:             "riya@example.com",
		EmploymentType:    "salaried",
		MonthlyIncome:     120000,
		RequestedAmount:   15000,
		ConsentHandle:     "consent-12345",
		BankAccountRef:    "bank-1234",
		DeviceFingerprint: "device-strong-12345",
		TypingCadenceMs:   110,
		FormFillSeconds:   58,
		AddressPincode:    "560001",
		EmployerName:      "Tech Labs",
		Metadata:          map[string]string{"source": "test"},
	}
}

func TestSubmitApplicationApprovesStrongApplicant(t *testing.T) {
	repo := newTestRepo()
	service := newTestService(repo, 10*time.Millisecond, 10*time.Millisecond, 10*time.Millisecond, 10*time.Millisecond)

	response, err := service.SubmitApplication(context.Background(), baselineRequest())
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}
	if response.Status != string(domain.StatusApproved) {
		t.Fatalf("expected approved status, got %s", response.Status)
	}
	if response.ApprovedLimit == 0 {
		t.Fatalf("expected approved limit to be set")
	}
}

func TestSubmitApplicationTimeoutFallsBackToManualReview(t *testing.T) {
	repo := newTestRepo()
	service := newTestService(repo, time.Second, 10*time.Millisecond, 10*time.Millisecond, 10*time.Millisecond)

	response, err := service.SubmitApplication(context.Background(), baselineRequest())
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}
	if response.Status != string(domain.StatusManualReview) {
		t.Fatalf("expected manual review status, got %s", response.Status)
	}
}

func TestSubmitApplicationRunsProvidersConcurrently(t *testing.T) {
	repo := newTestRepo()
	service := newTestService(repo, 180*time.Millisecond, 180*time.Millisecond, 180*time.Millisecond, 180*time.Millisecond)

	request := baselineRequest()
	request.IdempotencyKey = "idem-concurrent"

	started := time.Now()
	response, err := service.SubmitApplication(context.Background(), request)
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}
	elapsed := time.Since(started)

	if response.Status == "" {
		t.Fatalf("expected status in response")
	}
	if elapsed > 320*time.Millisecond {
		t.Fatalf("expected concurrent execution, took %s", elapsed)
	}
}

func TestSubmitApplicationReturnsIdempotentResult(t *testing.T) {
	repo := newTestRepo()
	service := newTestService(repo, 10*time.Millisecond, 10*time.Millisecond, 10*time.Millisecond, 10*time.Millisecond)

	first, err := service.SubmitApplication(context.Background(), baselineRequest())
	if err != nil {
		t.Fatalf("first submit returned error: %v", err)
	}

	second, err := service.SubmitApplication(context.Background(), baselineRequest())
	if err != nil {
		t.Fatalf("second submit returned error: %v", err)
	}

	if first.ApplicationID != second.ApplicationID {
		t.Fatalf("expected idempotent application id, got %s and %s", first.ApplicationID, second.ApplicationID)
	}
}

func BenchmarkSubmitApplicationParallel(b *testing.B) {
	for i := 0; i < b.N; i++ {
		repo := newTestRepo()
		service := newTestService(repo, 10*time.Millisecond, 10*time.Millisecond, 10*time.Millisecond, 10*time.Millisecond)
		request := baselineRequest()
		request.IdempotencyKey = "bench-" + time.Now().String()
		if _, err := service.SubmitApplication(context.Background(), request); err != nil {
			b.Fatalf("submit returned error: %v", err)
		}
	}
}
