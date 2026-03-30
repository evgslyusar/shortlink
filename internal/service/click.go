package service

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/domain"
)

const (
	clickChannelSize = 1000
	flushInterval    = 5 * time.Second
	flushBatchSize   = 100
	statsDays        = 30
)

// ClickBatchInserter persists a batch of clicks.
type ClickBatchInserter interface {
	BatchInsert(ctx context.Context, clicks []domain.Click) error
}

// ClickStatsQuerier provides aggregated click statistics.
type ClickStatsQuerier interface {
	CountByLink(ctx context.Context, linkID string) (int64, error)
	CountByDay(ctx context.Context, linkID string, days int) ([]domain.DayStat, error)
	CountByCountry(ctx context.Context, linkID string) ([]domain.CountryStat, error)
}

// ClickStats holds aggregated statistics for a link.
type ClickStats struct {
	TotalClicks int64              `json:"total_clicks"`
	ByDay       []domain.DayStat   `json:"by_day"`
	ByCountry   []domain.CountryStat `json:"by_country"`
}

// ClickService handles async click recording and stats retrieval.
type ClickService struct {
	inserter   ClickBatchInserter
	querier    ClickStatsQuerier
	linkFinder LinkBySlugFinder
	logger     *zap.Logger
	ch         chan domain.Click
}

// NewClickService creates a new ClickService.
func NewClickService(
	inserter ClickBatchInserter,
	querier ClickStatsQuerier,
	linkFinder LinkBySlugFinder,
	logger *zap.Logger,
) *ClickService {
	return &ClickService{
		inserter:   inserter,
		querier:    querier,
		linkFinder: linkFinder,
		logger:     logger,
		ch:         make(chan domain.Click, clickChannelSize),
	}
}

// Record sends a click to the background worker for async persistence.
// Non-blocking: drops the click and logs a warning if the channel is full.
func (s *ClickService) Record(click domain.Click) {
	select {
	case s.ch <- click:
	default:
		s.logger.Warn("click channel full, dropping event",
			zap.String("link_id", click.LinkID),
		)
	}
}

// Run starts the background flush worker. Blocks until ctx is cancelled,
// then drains remaining clicks and performs a final flush.
func (s *ClickService) Run(ctx context.Context) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	buf := make([]domain.Click, 0, flushBatchSize)

	for {
		select {
		case click := <-s.ch:
			buf = append(buf, click)
			if len(buf) >= flushBatchSize {
				s.flush(buf)
				buf = buf[:0]
			}

		case <-ticker.C:
			if len(buf) > 0 {
				s.flush(buf)
				buf = buf[:0]
			}

		case <-ctx.Done():
			// Drain remaining clicks from the channel.
			for {
				select {
				case click := <-s.ch:
					buf = append(buf, click)
				default:
					if len(buf) > 0 {
						s.flush(buf)
					}
					s.logger.Info("click worker stopped", zap.Int("drained", len(buf)))
					return
				}
			}
		}
	}
}

// GetStats returns aggregated click statistics for a link identified by slug.
// Returns ErrNotFound if slug does not exist, ErrForbidden if not owned by userID.
func (s *ClickService) GetStats(ctx context.Context, userID, slug string) (*ClickStats, error) {
	link, err := s.linkFinder.FindBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("click.GetStats: %w", err)
	}

	if !link.IsOwnedBy(userID) {
		return nil, domain.ErrForbidden
	}

	total, err := s.querier.CountByLink(ctx, link.ID)
	if err != nil {
		return nil, fmt.Errorf("click.GetStats count: %w", err)
	}

	byDay, err := s.querier.CountByDay(ctx, link.ID, statsDays)
	if err != nil {
		return nil, fmt.Errorf("click.GetStats by_day: %w", err)
	}

	byCountry, err := s.querier.CountByCountry(ctx, link.ID)
	if err != nil {
		return nil, fmt.Errorf("click.GetStats by_country: %w", err)
	}

	if byDay == nil {
		byDay = []domain.DayStat{}
	}
	if byCountry == nil {
		byCountry = []domain.CountryStat{}
	}

	return &ClickStats{
		TotalClicks: total,
		ByDay:       byDay,
		ByCountry:   byCountry,
	}, nil
}

func (s *ClickService) flush(clicks []domain.Click) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.inserter.BatchInsert(ctx, clicks); err != nil {
		s.logger.Error("failed to flush clicks",
			zap.Int("count", len(clicks)),
			zap.Error(err),
		)
	}
}
