package mock

import (
	"context"
	"strings"
	"time"

	"bnpl/internal/domain"
)

type FraudClient struct {
	Latency time.Duration
}

func (c FraudClient) Fetch(ctx context.Context, application domain.Application) (domain.FraudReport, error) {
	if err := sleepWithContext(ctx, c.Latency); err != nil {
		return domain.FraudReport{}, err
	}

	deviceReuseCount := 0
	fraudHit := false
	dedupeHit := false

	if strings.HasPrefix(strings.ToLower(application.DeviceFingerprint), "shared") {
		deviceReuseCount = 5
		dedupeHit = true
	}

	if application.FormFillSeconds > 0 && application.FormFillSeconds < 12 && application.TypingCadenceMs < 35 {
		fraudHit = true
	}

	if strings.Contains(strings.ToLower(application.Email), "fraud") {
		fraudHit = true
	}

	return domain.FraudReport{
		FraudHit:         fraudHit,
		DedupeHit:        dedupeHit,
		DeviceReuseCount: deviceReuseCount,
		Raw: map[string]any{
			"fraud_hit":          fraudHit,
			"dedupe_hit":         dedupeHit,
			"device_reuse_count": deviceReuseCount,
		},
	}, nil
}
