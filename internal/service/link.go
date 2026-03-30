package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/domain"
	"github.com/google/uuid"
)

const (
	maxSlugRetries  = 3
	guestLinkExpiry = 7 * 24 * time.Hour
	defaultCacheTTL = 24 * time.Hour
)

// LinkCreator persists a new link.
type LinkCreator interface {
	CreateLink(ctx context.Context, link *domain.Link) error
}

// LinkBySlugFinder looks up a link by slug.
type LinkBySlugFinder interface {
	FindBySlug(ctx context.Context, slug string) (*domain.Link, error)
}

// LinksByUserLister returns a paginated list of links for a user.
type LinksByUserLister interface {
	ListByUser(ctx context.Context, userID string, page, perPage int) ([]domain.Link, int, error)
}

// LinkDeleter removes a link by slug and owner atomically.
type LinkDeleter interface {
	DeleteBySlugAndUser(ctx context.Context, slug, userID string) error
}

// LinkCache provides slug→URL caching.
type LinkCache interface {
	GetOriginalURL(ctx context.Context, slug string) (string, error)
	SetOriginalURL(ctx context.Context, slug, url string, ttl time.Duration) error
	DeleteOriginalURL(ctx context.Context, slug string) error
}

// LinkService handles link creation, listing, deletion, and resolution.
type LinkService struct {
	creator LinkCreator
	finder  LinkBySlugFinder
	lister  LinksByUserLister
	deleter LinkDeleter
	cache   LinkCache
	logger  *zap.Logger
}

// NewLinkService creates a new LinkService.
func NewLinkService(
	creator LinkCreator,
	finder LinkBySlugFinder,
	lister LinksByUserLister,
	deleter LinkDeleter,
	cache LinkCache,
	logger *zap.Logger,
) *LinkService {
	return &LinkService{
		creator: creator,
		finder:  finder,
		lister:  lister,
		deleter: deleter,
		cache:   cache,
		logger:  logger,
	}
}

// CreateLink creates a new shortened link. If ownerID is empty, the link is
// treated as a guest link and expires in 7 days.
func (s *LinkService) CreateLink(ctx context.Context, ownerID, rawURL, customSlug string) (*domain.Link, error) {
	if err := validateURL(rawURL); err != nil {
		return nil, err
	}

	if customSlug != "" {
		if err := domain.ValidateCustomSlug(customSlug); err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()

	buildLink := func(slug string) *domain.Link {
		link := &domain.Link{
			ID:          uuid.NewString(),
			Slug:        slug,
			OriginalURL: rawURL,
			CreatedAt:   now,
		}
		if ownerID != "" {
			link.UserID = &ownerID
		} else {
			exp := now.Add(guestLinkExpiry)
			link.ExpiresAt = &exp
		}
		return link
	}

	// Custom slug: single attempt, no retry.
	if customSlug != "" {
		link := buildLink(customSlug)
		if err := s.creator.CreateLink(ctx, link); err != nil {
			return nil, fmt.Errorf("link.CreateLink: %w", err)
		}
		s.logger.Info("link created", zap.String("slug", link.Slug), zap.String("link_id", link.ID))
		return link, nil
	}

	// Auto-generated slug: retry on collision.
	for range maxSlugRetries {
		slug, err := domain.GenerateSlug()
		if err != nil {
			return nil, fmt.Errorf("link.CreateLink: %w", err)
		}

		link := buildLink(slug)
		err = s.creator.CreateLink(ctx, link)
		if err == nil {
			s.logger.Info("link created", zap.String("slug", link.Slug), zap.String("link_id", link.ID))
			return link, nil
		}
		if !errors.Is(err, domain.ErrAlreadyExists) {
			return nil, fmt.Errorf("link.CreateLink: %w", err)
		}
		// slug collision, retry with a new slug
	}

	return nil, fmt.Errorf("link.CreateLink: failed after %d attempts: %w", maxSlugRetries, domain.ErrAlreadyExists)
}

// ListLinks returns a paginated list of links owned by the given user.
func (s *LinkService) ListLinks(ctx context.Context, userID string, page, perPage int) ([]domain.Link, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	links, total, err := s.lister.ListByUser(ctx, userID, page, perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("link.ListLinks: %w", err)
	}
	return links, total, nil
}

// DeleteLink deletes a link by slug, verifying ownership atomically.
// Returns ErrNotFound if the slug does not exist, ErrForbidden if owned by another user.
func (s *LinkService) DeleteLink(ctx context.Context, userID, slug string) error {
	err := s.deleter.DeleteBySlugAndUser(ctx, slug, userID)
	if err == nil {
		// Use a detached context for cache invalidation — the DB delete already
		// committed, so cache cleanup should proceed even if the request is cancelled.
		cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cacheCancel()
		if cacheErr := s.cache.DeleteOriginalURL(cacheCtx, slug); cacheErr != nil {
			s.logger.Warn("cache delete error", zap.String("slug", slug), zap.Error(cacheErr))
		}
		s.logger.Info("link deleted", zap.String("slug", slug), zap.String("user_id", userID))
		return nil
	}

	if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("link.DeleteLink: %w", err)
	}

	// Atomic delete returned "not found" — determine if slug doesn't exist or belongs to another user.
	link, findErr := s.finder.FindBySlug(ctx, slug)
	if findErr != nil {
		// Slug truly doesn't exist.
		return fmt.Errorf("link.DeleteLink: %w", domain.ErrNotFound)
	}

	if !link.IsOwnedBy(userID) {
		return domain.ErrForbidden
	}

	// Link exists and is owned by this user but delete failed — unexpected.
	return fmt.Errorf("link.DeleteLink: %w", err)
}


// ResolveSlug returns the original URL for a slug, using cache where possible.
// Returns domain.ErrNotFound if the slug does not exist or the link is expired.
func (s *LinkService) ResolveSlug(ctx context.Context, slug string) (string, error) {
	// 1. Try cache.
	originalURL, err := s.cache.GetOriginalURL(ctx, slug)
	if err == nil {
		return originalURL, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		s.logger.Warn("cache get error, falling back to DB", zap.String("slug", slug), zap.Error(err))
	}

	// 2. Cache miss or error — hit DB.
	link, err := s.finder.FindBySlug(ctx, slug)
	if err != nil {
		return "", fmt.Errorf("link.ResolveSlug: %w", err)
	}

	// 3. Compute TTL and check expiry in one step.
	ttl := defaultCacheTTL
	if link.ExpiresAt != nil {
		ttl = time.Until(*link.ExpiresAt)
		if ttl <= 0 {
			return "", fmt.Errorf("link.ResolveSlug: link expired: %w", domain.ErrNotFound)
		}
	}

	// 4. Populate cache (best-effort).
	if cacheErr := s.cache.SetOriginalURL(ctx, slug, link.OriginalURL, ttl); cacheErr != nil {
		s.logger.Warn("cache set error", zap.String("slug", slug), zap.Error(cacheErr))
	}

	return link.OriginalURL, nil
}

func validateURL(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return &domain.ValidationError{
			Field:   "url",
			Message: "must be a valid HTTP or HTTPS URL",
		}
	}
	return nil
}
