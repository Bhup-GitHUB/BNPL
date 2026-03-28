package scoring

import (
	"context"
	"testing"

	"bnpl/internal/domain"
)

func TestScoreStrongApplicant(t *testing.T) {
	engine := Engine{}

	result, err := engine.Score(context.Background(), domain.FeatureVector{
		IncomeStability:      0.92,
		BalanceTrend:         "up",
		BounceCount:          0,
		BureauScoreBand:      "excellent",
		TypingCadenceAnomaly: false,
		FormCompletionSpeed:  "normal",
		DeviceReuseCount:     0,
		DedupeHit:            false,
		FraudHit:             false,
		MonthlyIncome:        120000,
		RequestedAmount:      15000,
	})
	if err != nil {
		t.Fatalf("score returned error: %v", err)
	}
	if result.Score < 80 {
		t.Fatalf("expected strong score, got %d", result.Score)
	}
}

func TestScoreFraudAndLowIncomeApplicant(t *testing.T) {
	engine := Engine{}

	result, err := engine.Score(context.Background(), domain.FeatureVector{
		IncomeStability:      0.35,
		BalanceTrend:         "down",
		BounceCount:          4,
		BureauScoreBand:      "poor",
		TypingCadenceAnomaly: true,
		FormCompletionSpeed:  "fast",
		DeviceReuseCount:     5,
		DedupeHit:            true,
		FraudHit:             true,
		MonthlyIncome:        18000,
		RequestedAmount:      90000,
	})
	if err != nil {
		t.Fatalf("score returned error: %v", err)
	}
	if result.Score >= 50 {
		t.Fatalf("expected weak score, got %d", result.Score)
	}
}
