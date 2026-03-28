package scoring

import (
	"context"
	"math"

	"bnpl/internal/domain"
)

type Engine struct{}

func (Engine) Score(_ context.Context, features domain.FeatureVector) (domain.ScoreResult, error) {
	breakdown := map[string]float64{
		"income_stability": features.IncomeStability * 28,
		"balance_trend":    trendScore(features.BalanceTrend),
		"bounce_count":     bouncePenalty(features.BounceCount),
		"bureau_band":      bureauBandScore(features.BureauScoreBand),
		"typing":           typingScore(features.TypingCadenceAnomaly),
		"form_speed":       formSpeedScore(features.FormCompletionSpeed),
		"device_reuse":     deviceReusePenalty(features.DeviceReuseCount),
		"amount_ratio":     amountRatioScore(features.MonthlyIncome, features.RequestedAmount),
		"thin_file":        thinFilePenalty(features.ThinFile),
		"dedupe":           dedupePenalty(features.DedupeHit),
		"fraud":            fraudPenalty(features.FraudHit),
	}

	total := 0.0
	for _, value := range breakdown {
		total += value
	}

	score := int(math.Round(total))
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return domain.ScoreResult{
		Score:     score,
		Breakdown: breakdown,
	}, nil
}

func trendScore(trend string) float64 {
	switch trend {
	case "up":
		return 18
	case "flat":
		return 11
	default:
		return 3
	}
}

func bouncePenalty(bounces int) float64 {
	return math.Max(0, 12-float64(bounces*4))
}

func bureauBandScore(band string) float64 {
	switch band {
	case "excellent":
		return 24
	case "good":
		return 18
	case "fair":
		return 11
	default:
		return 4
	}
}

func typingScore(anomaly bool) float64 {
	if anomaly {
		return 1
	}
	return 7
}

func formSpeedScore(speed string) float64 {
	switch speed {
	case "slow":
		return 5
	case "normal":
		return 7
	default:
		return 3
	}
}

func deviceReusePenalty(reuse int) float64 {
	return math.Max(0, 8-float64(reuse*2))
}

func amountRatioScore(monthlyIncome, requestedAmount int64) float64 {
	if monthlyIncome <= 0 {
		return 0
	}
	ratio := float64(requestedAmount) / float64(monthlyIncome)
	switch {
	case ratio <= 0.4:
		return 10
	case ratio <= 1.0:
		return 7
	case ratio <= 2.0:
		return 4
	default:
		return 1
	}
}

func thinFilePenalty(thinFile bool) float64 {
	if thinFile {
		return 2
	}
	return 6
}

func dedupePenalty(hit bool) float64 {
	if hit {
		return 0
	}
	return 4
}

func fraudPenalty(hit bool) float64 {
	if hit {
		return 0
	}
	return 6
}
