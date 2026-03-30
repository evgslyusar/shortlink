package transport

import (
	"context"

	"github.com/evgslyusar/shortlink/internal/service"
)

// clickStatsAdapter adapts service.ClickService to the LinkStatsSvc interface.
type clickStatsAdapter struct {
	svc *service.ClickService
}

// NewClickStatsAdapter wraps a ClickService to satisfy LinkStatsSvc.
func NewClickStatsAdapter(svc *service.ClickService) LinkStatsSvc {
	return &clickStatsAdapter{svc: svc}
}

func (a *clickStatsAdapter) GetStats(ctx context.Context, userID, slug string) (*LinkStats, error) {
	stats, err := a.svc.GetStats(ctx, userID, slug)
	if err != nil {
		return nil, err
	}
	return &LinkStats{
		TotalClicks: stats.TotalClicks,
		ByDay:       stats.ByDay,
		ByCountry:   stats.ByCountry,
	}, nil
}
