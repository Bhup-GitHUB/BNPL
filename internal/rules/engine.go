package rules

import (
	"context"

	"bnpl/internal/domain"
)

type Engine struct {
	ApproveScore int
	RejectScore  int
}

func (e Engine) Evaluate(_ context.Context, application domain.Application, features domain.FeatureVector, score domain.ScoreResult, criticalTimeout bool) (domain.RuleResult, error) {
	result := domain.RuleResult{
		Status:   domain.StatusManualReview,
		Reasons:  []string{},
		RuleHits: []string{},
	}

	if criticalTimeout {
		result.Status = domain.StatusManualReview
		result.Reasons = append(result.Reasons, "critical_provider_timeout")
		result.RuleHits = append(result.RuleHits, "timeout_to_manual_review")
		return result, nil
	}

	if features.FraudHit {
		result.Status = domain.StatusRejected
		result.Reasons = append(result.Reasons, "fraud_signal_detected")
		result.RuleHits = append(result.RuleHits, "fraud_reject")
		return result, nil
	}

	if features.DedupeHit {
		result.Status = domain.StatusRejected
		result.Reasons = append(result.Reasons, "duplicate_or_shared_device")
		result.RuleHits = append(result.RuleHits, "dedupe_reject")
		return result, nil
	}

	if features.ThinFile {
		result.Status = domain.StatusManualReview
		result.Reasons = append(result.Reasons, "thin_credit_file")
		result.RuleHits = append(result.RuleHits, "thin_file_review")
	}

	if score.Score < e.RejectScore {
		result.Status = domain.StatusRejected
		result.Reasons = append(result.Reasons, "score_below_reject_threshold")
		result.RuleHits = append(result.RuleHits, "score_reject")
		return result, nil
	}

	if score.Score >= e.ApproveScore && !features.ThinFile {
		result.Status = domain.StatusApproved
		result.Reasons = append(result.Reasons, "score_above_approve_threshold")
		result.RuleHits = append(result.RuleHits, "score_approve")
		result.ApprovedLimit = approvedLimit(application, score.Score)
		return result, nil
	}

	result.Status = domain.StatusManualReview
	result.Reasons = append(result.Reasons, "needs_manual_review")
	result.RuleHits = append(result.RuleHits, "default_review")
	return result, nil
}

func approvedLimit(application domain.Application, score int) int64 {
	base := application.MonthlyIncome / 2
	if base < 5000 {
		base = 5000
	}
	if score > 85 {
		base = application.MonthlyIncome
	}
	if base > application.RequestedAmount {
		return application.RequestedAmount
	}
	return base
}
