package mock

import (
	"context"
	"strings"
	"time"

	"bnpl/internal/domain"
)

type AccountAggregatorClient struct {
	Latency time.Duration
}

func (c AccountAggregatorClient) Fetch(ctx context.Context, application domain.Application) (domain.AccountAggregatorReport, error) {
	if err := sleepWithContext(ctx, c.Latency); err != nil {
		return domain.AccountAggregatorReport{}, err
	}

	incomeStability := 0.62
	if application.MonthlyIncome >= 120000 {
		incomeStability = 0.91
	} else if application.MonthlyIncome >= 70000 {
		incomeStability = 0.79
	}

	trend := "flat"
	if strings.Contains(strings.ToLower(application.EmployerName), "tech") || application.MonthlyIncome > application.RequestedAmount {
		trend = "up"
	}
	if application.RequestedAmount > application.MonthlyIncome*2 {
		trend = "down"
	}

	bounceCount := 0
	if application.MonthlyIncome < 30000 {
		bounceCount = 2
	}
	if application.RequestedAmount > application.MonthlyIncome*3 {
		bounceCount = 4
	}

	return domain.AccountAggregatorReport{
		IncomeStability: incomeStability,
		BalanceTrend:    trend,
		BounceCount:     bounceCount,
		ConsentRef:      maskValue(application.ConsentHandle),
		Raw: map[string]any{
			"income_stability": incomeStability,
			"balance_trend":    trend,
			"bounce_count":     bounceCount,
			"consent_ref":      maskValue(application.ConsentHandle),
		},
	}, nil
}
