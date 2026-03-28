package features

import (
	"context"
	"strings"
	"time"

	"bnpl/internal/domain"
)

type Builder struct {
	Latency time.Duration
}

func (b Builder) Extract(ctx context.Context, application domain.Application) (domain.BehaviorFeatures, error) {
	if b.Latency > 0 {
		timer := time.NewTimer(b.Latency)
		select {
		case <-ctx.Done():
			timer.Stop()
			return domain.BehaviorFeatures{}, ctx.Err()
		case <-timer.C:
		}
	}

	speed := "normal"
	switch {
	case application.FormFillSeconds > 0 && application.FormFillSeconds < 25:
		speed = "fast"
	case application.FormFillSeconds > 120:
		speed = "slow"
	}

	anomaly := application.TypingCadenceMs > 0 && application.TypingCadenceMs < 40
	if application.TypingCadenceMs > 450 {
		anomaly = true
	}

	confidence := 0.81
	if len(strings.TrimSpace(application.DeviceFingerprint)) < 8 {
		confidence = 0.42
	}

	return domain.BehaviorFeatures{
		TypingCadenceAnomaly: anomaly,
		FormCompletionSpeed:  speed,
		DeviceConfidence:     confidence,
		Raw: map[string]any{
			"typing_cadence_ms": application.TypingCadenceMs,
			"form_fill_seconds": application.FormFillSeconds,
			"device_confidence": confidence,
		},
	}, nil
}

func (b Builder) Build(application domain.Application, bureau domain.BureauReport, aa domain.AccountAggregatorReport, fraud domain.FraudReport, behavior domain.BehaviorFeatures) domain.FeatureVector {
	return domain.FeatureVector{
		IncomeStability:      aa.IncomeStability,
		BalanceTrend:         aa.BalanceTrend,
		BounceCount:          aa.BounceCount,
		BureauScoreBand:      bureau.ScoreBand,
		ThinFile:             bureau.ThinFile,
		TypingCadenceAnomaly: behavior.TypingCadenceAnomaly,
		FormCompletionSpeed:  behavior.FormCompletionSpeed,
		DeviceReuseCount:     fraud.DeviceReuseCount,
		DedupeHit:            fraud.DedupeHit,
		FraudHit:             fraud.FraudHit,
		MonthlyIncome:        application.MonthlyIncome,
		RequestedAmount:      application.RequestedAmount,
		FeatureSnapshot: map[string]any{
			"income_stability":        aa.IncomeStability,
			"balance_trend":           aa.BalanceTrend,
			"bounce_count":            aa.BounceCount,
			"bureau_score_band":       bureau.ScoreBand,
			"thin_file":               bureau.ThinFile,
			"typing_cadence_anomaly":  behavior.TypingCadenceAnomaly,
			"form_completion_speed":   behavior.FormCompletionSpeed,
			"device_reuse_count":      fraud.DeviceReuseCount,
			"dedupe_hit":              fraud.DedupeHit,
			"fraud_hit":               fraud.FraudHit,
			"monthly_income":          application.MonthlyIncome,
			"requested_amount":        application.RequestedAmount,
		},
	}
}
