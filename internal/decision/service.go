package decision

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"bnpl/internal/api"
	"bnpl/internal/domain"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service struct {
	repo           domain.ApplicationRepository
	audit          domain.AuditRepository
	bureau         domain.BureauClient
	account        domain.AccountAggregatorClient
	fraud          domain.FraudClient
	features       domain.FeatureBuilder
	scoring        domain.ScoringEngine
	rules          domain.RulesEngine
	logger         *log.Logger
	requestTimeout time.Duration
	bureauTimeout  time.Duration
	accountTimeout time.Duration
	fraudTimeout   time.Duration
}

type Options struct {
	Repo           domain.ApplicationRepository
	Audit          domain.AuditRepository
	Bureau         domain.BureauClient
	Account        domain.AccountAggregatorClient
	Fraud          domain.FraudClient
	Features       domain.FeatureBuilder
	Scoring        domain.ScoringEngine
	Rules          domain.RulesEngine
	Logger         *log.Logger
	RequestTimeout time.Duration
	BureauTimeout  time.Duration
	AccountTimeout time.Duration
	FraudTimeout   time.Duration
}

func NewService(options Options) *Service {
	return &Service{
		repo:           options.Repo,
		audit:          options.Audit,
		bureau:         options.Bureau,
		account:        options.Account,
		fraud:          options.Fraud,
		features:       options.Features,
		scoring:        options.Scoring,
		rules:          options.Rules,
		logger:         options.Logger,
		requestTimeout: options.RequestTimeout,
		bureauTimeout:  options.BureauTimeout,
		accountTimeout: options.AccountTimeout,
		fraudTimeout:   options.FraudTimeout,
	}
}

