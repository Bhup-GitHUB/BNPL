package mock

import (
	"context"
	"time"
)

func sleepWithContext(ctx context.Context, latency time.Duration) error {
	if latency <= 0 {
		return nil
	}

	timer := time.NewTimer(latency)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func maskValue(value string) string {
	if len(value) <= 4 {
		return value
	}
	return value[:2] + "****" + value[len(value)-2:]
}
