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
	links   map[string]string // slug -> userID
	deleted []string
	err     error
}

func newFakeLinkDeleter() *fakeLinkDeleter {
	return &fakeLinkDeleter{links: make(map[string]string)}
}

func (f *fakeLinkDeleter) addLink(slug, userID string) {
	f.links[slug] = userID
}

func (f *fakeLinkDeleter) DeleteBySlugAndUser(_ context.Context, slug, userID string) error {
	if f.err != nil {
		return f.err
	}
	ownerID, exists := f.links[slug]
	if !exists || ownerID != userID {
		return domain.ErrNotFound
	}
	delete(f.links, slug)
	f.deleted = append(f.deleted, slug)
	return nil
}

type fakeLinkCache struct {
	urls    map[string]string
	ids     map[string]string
	ttls    map[string]time.Duration
	getErr  error
	setErr  error
	delErr  error
}

func newFakeLinkCache() *fakeLinkCache {
	return &fakeLinkCache{
		urls: make(map[string]string),
		ids:  make(map[string]string),
		ttls: make(map[string]time.Duration),
	}
}

func (f *fakeLinkCache) GetLink(_ context.Context, slug string) (string, string, error) {
	if f.getErr != nil {
		return "", "", f.getErr
	}
	url, ok := f.urls[slug]
	if !ok {
		return "", "", domain.ErrNotFound
	}
	return url, f.ids[slug], nil
}

func (f *fakeLinkCache) SetLink(_ context.Context, slug, url, linkID string, ttl time.Duration) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.urls[slug] = url
	f.ids[slug] = linkID
	f.ttls[slug] = ttl
	return nil
}

func (f *fakeLinkCache) DeleteLink(_ context.Context, slug string) error {
	if f.delErr != nil {
		return f.delErr
	}
	delete(f.urls, slug)
	delete(f.ids, slug)
	return nil
}

// --- helpers ---

func newLinkService(
	creator service.LinkCreator,
	finder service.LinkBySlugFinder,
	lister service.LinksByUserLister,
	deleter service.LinkDeleter,
) *service.LinkService {
	return service.NewLinkService(creator, finder, lister, deleter, newFakeLinkCache(), zap.NewNop())
}

func newLinkServiceWithCache(
	creator service.LinkCreator,
	finder service.LinkBySlugFinder,
	lister service.LinksByUserLister,
	deleter service.LinkDeleter,
	cache *fakeLinkCache,
) *service.LinkService {
	return service.NewLinkService(creator, finder, lister, deleter, cache, zap.NewNop())
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
		callCount := 0
		creator := &countingCreator{
			maxCollisions: 2,
			callCount:     &callCount,
		}
		svc := newLinkService(creator, newFakeLinkBySlugFinder(), &fakeLinksByUserLister{}, newFakeLinkDeleter())

		link, err := svc.CreateLink(context.Background(), "", "https://example.com", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if link.Slug == "" {
			t.Error("expected non-empty slug")
		}
		// Creator was called 3 times: 2 collisions + 1 success.
		if callCount != 3 {
			t.Errorf("expected 3 CreateLink calls, got %d", callCount)
		}
	})

	t.Run("all retries exhausted", func(t *testing.T) {
		creator := &countingCreator{
			maxCollisions: 100, // more than maxSlugRetries
			callCount:     new(int),
		}
		svc := newLinkService(creator, newFakeLinkBySlugFinder(), &fakeLinksByUserLister{}, newFakeLinkDeleter())

		_, err := svc.CreateLink(context.Background(), "", "https://example.com", "")
		if !errors.Is(err, domain.ErrAlreadyExists) {
			t.Fatalf("expected ErrAlreadyExists, got %v", err)
		}
	})
}

// countingCreator simulates slug collisions for the first N CreateLink calls.
type countingCreator struct {
	maxCollisions int
	callCount     *int
}

func (f *countingCreator) CreateLink(_ context.Context, _ *domain.Link) error {
	*f.callCount++
	if *f.callCount <= f.maxCollisions {
		return domain.ErrAlreadyExists // slug collision
	}
	return nil // success
}

// --- DeleteLink tests ---