func (s *Service) SubmitApplication(ctx context.Context, req *api.SubmitApplicationRequest) (*api.SubmitApplicationResponse, error) {
	if err := validateSubmitRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()

	if existing, found, err := s.repo.GetByIdempotencyKey(ctx, req.IdempotencyKey); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	} else if found {
		s.logger.Printf("submit_application idempotent_hit idempotency_key=%s application_id=%s status=%s", req.IdempotencyKey, existing.Application.ID, existing.Application.Status)
		return buildSubmitResponse(existing, time.Since(startedAt)), nil
	}

	now := time.Now().UTC()
	application := domain.Application{
		ID:                uuid.NewString(),
		TraceID:           uuid.NewString(),
		IdempotencyKey:    req.IdempotencyKey,
		FullName:          req.FullName,
		PAN:               req.PAN,
		Mobile:            req.Mobile,
		Email:             req.Email,
		EmploymentType:    req.EmploymentType,
		MonthlyIncome:     req.MonthlyIncome,
		RequestedAmount:   req.RequestedAmount,
		ConsentHandle:     req.ConsentHandle,
		BankAccountRef:    req.BankAccountRef,
		DeviceFingerprint: req.DeviceFingerprint,
		TypingCadenceMs:   req.TypingCadenceMs,
		FormFillSeconds:   req.FormFillSeconds,
		AddressPincode:    req.AddressPincode,
		EmployerName:      req.EmployerName,
		Metadata:          req.Metadata,
		Status:            domain.StatusPending,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	s.logger.Printf("submit_application start trace_id=%s application_id=%s applicant=%s requested_amount=%d", application.TraceID, application.ID, application.FullName, application.RequestedAmount)

	created, err := s.repo.CreateApplication(ctx, application)
	if err != nil {
		existing, found, fetchErr := s.repo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
		if fetchErr == nil && found {
			return buildSubmitResponse(existing, time.Since(startedAt)), nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	_ = s.audit.AppendAuditEvent(ctx, domain.AuditEvent{
		ApplicationID: created.ID,
		EventType:     "application_created",
		Payload: map[string]any{
			"trace_id": created.TraceID,
			"status":   created.Status,
		},
		CreatedAt: time.Now().UTC(),
	})

	var (
		wg              sync.WaitGroup
		mu              sync.Mutex
		bureauReport    domain.BureauReport
		accountReport   domain.AccountAggregatorReport
		fraudReport     domain.FraudReport
		behaviorReport  domain.BehaviorFeatures
		snapshots       []domain.ProviderSnapshot
		criticalTimeout bool
	)

	appendSnapshot := func(snapshot domain.ProviderSnapshot) {
		mu.Lock()
		defer mu.Unlock()
		snapshots = append(snapshots, snapshot)
	}

	runCritical := func(name string, timeout time.Duration, fetch func(context.Context) (map[string]any, error), fallback func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			providerStart := time.Now()
			s.logger.Printf("provider start trace_id=%s application_id=%s provider=%s", created.TraceID, created.ID, name)

			providerCtx, providerCancel := context.WithTimeout(ctx, timeout)
			defer providerCancel()

			payload, err := fetch(providerCtx)
			latency := time.Since(providerStart).Milliseconds()
			statusValue := "ok"
			if err != nil {
				statusValue = "error"
				if errors.Is(err, context.DeadlineExceeded) {
					statusValue = "timeout"
					mu.Lock()
					criticalTimeout = true
					mu.Unlock()
				}
				fallback()
				payload = map[string]any{
					"error": err.Error(),
				}
			}

			s.logger.Printf("provider finish trace_id=%s application_id=%s provider=%s status=%s latency_ms=%d", created.TraceID, created.ID, name, statusValue, latency)
			appendSnapshot(domain.ProviderSnapshot{
				ApplicationID: created.ID,
				ProviderName:  name,
				Status:        statusValue,
				LatencyMs:     latency,
				Payload:       payload,
				CreatedAt:     time.Now().UTC(),
			})
		}()
	}

	runCritical("bureau", s.bureauTimeout, func(providerCtx context.Context) (map[string]any, error) {
		report, err := s.bureau.Fetch(providerCtx, created)
		if err != nil {
			return nil, err
		}
		bureauReport = report
		return report.Raw, nil
	}, func() {
		bureauReport = domain.BureauReport{
			ScoreBand: "fair",
			ThinFile:  true,
			Provider:  "fallback",
			Raw:       map[string]any{"fallback": true},
		}
	})

	runCritical("account_aggregator", s.accountTimeout, func(providerCtx context.Context) (map[string]any, error) {
		report, err := s.account.Fetch(providerCtx, created)
		if err != nil {
			return nil, err
		}
		accountReport = report
		return report.Raw, nil
	}, func() {
		accountReport = domain.AccountAggregatorReport{
			IncomeStability: 0.5,
			BalanceTrend:    "flat",
			BounceCount:     2,
			ConsentRef:      "fallback",
			Raw:             map[string]any{"fallback": true},
		}
	})

	runCritical("fraud", s.fraudTimeout, func(providerCtx context.Context) (map[string]any, error) {
		report, err := s.fraud.Fetch(providerCtx, created)
		if err != nil {
			return nil, err
		}
		fraudReport = report
		return report.Raw, nil
	}, func() {
		fraudReport = domain.FraudReport{
			FraudHit:         false,
			DedupeHit:        false,
			DeviceReuseCount: 0,
			Raw:              map[string]any{"fallback": true},
		}
	})

	wg.Add(1)
	go func() {
		defer wg.Done()

		providerStart := time.Now()
		s.logger.Printf("provider start trace_id=%s application_id=%s provider=behavior_features", created.TraceID, created.ID)

		report, err := s.features.Extract(ctx, created)
		latency := time.Since(providerStart).Milliseconds()
		statusValue := "ok"
		payload := map[string]any{}
		if err != nil {
			statusValue = "error"
			payload["error"] = err.Error()
			behaviorReport = domain.BehaviorFeatures{
				TypingCadenceAnomaly: true,
				FormCompletionSpeed:  "fast",
				DeviceConfidence:     0.35,
				Raw:                  map[string]any{"fallback": true},
			}
		} else {
			behaviorReport = report
			payload = report.Raw
		}

		s.logger.Printf("provider finish trace_id=%s application_id=%s provider=behavior_features status=%s latency_ms=%d", created.TraceID, created.ID, statusValue, latency)
		appendSnapshot(domain.ProviderSnapshot{
			ApplicationID: created.ID,
			ProviderName:  "behavior_features",
			Status:        statusValue,
			LatencyMs:     latency,
			Payload:       payload,
			CreatedAt:     time.Now().UTC(),
		})
	}()

	wg.Wait()

	featureVector := s.features.Build(created, bureauReport, accountReport, fraudReport, behaviorReport)
	s.logger.Printf("scoring start trace_id=%s application_id=%s", created.TraceID, created.ID)
	scoreResult, err := s.scoring.Score(ctx, featureVector)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.logger.Printf("scoring finish trace_id=%s application_id=%s score=%d", created.TraceID, created.ID, scoreResult.Score)

	ruleResult, err := s.rules.Evaluate(ctx, created, featureVector, scoreResult, criticalTimeout)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.logger.Printf("rules applied trace_id=%s application_id=%s status=%s rules=%v", created.TraceID, created.ID, ruleResult.Status, ruleResult.RuleHits)

	decision := domain.Decision{
		ApplicationID:  created.ID,
		Status:         ruleResult.Status,
		Score:          scoreResult.Score,
		ApprovedLimit:  ruleResult.ApprovedLimit,
		Reasons:        uniqueStrings(ruleResult.Reasons),
		RuleHits:       uniqueStrings(ruleResult.RuleHits),
		ScoreBreakdown: scoreResult.Breakdown,
		FeatureSummary: featureVector.FeatureSnapshot,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.repo.SaveDecision(ctx, decision, snapshots); err != nil {
		s.logger.Printf("decision persist failed trace_id=%s application_id=%s err=%v", created.TraceID, created.ID, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	_ = s.audit.AppendAuditEvent(ctx, domain.AuditEvent{
		ApplicationID: created.ID,
		EventType:     "decision_persisted",
		Payload: map[string]any{
			"status":         decision.Status,
			"score":          decision.Score,
			"approved_limit": decision.ApprovedLimit,
		},
		CreatedAt: time.Now().UTC(),
	})

	record, err := s.repo.GetApplication(ctx, created.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.logger.Printf("submit_application finish trace_id=%s application_id=%s final_status=%s latency_ms=%d", created.TraceID, created.ID, decision.Status, time.Since(startedAt).Milliseconds())
	return buildSubmitResponse(record, time.Since(startedAt)), nil
}

func (s *Service) GetApplication(ctx context.Context, req *api.GetApplicationRequest) (*api.GetApplicationResponse, error) {
	if req.ApplicationID == "" {
		return nil, status.Error(codes.InvalidArgument, "application_id is required")
	}
	record, err := s.repo.GetApplication(ctx, req.ApplicationID)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return buildGetApplicationResponse(record), nil
}

func (s *Service) GetDecisionTrace(ctx context.Context, req *api.GetDecisionTraceRequest) (*api.GetDecisionTraceResponse, error) {
	if req.ApplicationID == "" {
		return nil, status.Error(codes.InvalidArgument, "application_id is required")
	}
	trace, err := s.repo.GetDecisionTrace(ctx, req.ApplicationID)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &api.GetDecisionTraceResponse{
		ApplicationID:   trace.ApplicationID,
		TraceID:         trace.TraceID,
		Status:          string(trace.Status),
		Score:           int32(trace.Score),
		ScoreBreakdown:  trace.ScoreBreakdown,
		RuleHits:        trace.RuleHits,
		Reasons:         trace.Reasons,
		FraudHit:        trace.FraudHit,
		DedupeHit:       trace.DedupeHit,
		ProviderState:   trace.ProviderState,
		ProviderLatency: trace.ProviderLatency,
		FeatureSnapshot: trace.FeatureSnapshot,
		ApprovedLimit:   trace.ApprovedLimit,
		CreatedAt:       trace.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       trace.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func validateSubmitRequest(req *api.SubmitApplicationRequest) error {
	switch {
	case req == nil:
		return fmt.Errorf("request is required")
	case req.IdempotencyKey == "":
		return fmt.Errorf("idempotency_key is required")
	case req.FullName == "":
		return fmt.Errorf("full_name is required")
	case req.PAN == "":
		return fmt.Errorf("pan is required")
	case req.Mobile == "":
		return fmt.Errorf("mobile is required")
	case req.EmploymentType == "":
		return fmt.Errorf("employment_type is required")
	case req.MonthlyIncome <= 0:
		return fmt.Errorf("monthly_income must be positive")
	case req.RequestedAmount <= 0:
		return fmt.Errorf("requested_amount must be positive")
	case req.ConsentHandle == "":
		return fmt.Errorf("consent_handle is required")
	case req.BankAccountRef == "":
		return fmt.Errorf("bank_account_ref is required")
	case req.DeviceFingerprint == "":
		return fmt.Errorf("device_fingerprint is required")
	}
	return nil
}

func buildSubmitResponse(record domain.ApplicationRecord, latency time.Duration) *api.SubmitApplicationResponse {
	providerState := map[string]string{}
	for _, snapshot := range record.Snapshots {
		providerState[snapshot.ProviderName] = snapshot.Status
	}

	return &api.SubmitApplicationResponse{
		ApplicationID: record.Application.ID,
		TraceID:       record.Application.TraceID,
		Status:        effectiveStatus(record),
		ApprovedLimit: record.Decision.ApprovedLimit,
		Reasons:       record.Decision.Reasons,
		Score:         int32(record.Decision.Score),
		LatencyMs:     latency.Milliseconds(),
		ProviderState: providerState,
		CreatedAt:     record.Application.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     record.Application.UpdatedAt.Format(time.RFC3339),
	}
}

func buildGetApplicationResponse(record domain.ApplicationRecord) *api.GetApplicationResponse {
	providerState := map[string]string{}
	providerLatency := map[string]int64{}
	for _, snapshot := range record.Snapshots {
		providerState[snapshot.ProviderName] = snapshot.Status
		providerLatency[snapshot.ProviderName] = snapshot.LatencyMs
	}

	return &api.GetApplicationResponse{
		ApplicationID:   record.Application.ID,
		TraceID:         record.Application.TraceID,
		Status:          effectiveStatus(record),
		ApprovedLimit:   record.Decision.ApprovedLimit,
		Reasons:         record.Decision.Reasons,
		Score:           int32(record.Decision.Score),
		ProviderState:   providerState,
		ProviderLatency: providerLatency,
		CreatedAt:       record.Application.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       record.Application.UpdatedAt.Format(time.RFC3339),
	}
}

func effectiveStatus(record domain.ApplicationRecord) string {
	if record.Decision.Status != "" {
		return string(record.Decision.Status)
	}
	return string(record.Application.Status)
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func isUniqueConflict(err error) bool {
	return false
}
