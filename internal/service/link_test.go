package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/domain"
	"github.com/evgslyusar/shortlink/internal/service"
)

// --- fakes ---

type fakeLinkCreator struct {
	links map[string]*domain.Link
	err   error
}

func newFakeLinkCreator() *fakeLinkCreator {
	return &fakeLinkCreator{links: make(map[string]*domain.Link)}
}

func (f *fakeLinkCreator) CreateLink(_ context.Context, link *domain.Link) error {
	if f.err != nil {
		return f.err
	}
	if _, exists := f.links[link.Slug]; exists {
		return domain.ErrAlreadyExists
	}
	f.links[link.Slug] = link
	return nil
}

type fakeLinkBySlugFinder struct {
	links map[string]*domain.Link
}

func newFakeLinkBySlugFinder() *fakeLinkBySlugFinder {
	return &fakeLinkBySlugFinder{links: make(map[string]*domain.Link)}
}

func (f *fakeLinkBySlugFinder) addLink(link *domain.Link) {
	f.links[link.Slug] = link
}

func (f *fakeLinkBySlugFinder) FindBySlug(_ context.Context, slug string) (*domain.Link, error) {
	link, ok := f.links[slug]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return link, nil
}

type fakeLinksByUserLister struct {
	links []domain.Link
	total int
	err   error
}

func (f *fakeLinksByUserLister) ListByUser(_ context.Context, _ string, _, _ int) ([]domain.Link, int, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.links, f.total, nil
}

type fakeLinkDeleter struct {
	deleted []string
	err     error
}

func (f *fakeLinkDeleter) DeleteBySlug(_ context.Context, slug string) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, slug)
	return nil
}

// --- helpers ---

func newLinkService(
	creator service.LinkCreator,
	finder service.LinkBySlugFinder,
	lister service.LinksByUserLister,
	deleter service.LinkDeleter,
) *service.LinkService {
	return service.NewLinkService(creator, finder, lister, deleter, zap.NewNop())
}

func ptr(s string) *string { return &s }

// --- CreateLink tests ---

