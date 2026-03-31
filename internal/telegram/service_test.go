package telegram_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/domain"
	"github.com/evgslyusar/shortlink/internal/telegram"
)

// --- fakes ---

type fakeAccountLinker struct {
	accounts map[int64]*domain.TelegramAccount
	err      error
}

func newFakeAccountLinker() *fakeAccountLinker {
	return &fakeAccountLinker{accounts: make(map[int64]*domain.TelegramAccount)}
}

func (f *fakeAccountLinker) LinkTelegram(_ context.Context, a *domain.TelegramAccount) error {
	if f.err != nil {
		return f.err
	}
	if _, exists := f.accounts[a.TelegramID]; exists {
		return domain.ErrAlreadyExists
	}
	f.accounts[a.TelegramID] = a
	return nil
}

type fakeAccountFinder struct {
	accounts map[int64]*domain.TelegramAccount
}

func newFakeAccountFinder() *fakeAccountFinder {
	return &fakeAccountFinder{accounts: make(map[int64]*domain.TelegramAccount)}
}

func (f *fakeAccountFinder) add(telegramID int64, userID string) {
	f.accounts[telegramID] = &domain.TelegramAccount{
		TelegramID: telegramID,
		UserID:     userID,
	}
}

func (f *fakeAccountFinder) FindByTelegramID(_ context.Context, telegramID int64) (*domain.TelegramAccount, error) {
	a, ok := f.accounts[telegramID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

type fakeAuth struct {
	users map[string]*domain.User // email → user
}

func newFakeAuth() *fakeAuth {
	return &fakeAuth{users: make(map[string]*domain.User)}
}

func (f *fakeAuth) addUser(email, password, userID string) {
	f.users[email] = &domain.User{ID: userID, Email: email, Password: password}
}

func (f *fakeAuth) Login(_ context.Context, email, password string) (*domain.User, error) {
	u, ok := f.users[email]
	if !ok || u.Password != password {
		return nil, domain.ErrUnauthorized
	}
	return u, nil
}

type fakeLinkCreator struct {
	slugCounter int
}

func (f *fakeLinkCreator) CreateLink(_ context.Context, ownerID, rawURL, _ string) (*domain.Link, error) {
	f.slugCounter++
	return &domain.Link{
		ID:          "link-id",
		Slug:        "abc123",
		OriginalURL: rawURL,
		UserID:      strPtr(ownerID),
	}, nil
}

type fakeLinkLister struct {
	links []domain.Link
}

func (f *fakeLinkLister) ListLinks(_ context.Context, _ string, _, _ int) ([]domain.Link, int, error) {
	return f.links, len(f.links), nil
}

type fakeLinkFinder struct {
	links map[string]*domain.Link
}

func newFakeLinkFinder() *fakeLinkFinder {
	return &fakeLinkFinder{links: make(map[string]*domain.Link)}
}

func (f *fakeLinkFinder) add(link *domain.Link) {
	f.links[link.Slug] = link
}

func (f *fakeLinkFinder) FindBySlug(_ context.Context, slug string) (*domain.Link, error) {
	l, ok := f.links[slug]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return l, nil
}

type fakeClickCounter struct {
	counts map[string]int64
}

func newFakeClickCounter() *fakeClickCounter {
	return &fakeClickCounter{counts: make(map[string]int64)}
}

func (f *fakeClickCounter) CountByLink(_ context.Context, linkID string) (int64, error) {
	return f.counts[linkID], nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// --- helpers ---

func newTestService(
	linker telegram.AccountLinker,
	finder telegram.AccountFinder,
	auth telegram.Authenticator,
	linkCreator telegram.LinkCreatorSvc,
	linkLister telegram.LinkListerSvc,
	linkFinder telegram.LinkBySlugFinder,
	clicks telegram.ClickCounter,
) *telegram.Service {
	return telegram.NewService(
		linker, finder, auth,
		linkCreator, linkLister, linkFinder, clicks,
		"http://localhost:8080", zap.NewNop(),
	)
}

// --- tests ---

func TestService_Shorten(t *testing.T) {
	t.Run("guest shortens URL", func(t *testing.T) {
		svc := newTestService(
			newFakeAccountLinker(), newFakeAccountFinder(), newFakeAuth(),
			&fakeLinkCreator{}, &fakeLinkLister{}, newFakeLinkFinder(), newFakeClickCounter(),
		)
		result := svc.Shorten(context.Background(), 123, "https://example.com")
		if result != "http://localhost:8080/abc123" {
			t.Errorf("expected short URL, got %q", result)
		}
	})

	t.Run("linked user shortens URL", func(t *testing.T) {
		finder := newFakeAccountFinder()
		finder.add(123, "user-1")
		svc := newTestService(
			newFakeAccountLinker(), finder, newFakeAuth(),
			&fakeLinkCreator{}, &fakeLinkLister{}, newFakeLinkFinder(), newFakeClickCounter(),
		)
		result := svc.Shorten(context.Background(), 123, "https://example.com")
		if !strings.Contains(result, "abc123") {
			t.Errorf("expected short URL with slug, got %q", result)
		}
	})
}

func TestService_List(t *testing.T) {
	t.Run("unlinked user gets prompt", func(t *testing.T) {
		svc := newTestService(
			newFakeAccountLinker(), newFakeAccountFinder(), newFakeAuth(),
			&fakeLinkCreator{}, &fakeLinkLister{}, newFakeLinkFinder(), newFakeClickCounter(),
		)
		result := svc.List(context.Background(), 999)
		if !strings.Contains(result, "link your Telegram") {
			t.Errorf("expected link prompt, got %q", result)
		}
	})

	t.Run("linked user gets links", func(t *testing.T) {
		finder := newFakeAccountFinder()
		finder.add(123, "user-1")
		lister := &fakeLinkLister{
			links: []domain.Link{
				{Slug: "aaa", OriginalURL: "https://a.com", CreatedAt: time.Now()},
				{Slug: "bbb", OriginalURL: "https://b.com", CreatedAt: time.Now()},
			},
		}
		svc := newTestService(
			newFakeAccountLinker(), finder, newFakeAuth(),
			&fakeLinkCreator{}, lister, newFakeLinkFinder(), newFakeClickCounter(),
		)
		result := svc.List(context.Background(), 123)
		if !strings.Contains(result, "aaa") || !strings.Contains(result, "bbb") {
			t.Errorf("expected links in output, got %q", result)
		}
	})
}

func TestService_Stats(t *testing.T) {
	t.Run("returns click count for owned link", func(t *testing.T) {
		finder := newFakeAccountFinder()
		finder.add(123, "user-1")
		linkFinder := newFakeLinkFinder()
		linkFinder.add(&domain.Link{ID: "link-1", Slug: "abc", UserID: strPtr("user-1")})
		clicks := newFakeClickCounter()
		clicks.counts["link-1"] = 42

		svc := newTestService(
			newFakeAccountLinker(), finder, newFakeAuth(),
			&fakeLinkCreator{}, &fakeLinkLister{}, linkFinder, clicks,
		)
		result := svc.Stats(context.Background(), 123, "abc")
		if !strings.Contains(result, "42") {
			t.Errorf("expected click count 42, got %q", result)
		}
	})

	t.Run("unlinked user gets prompt", func(t *testing.T) {
		svc := newTestService(
			newFakeAccountLinker(), newFakeAccountFinder(), newFakeAuth(),
			&fakeLinkCreator{}, &fakeLinkLister{}, newFakeLinkFinder(), newFakeClickCounter(),
		)
		result := svc.Stats(context.Background(), 999, "abc")
		if !strings.Contains(result, "link your Telegram") {
			t.Errorf("expected link prompt, got %q", result)
		}
	})

	t.Run("non-owned link returns error", func(t *testing.T) {
		finder := newFakeAccountFinder()
		finder.add(123, "user-1")
		linkFinder := newFakeLinkFinder()
		linkFinder.add(&domain.Link{ID: "link-1", Slug: "abc", UserID: strPtr("user-2")})

		svc := newTestService(
			newFakeAccountLinker(), finder, newFakeAuth(),
			&fakeLinkCreator{}, &fakeLinkLister{}, linkFinder, newFakeClickCounter(),
		)
		result := svc.Stats(context.Background(), 123, "abc")
		if !strings.Contains(result, "don't own") {
			t.Errorf("expected ownership error, got %q", result)
		}
	})
}

func TestService_AccountConnect(t *testing.T) {
	t.Run("successful connect", func(t *testing.T) {
		auth := newFakeAuth()
		auth.addUser("test@example.com", "password123", "user-1")
		linker := newFakeAccountLinker()

		svc := newTestService(
			linker, newFakeAccountFinder(), auth,
			&fakeLinkCreator{}, &fakeLinkLister{}, newFakeLinkFinder(), newFakeClickCounter(),
		)
		result := svc.AccountConnect(context.Background(), 123, "testuser", "test@example.com", "password123")
		if !strings.Contains(result, "linked") {
			t.Errorf("expected success message, got %q", result)
		}
		if len(linker.accounts) != 1 {
			t.Error("expected account to be created")
		}
	})

	t.Run("wrong credentials", func(t *testing.T) {
		auth := newFakeAuth()
		auth.addUser("test@example.com", "password123", "user-1")

		svc := newTestService(
			newFakeAccountLinker(), newFakeAccountFinder(), auth,
			&fakeLinkCreator{}, &fakeLinkLister{}, newFakeLinkFinder(), newFakeClickCounter(),
		)
		result := svc.AccountConnect(context.Background(), 123, "testuser", "test@example.com", "wrong")
		if !strings.Contains(result, "Invalid email or password") {
			t.Errorf("expected credentials error, got %q", result)
		}
	})

	t.Run("already linked", func(t *testing.T) {
		auth := newFakeAuth()
		auth.addUser("test@example.com", "password123", "user-1")
		linker := newFakeAccountLinker()
		linker.accounts[123] = &domain.TelegramAccount{TelegramID: 123}

		svc := newTestService(
			linker, newFakeAccountFinder(), auth,
			&fakeLinkCreator{}, &fakeLinkLister{}, newFakeLinkFinder(), newFakeClickCounter(),
		)
		result := svc.AccountConnect(context.Background(), 123, "testuser", "test@example.com", "password123")
		if !strings.Contains(result, "already linked") {
			t.Errorf("expected already linked message, got %q", result)
		}
	})
}

func TestService_AccountInfo(t *testing.T) {
	t.Run("unlinked user gets instructions", func(t *testing.T) {
		svc := newTestService(
			newFakeAccountLinker(), newFakeAccountFinder(), newFakeAuth(),
			&fakeLinkCreator{}, &fakeLinkLister{}, newFakeLinkFinder(), newFakeClickCounter(),
		)
		result := svc.AccountInfo(context.Background(), 999)
		if !strings.Contains(result, "not linked") {
			t.Errorf("expected not linked message, got %q", result)
		}
		if !strings.Contains(result, "/account connect") {
			t.Errorf("expected connect instructions, got %q", result)
		}
	})

	t.Run("linked user gets confirmation", func(t *testing.T) {
		finder := newFakeAccountFinder()
		finder.add(123, "user-1")
		svc := newTestService(
			newFakeAccountLinker(), finder, newFakeAuth(),
			&fakeLinkCreator{}, &fakeLinkLister{}, newFakeLinkFinder(), newFakeClickCounter(),
		)
		result := svc.AccountInfo(context.Background(), 123)
		if !strings.Contains(result, "linked") {
			t.Errorf("expected linked confirmation, got %q", result)
		}
	})
}
