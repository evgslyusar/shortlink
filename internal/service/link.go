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
	maxSlugRetries    = 3
	guestLinkExpiry   = 7 * 24 * time.Hour
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

// LinkService handles link creation, listing, and deletion.
type LinkService struct {
	creator LinkCreator
	finder  LinkBySlugFinder
	lister  LinksByUserLister
	deleter LinkDeleter
	logger  *zap.Logger
}

// NewLinkService creates a new LinkService.
func NewLinkService(
	creator LinkCreator,
	finder LinkBySlugFinder,
	lister LinksByUserLister,
	deleter LinkDeleter,
	logger *zap.Logger,
) *LinkService {
	return &LinkService{
		creator: creator,
		finder:  finder,
		lister:  lister,
		deleter: deleter,
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
