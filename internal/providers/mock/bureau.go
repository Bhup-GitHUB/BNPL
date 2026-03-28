package mock

import (
	"context"
	"hash/fnv"
	"strings"
	"time"

	"bnpl/internal/domain"
)

type BureauClient struct {
	Latency time.Duration
}

func (c BureauClient) Fetch(ctx context.Context, application domain.Application) (domain.BureauReport, error) {
	if err := sleepWithContext(ctx, c.Latency); err != nil {
		return domain.BureauReport{}, err
	}

	seed := hashValue(application.PAN + application.Mobile)
	band := "fair"
	thinFile := false
	switch seed % 4 {
	case 0:
		band = "excellent"
	case 1:
		band = "good"
	case 2:
		band = "fair"
	default:
		band = "poor"
	}

	if strings.HasSuffix(application.PAN, "Z") || seed%7 == 0 {
		thinFile = true
	}

	return domain.BureauReport{
		ScoreBand: band,
		ThinFile:  thinFile,
		Provider:  "mock-cibil",
		Raw: map[string]any{
			"provider":   "mock-cibil",
			"score_band": band,
			"thin_file":  thinFile,
		},
	}, nil
}

func hashValue(input string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(input))
	return h.Sum32()
}