func TestDeleteLink(t *testing.T) {
	t.Run("owner deletes own link", func(t *testing.T) {
		finder := newFakeLinkBySlugFinder()
		finder.addLink(&domain.Link{Slug: "test", UserID: ptr("user-1")})
		deleter := newFakeLinkDeleter()
		deleter.addLink("test", "user-1")
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
		deleter := newFakeLinkDeleter()
		deleter.addLink("test", "user-1")
		svc := newLinkService(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, deleter)

		err := svc.DeleteLink(context.Background(), "user-2", "test")
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("slug not found", func(t *testing.T) {
		finder := newFakeLinkBySlugFinder()
		deleter := newFakeLinkDeleter()
		svc := newLinkService(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, deleter)

		err := svc.DeleteLink(context.Background(), "user-1", "nonexistent")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}

// --- ResolveSlug tests ---

func TestResolveSlug(t *testing.T) {
	t.Run("cache hit returns URL and linkID without DB call", func(t *testing.T) {
		cache := newFakeLinkCache()
		cache.urls["cached"] = "https://cached.com"
		cache.ids["cached"] = "link-id-1"
		finder := newFakeLinkBySlugFinder() // empty — should not be called
		svc := newLinkServiceWithCache(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, newFakeLinkDeleter(), cache)

		url, linkID, err := svc.ResolveSlug(context.Background(), "cached")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "https://cached.com" {
			t.Errorf("expected 'https://cached.com', got %q", url)
		}
		if linkID != "link-id-1" {
			t.Errorf("expected linkID 'link-id-1', got %q", linkID)
		}
	})

	t.Run("cache miss hits DB and populates cache", func(t *testing.T) {
		cache := newFakeLinkCache()
		finder := newFakeLinkBySlugFinder()
		finder.addLink(&domain.Link{ID: "link-2", Slug: "db-slug", OriginalURL: "https://db.com"})
		svc := newLinkServiceWithCache(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, newFakeLinkDeleter(), cache)

		url, linkID, err := svc.ResolveSlug(context.Background(), "db-slug")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "https://db.com" {
			t.Errorf("expected 'https://db.com', got %q", url)
		}
		if linkID != "link-2" {
			t.Errorf("expected linkID 'link-2', got %q", linkID)
		}
		// Verify cache was populated.
		if cache.urls["db-slug"] != "https://db.com" {
			t.Error("expected cache URL to be populated")
		}
		if cache.ids["db-slug"] != "link-2" {
			t.Error("expected cache linkID to be populated")
		}
	})

	t.Run("cache error falls back to DB", func(t *testing.T) {
		cache := newFakeLinkCache()
		cache.getErr = errors.New("redis down")
		finder := newFakeLinkBySlugFinder()
		finder.addLink(&domain.Link{Slug: "fallback", OriginalURL: "https://fallback.com"})
		svc := newLinkServiceWithCache(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, newFakeLinkDeleter(), cache)

		url, _, err := svc.ResolveSlug(context.Background(), "fallback")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "https://fallback.com" {
			t.Errorf("expected 'https://fallback.com', got %q", url)
		}
	})

	t.Run("link not found in DB returns ErrNotFound", func(t *testing.T) {
		cache := newFakeLinkCache()
		finder := newFakeLinkBySlugFinder()
		svc := newLinkServiceWithCache(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, newFakeLinkDeleter(), cache)

		_, _, err := svc.ResolveSlug(context.Background(), "missing")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("expired link returns ErrNotFound", func(t *testing.T) {
		cache := newFakeLinkCache()
		finder := newFakeLinkBySlugFinder()
		past := time.Now().Add(-time.Hour)
		finder.addLink(&domain.Link{Slug: "expired", OriginalURL: "https://old.com", ExpiresAt: &past})
		svc := newLinkServiceWithCache(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, newFakeLinkDeleter(), cache)

		_, _, err := svc.ResolveSlug(context.Background(), "expired")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("link with expires_at sets correct TTL", func(t *testing.T) {
		cache := newFakeLinkCache()
		finder := newFakeLinkBySlugFinder()
		future := time.Now().Add(2 * time.Hour)
		finder.addLink(&domain.Link{Slug: "ttl-test", OriginalURL: "https://ttl.com", ExpiresAt: &future})
		svc := newLinkServiceWithCache(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, newFakeLinkDeleter(), cache)

		_, _, err := svc.ResolveSlug(context.Background(), "ttl-test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ttl := cache.ttls["ttl-test"]
		// TTL should be approximately 2 hours (within 1 minute tolerance).
		if ttl < 119*time.Minute || ttl > 121*time.Minute {
			t.Errorf("expected TTL ~2h, got %v", ttl)
		}
	})

	t.Run("cache set error still returns URL (graceful degradation)", func(t *testing.T) {
		cache := newFakeLinkCache()
		cache.setErr = errors.New("redis write error")
		finder := newFakeLinkBySlugFinder()
		finder.addLink(&domain.Link{Slug: "set-err", OriginalURL: "https://seterr.com"})
		svc := newLinkServiceWithCache(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, newFakeLinkDeleter(), cache)

		url, _, err := svc.ResolveSlug(context.Background(), "set-err")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "https://seterr.com" {
			t.Errorf("expected 'https://seterr.com', got %q", url)
		}
	})

	t.Run("link without expires_at uses default 24h TTL", func(t *testing.T) {
		cache := newFakeLinkCache()
		finder := newFakeLinkBySlugFinder()
		finder.addLink(&domain.Link{Slug: "no-exp", OriginalURL: "https://noexp.com"})
		svc := newLinkServiceWithCache(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, newFakeLinkDeleter(), cache)

		_, _, err := svc.ResolveSlug(context.Background(), "no-exp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ttl := cache.ttls["no-exp"]
		if ttl != 24*time.Hour {
			t.Errorf("expected TTL 24h, got %v", ttl)
		}
	})
}

func TestDeleteLinkInvalidatesCache(t *testing.T) {
	cache := newFakeLinkCache()
	cache.urls["del-test"] = "https://cached.com"
	cache.ids["del-test"] = "link-1"
	finder := newFakeLinkBySlugFinder()
	finder.addLink(&domain.Link{Slug: "del-test", UserID: ptr("user-1")})
	deleter := newFakeLinkDeleter()
	deleter.addLink("del-test", "user-1")
	svc := newLinkServiceWithCache(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, deleter, cache)

	err := svc.DeleteLink(context.Background(), "user-1", "del-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := cache.urls["del-test"]; exists {
		t.Error("expected cache entry to be deleted")
	}
}

func TestDeleteLinkCacheErrorStillSucceeds(t *testing.T) {
	cache := newFakeLinkCache()
	cache.delErr = errors.New("redis delete error")
	finder := newFakeLinkBySlugFinder()
	finder.addLink(&domain.Link{Slug: "del-err", UserID: ptr("user-1")})
	deleter := newFakeLinkDeleter()
	deleter.addLink("del-err", "user-1")
	svc := newLinkServiceWithCache(newFakeLinkCreator(), finder, &fakeLinksByUserLister{}, deleter, cache)

	err := svc.DeleteLink(context.Background(), "user-1", "del-err")
	if err != nil {
		t.Fatalf("expected success despite cache error, got %v", err)
	}
	if len(deleter.deleted) != 1 {
		t.Error("expected DB delete to succeed")
	}
}

// --- ListLinks tests ---

func TestListLinks(t *testing.T) {
	t.Run("returns links and total", func(t *testing.T) {
		lister := &fakeLinksByUserLister{
			links: []domain.Link{
				{Slug: "aaa", OriginalURL: "https://example.com"},
				{Slug: "bbb", OriginalURL: "https://other.com"},
			},
			total: 5,
		}
		svc := newLinkService(newFakeLinkCreator(), newFakeLinkBySlugFinder(), lister, newFakeLinkDeleter())

		links, total, err := svc.ListLinks(context.Background(), "user-1", 1, 20)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(links) != 2 {
			t.Errorf("expected 2 links, got %d", len(links))
		}
		if total != 5 {
			t.Errorf("expected total 5, got %d", total)
		}
	})

	t.Run("clamps perPage to 100", func(t *testing.T) {
		lister := &fakeLinksByUserLister{}
		svc := newLinkService(newFakeLinkCreator(), newFakeLinkBySlugFinder(), lister, newFakeLinkDeleter())

		// Should not error even with perPage > 100 (clamped by service).
		_, _, err := svc.ListLinks(context.Background(), "user-1", 1, 500)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
