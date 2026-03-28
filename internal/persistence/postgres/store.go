package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bnpl/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) CreateApplication(ctx context.Context, application domain.Application) (domain.Application, error) {
	metadata, err := json.Marshal(application.Metadata)
	if err != nil {
		return domain.Application{}, err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO applications (
			id, trace_id, idempotency_key, full_name, pan, mobile, email, employment_type,
			monthly_income, requested_amount, consent_handle, bank_account_ref, device_fingerprint,
			typing_cadence_ms, form_fill_seconds, address_pincode, employer_name, metadata, status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19, $20, $21
		)
	`, application.ID, application.TraceID, application.IdempotencyKey, application.FullName, application.PAN, application.Mobile, application.Email,
		application.EmploymentType, application.MonthlyIncome, application.RequestedAmount, application.ConsentHandle, application.BankAccountRef,
		application.DeviceFingerprint, application.TypingCadenceMs, application.FormFillSeconds, application.AddressPincode,
		application.EmployerName, metadata, string(application.Status), application.CreatedAt, application.UpdatedAt)
	if err != nil {
		return domain.Application{}, err
	}

	return application, nil
}

func (s *Store) GetByIdempotencyKey(ctx context.Context, key string) (domain.ApplicationRecord, bool, error) {
	record, err := s.getApplicationRecord(ctx, "SELECT a.id FROM applications a WHERE a.idempotency_key = $1", key)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ApplicationRecord{}, false, nil
		}
		return domain.ApplicationRecord{}, false, err
	}
	return record, true, nil
}

func (s *Store) SaveDecision(ctx context.Context, decision domain.Decision, snapshots []domain.ProviderSnapshot) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	reasons, err := json.Marshal(decision.Reasons)
	if err != nil {
		return err
	}
	ruleHits, err := json.Marshal(decision.RuleHits)
	if err != nil {
		return err
	}
	scoreBreakdown, err := json.Marshal(decision.ScoreBreakdown)
	if err != nil {
		return err
	}
	featureSummary, err := json.Marshal(decision.FeatureSummary)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO decisions (
			application_id, status, score, approved_limit, reasons, rule_hits, score_breakdown, feature_summary, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
		ON CONFLICT (application_id) DO UPDATE SET
			status = EXCLUDED.status,
			score = EXCLUDED.score,
			approved_limit = EXCLUDED.approved_limit,
			reasons = EXCLUDED.reasons,
			rule_hits = EXCLUDED.rule_hits,
			score_breakdown = EXCLUDED.score_breakdown,
			feature_summary = EXCLUDED.feature_summary,
			updated_at = EXCLUDED.updated_at
	`, decision.ApplicationID, string(decision.Status), decision.Score, decision.ApprovedLimit, reasons, ruleHits, scoreBreakdown, featureSummary, decision.CreatedAt, decision.UpdatedAt)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `DELETE FROM provider_snapshots WHERE application_id = $1`, decision.ApplicationID)
	if err != nil {
		return err
	}

	for _, snapshot := range snapshots {
		payload, marshalErr := json.Marshal(snapshot.Payload)
		if marshalErr != nil {
			return marshalErr
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO provider_snapshots (
				application_id, provider_name, status, latency_ms, payload, created_at
			) VALUES ($1, $2, $3, $4, $5, $6)
		`, snapshot.ApplicationID, snapshot.ProviderName, snapshot.Status, snapshot.LatencyMs, payload, snapshot.CreatedAt)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE applications
		SET status = $2, updated_at = $3
		WHERE id = $1
	`, decision.ApplicationID, string(decision.Status), decision.UpdatedAt)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Store) AppendAuditEvent(ctx context.Context, event domain.AuditEvent) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_events (application_id, event_type, payload, created_at)
		VALUES ($1, $2, $3, $4)
	`, event.ApplicationID, event.EventType, payload, event.CreatedAt)
	return err
}

func (s *Store) GetApplication(ctx context.Context, applicationID string) (domain.ApplicationRecord, error) {
	return s.getApplicationRecord(ctx, "SELECT a.id FROM applications a WHERE a.id = $1", applicationID)
}

func (s *Store) GetDecisionTrace(ctx context.Context, applicationID string) (domain.DecisionTrace, error) {
	record, err := s.GetApplication(ctx, applicationID)
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

	if fraudValue, ok := record.Decision.FeatureSummary["fraud_hit"].(bool); ok {
		trace.FraudHit = fraudValue
	}
	if dedupeValue, ok := record.Decision.FeatureSummary["dedupe_hit"].(bool); ok {
		trace.DedupeHit = dedupeValue
	}

	for _, snapshot := range record.Snapshots {
		trace.ProviderState[snapshot.ProviderName] = snapshot.Status
		trace.ProviderLatency[snapshot.ProviderName] = snapshot.LatencyMs
	}

	return trace, nil
}

func (s *Store) getApplicationRecord(ctx context.Context, finderQuery string, arg string) (domain.ApplicationRecord, error) {
	var applicationID string
	err := s.pool.QueryRow(ctx, finderQuery, arg).Scan(&applicationID)
	if err != nil {
		return domain.ApplicationRecord{}, err
	}

	row := s.pool.QueryRow(ctx, `
		SELECT
			a.id, a.trace_id, a.idempotency_key, a.full_name, a.pan, a.mobile, a.email, a.employment_type,
			a.monthly_income, a.requested_amount, a.consent_handle, a.bank_account_ref, a.device_fingerprint,
			a.typing_cadence_ms, a.form_fill_seconds, a.address_pincode, a.employer_name, a.metadata, a.status, a.created_at, a.updated_at,
			d.status, d.score, d.approved_limit, d.reasons, d.rule_hits, d.score_breakdown, d.feature_summary, d.created_at, d.updated_at
		FROM applications a
		LEFT JOIN decisions d ON d.application_id = a.id
		WHERE a.id = $1
	`, applicationID)

	var (
		metadataBytes       []byte
		appStatus           string
		decisionStatus      *string
		score               *int32
		approvedLimit       *int64
		reasonsBytes        []byte
		ruleHitsBytes       []byte
		scoreBreakdownBytes []byte
		featureSummaryBytes []byte
		decisionCreatedAt   *time.Time
		decisionUpdatedAt   *time.Time
		record              domain.ApplicationRecord
	)

	err = row.Scan(
		&record.Application.ID,
		&record.Application.TraceID,
		&record.Application.IdempotencyKey,
		&record.Application.FullName,
		&record.Application.PAN,
		&record.Application.Mobile,
		&record.Application.Email,
		&record.Application.EmploymentType,
		&record.Application.MonthlyIncome,
		&record.Application.RequestedAmount,
		&record.Application.ConsentHandle,
		&record.Application.BankAccountRef,
		&record.Application.DeviceFingerprint,
		&record.Application.TypingCadenceMs,
		&record.Application.FormFillSeconds,
		&record.Application.AddressPincode,
		&record.Application.EmployerName,
		&metadataBytes,
		&appStatus,
		&record.Application.CreatedAt,
		&record.Application.UpdatedAt,
		&decisionStatus,
		&score,
		&approvedLimit,
		&reasonsBytes,
		&ruleHitsBytes,
		&scoreBreakdownBytes,
		&featureSummaryBytes,
		&decisionCreatedAt,
		&decisionUpdatedAt,
	)
	if err != nil {
		return domain.ApplicationRecord{}, err
	}

	record.Application.Status = domain.DecisionStatus(appStatus)
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &record.Application.Metadata); err != nil {
			return domain.ApplicationRecord{}, err
		}
	}

	if decisionStatus != nil {
		record.Decision.ApplicationID = record.Application.ID
		record.Decision.Status = domain.DecisionStatus(*decisionStatus)
		if score != nil {
			record.Decision.Score = int(*score)
		}
		if approvedLimit != nil {
			record.Decision.ApprovedLimit = *approvedLimit
		}
		if len(reasonsBytes) > 0 {
			if err := json.Unmarshal(reasonsBytes, &record.Decision.Reasons); err != nil {
				return domain.ApplicationRecord{}, err
			}
		}
		if len(ruleHitsBytes) > 0 {
			if err := json.Unmarshal(ruleHitsBytes, &record.Decision.RuleHits); err != nil {
				return domain.ApplicationRecord{}, err
			}
		}
		if len(scoreBreakdownBytes) > 0 {
			if err := json.Unmarshal(scoreBreakdownBytes, &record.Decision.ScoreBreakdown); err != nil {
				return domain.ApplicationRecord{}, err
			}
		}
		if len(featureSummaryBytes) > 0 {
			if err := json.Unmarshal(featureSummaryBytes, &record.Decision.FeatureSummary); err != nil {
				return domain.ApplicationRecord{}, err
			}
		}
		if decisionCreatedAt != nil {
			record.Decision.CreatedAt = *decisionCreatedAt
		}
		if decisionUpdatedAt != nil {
			record.Decision.UpdatedAt = *decisionUpdatedAt
		}
	}

	snapshots, err := s.loadSnapshots(ctx, record.Application.ID)
	if err != nil {
		return domain.ApplicationRecord{}, err
	}
	record.Snapshots = snapshots

	return record, nil
}

func (s *Store) loadSnapshots(ctx context.Context, applicationID string) ([]domain.ProviderSnapshot, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT provider_name, status, latency_ms, payload, created_at
		FROM provider_snapshots
		WHERE application_id = $1
		ORDER BY provider_name ASC
	`, applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	snapshots := make([]domain.ProviderSnapshot, 0)
	for rows.Next() {
		var (
			snapshot domain.ProviderSnapshot
			payload  []byte
		)
		snapshot.ApplicationID = applicationID
		if err := rows.Scan(&snapshot.ProviderName, &snapshot.Status, &snapshot.LatencyMs, &payload, &snapshot.CreatedAt); err != nil {
			return nil, err
		}
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &snapshot.Payload); err != nil {
				return nil, err
			}
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, rows.Err()
}

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)

	for _, file := range files {
		path := filepath.Join(dir, file)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", file, err)
		}
	}

	return nil
}

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