func TestCreateLink(t *testing.T) {
	t.Run("guest link sets expires_at to ~7 days", func(t *testing.T) {
		creator := newFakeLinkCreator()
		finder := newFakeLinkBySlugFinder()
		svc := newLinkService(creator, finder, &fakeLinksByUserLister{}, &fakeLinkDeleter{})

		link, err := svc.CreateLink(context.Background(), "", "https://example.com", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if link.UserID != nil {
			t.Error("expected nil UserID for guest link")
		}
		if link.ExpiresAt == nil {
			t.Fatal("expected non-nil ExpiresAt for guest link")
		}
		// Verify expiry is approximately 7 days from now (within 1 minute tolerance).
		expected := time.Now().UTC().Add(7 * 24 * time.Hour)
		diff := link.ExpiresAt.Sub(expected)
		if diff < -time.Minute || diff > time.Minute {
			t.Errorf("ExpiresAt %v not within 1 minute of expected %v", link.ExpiresAt, expected)
		}
	})

	t.Run("user link has nil expires_at", func(t *testing.T) {
		creator := newFakeLinkCreator()
		finder := newFakeLinkBySlugFinder()
		svc := newLinkService(creator, finder, &fakeLinksByUserLister{}, &fakeLinkDeleter{})

		link, err := svc.CreateLink(context.Background(), "user-123", "https://example.com", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if link.UserID == nil || *link.UserID != "user-123" {
			t.Errorf("expected UserID 'user-123', got %v", link.UserID)
		}
		if link.ExpiresAt != nil {
			t.Error("expected nil ExpiresAt for user link")
		}
	})

	t.Run("custom slug valid", func(t *testing.T) {
		creator := newFakeLinkCreator()
		finder := newFakeLinkBySlugFinder()
		svc := newLinkService(creator, finder, &fakeLinksByUserLister{}, &fakeLinkDeleter{})

		link, err := svc.CreateLink(context.Background(), "user-1", "https://example.com", "my-slug")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if link.Slug != "my-slug" {
			t.Errorf("expected slug 'my-slug', got %q", link.Slug)
		}
	})

	t.Run("custom slug invalid", func(t *testing.T) {
		creator := newFakeLinkCreator()
		finder := newFakeLinkBySlugFinder()
		svc := newLinkService(creator, finder, &fakeLinksByUserLister{}, &fakeLinkDeleter{})

		_, err := svc.CreateLink(context.Background(), "user-1", "https://example.com", "ab")
		var ve *domain.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected ValidationError, got %v", err)
		}
		if ve.Field != "slug" {
			t.Errorf("expected field 'slug', got %q", ve.Field)
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		creator := newFakeLinkCreator()
		finder := newFakeLinkBySlugFinder()
		svc := newLinkService(creator, finder, &fakeLinksByUserLister{}, &fakeLinkDeleter{})

		_, err := svc.CreateLink(context.Background(), "", "not-a-url", "")
		var ve *domain.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected ValidationError, got %v", err)
		}
		if ve.Field != "url" {
			t.Errorf("expected field 'url', got %q", ve.Field)
		}
	})

	t.Run("collision retry succeeds on third attempt", func(t *testing.T) {
		// Finder returns "found" for the first 2 generated slugs, then "not found".
		callCount := 0
		finder := &countingFinder{
			maxCollisions: 2,
			callCount:     &callCount,
		}
		creator := newFakeLinkCreator()
		svc := newLinkService(creator, finder, &fakeLinksByUserLister{}, &fakeLinkDeleter{})

		link, err := svc.CreateLink(context.Background(), "", "https://example.com", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if link.Slug == "" {
			t.Error("expected non-empty slug")
		}
		// Finder was called 3 times: 2 collisions + 1 available.
		if callCount != 3 {
			t.Errorf("expected 3 FindBySlug calls, got %d", callCount)
		}
	})

	t.Run("all retries exhausted", func(t *testing.T) {
		// Finder always returns "found" (slug exists).
		finder := &countingFinder{
			maxCollisions: 100, // more than maxSlugRetries
			callCount:     new(int),
		}
		creator := newFakeLinkCreator()
		svc := newLinkService(creator, finder, &fakeLinksByUserLister{}, &fakeLinkDeleter{})

		_, err := svc.CreateLink(context.Background(), "", "https://example.com", "")
		if !errors.Is(err, domain.ErrAlreadyExists) {
			t.Fatalf("expected ErrAlreadyExists, got %v", err)
		}
	})
}

// countingFinder simulates slug collisions for the first N calls.
type countingFinder struct {
	maxCollisions int
	callCount     *int
}

func (f *countingFinder) FindBySlug(_ context.Context, slug string) (*domain.Link, error) {
	*f.callCount++
	if *f.callCount <= f.maxCollisions {
		return &domain.Link{Slug: slug}, nil // slug exists (collision)
	}
	return nil, domain.ErrNotFound // slug available
}

// --- DeleteLink tests ---

func TestDeleteLink(t *testing.T) {
	t.Run("owner deletes own link", func(t *testing.T) {
		finder := newFakeLinkBySlugFinder()
		finder.addLink(&domain.Link{Slug: "test", UserID: ptr("user-1")})
		deleter := &fakeLinkDeleter{}
		svc := newLinkService(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, deleter)

		err := svc.DeleteLink(context.Background(), "user-1", "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(deleter.deleted) != 1 || deleter.deleted[0] != "test" {
			t.Errorf("expected slug 'test' to be deleted, got %v", deleter.deleted)
		}
	})

	t.Run("non-owner gets forbidden", func(t *testing.T) {
		finder := newFakeLinkBySlugFinder()
		finder.addLink(&domain.Link{Slug: "test", UserID: ptr("user-1")})
		svc := newLinkService(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, &fakeLinkDeleter{})

		err := svc.DeleteLink(context.Background(), "user-2", "test")
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("slug not found", func(t *testing.T) {
		finder := newFakeLinkBySlugFinder()
		svc := newLinkService(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, &fakeLinkDeleter{})

		err := svc.DeleteLink(context.Background(), "user-1", "nonexistent")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}
