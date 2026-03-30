package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/domain"
	"github.com/evgslyusar/shortlink/internal/service"
)

// --- fakes ---

type fakeClickBatchInserter struct {
	mu      sync.Mutex
	batches [][]domain.Click
	err     error
}

func (f *fakeClickBatchInserter) BatchInsert(_ context.Context, clicks []domain.Click) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]domain.Click, len(clicks))
	copy(cp, clicks)
	f.batches = append(f.batches, cp)
	return nil
}

func (f *fakeClickBatchInserter) allClicks() []domain.Click {
	f.mu.Lock()
	defer f.mu.Unlock()
	var all []domain.Click
	for _, b := range f.batches {
		all = append(all, b...)
	}
	return all
}

type fakeClickStatsQuerier struct {
	total     int64
	byDay     []domain.DayStat
	byCountry []domain.CountryStat
	err       error
}

func (f *fakeClickStatsQuerier) CountByLink(_ context.Context, _ string) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.total, nil
}

func (f *fakeClickStatsQuerier) CountByDay(_ context.Context, _ string, _ int) ([]domain.DayStat, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byDay, nil
}

func (f *fakeClickStatsQuerier) CountByCountry(_ context.Context, _ string) ([]domain.CountryStat, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byCountry, nil
}

// --- tests ---

func TestClickService_Record(t *testing.T) {
	t.Run("recorded click is flushed to inserter", func(t *testing.T) {
		inserter := &fakeClickBatchInserter{}
		finder := newFakeLinkBySlugFinder()
		svc := service.NewClickService(inserter, &fakeClickStatsQuerier{}, finder, zap.NewNop())

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			svc.Run(ctx)
		}()

		click := domain.Click{ID: "c1", LinkID: "l1", ClickedAt: time.Now().UTC()}
		svc.Record(click)

		// Wait for flush interval to trigger.
		time.Sleep(6 * time.Second)
		cancel()
		<-done

		all := inserter.allClicks()
		if len(all) != 1 {
			t.Fatalf("expected 1 click flushed, got %d", len(all))
		}
		if all[0].ID != "c1" {
			t.Errorf("expected click ID 'c1', got %q", all[0].ID)
		}
	})

	t.Run("channel full drops click without blocking", func(t *testing.T) {
		inserter := &fakeClickBatchInserter{}
		finder := newFakeLinkBySlugFinder()
		svc := service.NewClickService(inserter, &fakeClickStatsQuerier{}, finder, zap.NewNop())

		// Fill the channel without starting the worker.
		for i := range 1000 {
			svc.Record(domain.Click{ID: "fill", LinkID: "l1", ClickedAt: time.Now().UTC()})
			_ = i
		}

		// This should not block — the click is dropped.
		done := make(chan struct{})
		go func() {
			svc.Record(domain.Click{ID: "overflow", LinkID: "l1", ClickedAt: time.Now().UTC()})
			close(done)
		}()

		select {
		case <-done:
			// Success — Record did not block.
		case <-time.After(time.Second):
			t.Fatal("Record blocked on full channel")
		}
	})

	t.Run("graceful shutdown drains remaining clicks", func(t *testing.T) {
		inserter := &fakeClickBatchInserter{}
		finder := newFakeLinkBySlugFinder()
		svc := service.NewClickService(inserter, &fakeClickStatsQuerier{}, finder, zap.NewNop())

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			svc.Run(ctx)
		}()

		// Record a few clicks.
		for i := range 5 {
			svc.Record(domain.Click{ID: "drain", LinkID: "l1", ClickedAt: time.Now().UTC()})
			_ = i
		}

		// Cancel immediately — worker should drain before returning.
		time.Sleep(50 * time.Millisecond) // let clicks reach channel
		cancel()
		<-done

		all := inserter.allClicks()
		if len(all) != 5 {
			t.Errorf("expected 5 clicks drained, got %d", len(all))
		}
	})
}

func TestClickService_GetStats(t *testing.T) {
	t.Run("returns stats for owned link", func(t *testing.T) {
		finder := newFakeLinkBySlugFinder()
		finder.addLink(&domain.Link{ID: "link-1", Slug: "abc", UserID: ptr("user-1")})
		testDate := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)
		querier := &fakeClickStatsQuerier{
			total:     42,
			byDay:     []domain.DayStat{{Date: testDate, Count: 10}},
			byCountry: []domain.CountryStat{{Country: "US", Count: 30}},
		}
		svc := service.NewClickService(&fakeClickBatchInserter{}, querier, finder, zap.NewNop())

		stats, err := svc.GetStats(context.Background(), "user-1", "abc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.TotalClicks != 42 {
			t.Errorf("expected 42 total clicks, got %d", stats.TotalClicks)
		}
		if len(stats.ByDay) != 1 || !stats.ByDay[0].Date.Equal(testDate) {
			t.Errorf("unexpected by_day: %v", stats.ByDay)
		}
		if len(stats.ByCountry) != 1 || stats.ByCountry[0].Country != "US" {
			t.Errorf("unexpected by_country: %v", stats.ByCountry)
		}
	})

	t.Run("non-owner gets forbidden", func(t *testing.T) {
		finder := newFakeLinkBySlugFinder()
		finder.addLink(&domain.Link{ID: "link-1", Slug: "abc", UserID: ptr("user-1")})
		svc := service.NewClickService(&fakeClickBatchInserter{}, &fakeClickStatsQuerier{}, finder, zap.NewNop())

		_, err := svc.GetStats(context.Background(), "user-2", "abc")
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("slug not found", func(t *testing.T) {
		finder := newFakeLinkBySlugFinder()
		svc := service.NewClickService(&fakeClickBatchInserter{}, &fakeClickStatsQuerier{}, finder, zap.NewNop())

		_, err := svc.GetStats(context.Background(), "user-1", "missing")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("empty stats return empty slices not nil", func(t *testing.T) {
		finder := newFakeLinkBySlugFinder()
		finder.addLink(&domain.Link{ID: "link-1", Slug: "abc", UserID: ptr("user-1")})
		querier := &fakeClickStatsQuerier{total: 0}
		svc := service.NewClickService(&fakeClickBatchInserter{}, querier, finder, zap.NewNop())

		stats, err := svc.GetStats(context.Background(), "user-1", "abc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.ByDay == nil {
			t.Error("expected non-nil ByDay slice")
		}
		if stats.ByCountry == nil {
			t.Error("expected non-nil ByCountry slice")
		}
	})
}
