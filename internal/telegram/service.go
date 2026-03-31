package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/domain"
	"github.com/google/uuid"
)

// AccountLinker creates a telegram-to-user binding.
type AccountLinker interface {
	LinkTelegram(ctx context.Context, account *domain.TelegramAccount) error
}

// AccountFinder looks up a telegram account by Telegram user ID.
type AccountFinder interface {
	FindByTelegramID(ctx context.Context, telegramID int64) (*domain.TelegramAccount, error)
}

// Authenticator verifies user credentials.
type Authenticator interface {
	Login(ctx context.Context, email, password string) (*domain.User, error)
}

// LinkCreatorSvc creates a shortened link.
type LinkCreatorSvc interface {
	CreateLink(ctx context.Context, ownerID, rawURL, customSlug string) (*domain.Link, error)
}

// LinkListerSvc lists links owned by a user.
type LinkListerSvc interface {
	ListLinks(ctx context.Context, userID string, page, perPage int) ([]domain.Link, int, error)
}

// LinkBySlugFinder looks up a link by slug.
type LinkBySlugFinder interface {
	FindBySlug(ctx context.Context, slug string) (*domain.Link, error)
}

// ClickCounter returns the total click count for a link.
type ClickCounter interface {
	CountByLink(ctx context.Context, linkID string) (int64, error)
}

// Service orchestrates Telegram bot commands using domain services.
type Service struct {
	linker      AccountLinker
	finder      AccountFinder
	auth        Authenticator
	linkCreator LinkCreatorSvc
	linkLister  LinkListerSvc
	linkFinder  LinkBySlugFinder
	clicks      ClickCounter
	baseURL     string
	logger      *zap.Logger
}

// NewService creates a new Telegram Service.
func NewService(
	linker AccountLinker,
	finder AccountFinder,
	auth Authenticator,
	linkCreator LinkCreatorSvc,
	linkLister LinkListerSvc,
	linkFinder LinkBySlugFinder,
	clicks ClickCounter,
	baseURL string,
	logger *zap.Logger,
) *Service {
	return &Service{
		linker:      linker,
		finder:      finder,
		auth:        auth,
		linkCreator: linkCreator,
		linkLister:  linkLister,
		linkFinder:  linkFinder,
		clicks:      clicks,
		baseURL:     baseURL,
		logger:      logger,
	}
}

// Shorten creates a short link. If the Telegram user is linked, the link is owned.
func (s *Service) Shorten(ctx context.Context, telegramID int64, rawURL string) string {
	ownerID := s.resolveOwner(ctx, telegramID)

	link, err := s.linkCreator.CreateLink(ctx, ownerID, rawURL, "")
	if err != nil {
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			return fmt.Sprintf("Invalid URL: %s", ve.Message)
		}
		s.logger.Error("failed to create link", zap.Error(err))
		return "Failed to create short link. Please try again."
	}

	return fmt.Sprintf("%s/%s", s.baseURL, link.Slug)
}

// List returns the 5 most recent links for a linked account.
func (s *Service) List(ctx context.Context, telegramID int64) string {
	account, err := s.finder.FindByTelegramID(ctx, telegramID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return needLinkMessage
		}
		s.logger.Error("failed to find account", zap.Error(err))
		return "Something went wrong. Please try again."
	}

	links, _, err := s.linkLister.ListLinks(ctx, account.UserID, 1, 5)
	if err != nil {
		s.logger.Error("failed to list links", zap.Error(err))
		return "Failed to fetch links. Please try again."
	}

	if len(links) == 0 {
		return "You have no links yet. Send me a URL to create one!"
	}

	var b strings.Builder
	b.WriteString("Your recent links:\n")
	for _, l := range links {
		fmt.Fprintf(&b, "• %s/%s → %s\n", s.baseURL, l.Slug, l.OriginalURL)
	}
	return b.String()
}

// Stats returns the total click count for a slug owned by the linked account.
func (s *Service) Stats(ctx context.Context, telegramID int64, slug string) string {
	account, err := s.finder.FindByTelegramID(ctx, telegramID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return needLinkMessage
		}
		s.logger.Error("failed to find account", zap.Error(err))
		return "Something went wrong. Please try again."
	}

	link, err := s.linkFinder.FindBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Sprintf("Link %q not found.", slug)
		}
		s.logger.Error("failed to find link", zap.Error(err))
		return "Something went wrong. Please try again."
	}

	if !link.IsOwnedBy(account.UserID) {
		return "You don't own this link."
	}

	count, err := s.clicks.CountByLink(ctx, link.ID)
	if err != nil {
		s.logger.Error("failed to count clicks", zap.Error(err))
		return "Failed to fetch stats. Please try again."
	}

	return fmt.Sprintf("Stats for %s/%s:\nTotal clicks: %d", s.baseURL, slug, count)
}

// AccountConnect verifies credentials and links a Telegram user to a Slink account.
func (s *Service) AccountConnect(ctx context.Context, telegramID int64, username, email, password string) string {
	user, err := s.auth.Login(ctx, email, password)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			return "Invalid email or password."
		}
		s.logger.Error("login failed during account connect", zap.Error(err))
		return "Something went wrong. Please try again."
	}

	account := &domain.TelegramAccount{
		ID:         uuid.NewString(),
		UserID:     user.ID,
		TelegramID: telegramID,
		Username:   username,
		LinkedAt:   time.Now().UTC(),
	}
	if err := s.linker.LinkTelegram(ctx, account); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return "This account is already linked."
		}
		s.logger.Error("failed to link telegram account", zap.Error(err))
		return "Failed to link account. Please try again."
	}

	s.logger.Info("telegram account linked",
		zap.String("user_id", user.ID),
		zap.Int64("telegram_id", telegramID),
	)
	return fmt.Sprintf("Account linked! Your Telegram is now connected to %s.", email)
}

// AccountInfo checks if the Telegram user has a linked account.
func (s *Service) AccountInfo(ctx context.Context, telegramID int64) string {
	_, err := s.finder.FindByTelegramID(ctx, telegramID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "Your Telegram account is not linked.\n\nTo link: /account connect <email> <password>"
		}
		s.logger.Error("failed to find account", zap.Error(err))
		return "Something went wrong. Please try again."
	}
	return "Your Telegram account is linked."
}

func (s *Service) resolveOwner(ctx context.Context, telegramID int64) string {
	account, err := s.finder.FindByTelegramID(ctx, telegramID)
	if err != nil {
		return "" // guest
	}
	return account.UserID
}

const needLinkMessage = "You need to link your Telegram account first.\n\nUse: /account connect <email> <password>"
