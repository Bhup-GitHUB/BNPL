package rules

import (
	"context"
	"testing"

	"bnpl/internal/domain"
)

func TestEvaluateApprovesHighScore(t *testing.T) {
	engine := Engine{ApproveScore: 72, RejectScore: 48}

	result, err := engine.Evaluate(context.Background(), domain.Application{
		MonthlyIncome:   90000,
		RequestedAmount: 15000,
	}, domain.FeatureVector{}, domain.ScoreResult{Score: 81}, false)
	if err != nil {
		t.Fatalf("evaluate returned error: %v", err)
	}
	if result.Status != domain.StatusApproved {
		t.Fatalf("expected approved, got %s", result.Status)
	}
	if result.ApprovedLimit == 0 {
		t.Fatalf("expected approved limit to be set")
	}
}

func TestEvaluateRejectsFraud(t *testing.T) {
	engine := Engine{ApproveScore: 72, RejectScore: 48}

	result, err := engine.Evaluate(context.Background(), domain.Application{}, domain.FeatureVector{
		FraudHit: true,
	}, domain.ScoreResult{Score: 99}, false)
	if err != nil {
		t.Fatalf("evaluate returned error: %v", err)
	}
	if result.Status != domain.StatusRejected {
		t.Fatalf("expected rejected, got %s", result.Status)
	}
}

func TestEvaluateManualReviewOnTimeout(t *testing.T) {
	engine := Engine{ApproveScore: 72, RejectScore: 48}

	result, err := engine.Evaluate(context.Background(), domain.Application{}, domain.FeatureVector{}, domain.ScoreResult{Score: 99}, true)
	if err != nil {
		t.Fatalf("evaluate returned error: %v", err)
	}
	if result.Status != domain.StatusManualReview {
		t.Fatalf("expected manual review, got %s", result.Status)
	}
}
