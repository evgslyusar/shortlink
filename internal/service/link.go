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

// LinkDeleter removes a link by slug.
type LinkDeleter interface {
	DeleteBySlug(ctx context.Context, slug string) error
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

	slug, err := s.resolveSlug(ctx, customSlug)
	if err != nil {
		return nil, err
	}

	link := &domain.Link{
		ID:          uuid.NewString(),
		Slug:        slug,
		OriginalURL: rawURL,
		CreatedAt:   time.Now().UTC(),
	}

	if ownerID != "" {
		link.UserID = &ownerID
	} else {
		exp := time.Now().UTC().Add(guestLinkExpiry)
		link.ExpiresAt = &exp
	}

	if err := s.creator.CreateLink(ctx, link); err != nil {
		return nil, fmt.Errorf("link.CreateLink: %w", err)
	}

	s.logger.Info("link created",
		zap.String("slug", link.Slug),
		zap.String("link_id", link.ID),
	)

	return link, nil
}

// ListLinks returns a paginated list of links owned by the given user.
func (s *LinkService) ListLinks(ctx context.Context, userID string, page, perPage int) ([]domain.Link, int, error) {
	links, total, err := s.lister.ListByUser(ctx, userID, page, perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("link.ListLinks: %w", err)
	}
	return links, total, nil
}

// DeleteLink deletes a link by slug, verifying ownership.
func (s *LinkService) DeleteLink(ctx context.Context, userID, slug string) error {
	link, err := s.finder.FindBySlug(ctx, slug)
	if err != nil {
		return fmt.Errorf("link.DeleteLink: %w", err)
	}

	if !link.IsOwnedBy(userID) {
		return domain.ErrForbidden
	}

	if err := s.deleter.DeleteBySlug(ctx, slug); err != nil {
		return fmt.Errorf("link.DeleteLink: %w", err)
	}

	s.logger.Info("link deleted",
		zap.String("slug", slug),
		zap.String("user_id", userID),
	)

	return nil
}

func (s *LinkService) resolveSlug(ctx context.Context, customSlug string) (string, error) {
	if customSlug != "" {
		if err := domain.ValidateCustomSlug(customSlug); err != nil {
			return "", err
		}
		return customSlug, nil
	}

	for range maxSlugRetries {
		slug, err := domain.GenerateSlug()
		if err != nil {
			return "", fmt.Errorf("link.resolveSlug: %w", err)
		}

		// Check if slug is already taken by doing a dry-run find.
		// We rely on the DB unique constraint as the source of truth,
		// but pre-checking avoids unnecessary INSERT round-trips.
		_, err = s.finder.FindBySlug(ctx, slug)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return slug, nil // slug is available
			}
			return "", fmt.Errorf("link.resolveSlug: %w", err)
		}
		// slug exists, retry
	}

	return "", fmt.Errorf("link.resolveSlug: failed to generate unique slug after %d attempts: %w", maxSlugRetries, domain.ErrAlreadyExists)
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
