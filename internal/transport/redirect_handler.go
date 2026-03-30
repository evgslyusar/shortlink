package transport

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/domain"
)

// SlugResolver resolves a slug to its original URL.
type SlugResolver interface {
	ResolveSlug(ctx context.Context, slug string) (string, error)
}

// RedirectHandler handles short link redirects.
type RedirectHandler struct {
	resolver SlugResolver
	logger   *zap.Logger
}

// NewRedirectHandler creates a new RedirectHandler.
func NewRedirectHandler(resolver SlugResolver, logger *zap.Logger) *RedirectHandler {
	return &RedirectHandler{
		resolver: resolver,
		logger:   logger,
	}
}

// Redirect handles GET /{slug} — resolves the slug and issues a 302 redirect.
func (h *RedirectHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	url, err := h.resolver.ResolveSlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			respondError(w, r, http.StatusNotFound, ErrCodeNotFound, "link not found")
			return
		}
		h.logger.Error("resolve slug error", zap.String("slug", slug), zap.Error(err))
		respondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "internal error")
		return
	}

	http.Redirect(w, r, url, http.StatusFound)
}
