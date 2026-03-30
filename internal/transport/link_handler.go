package transport

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/domain"
)

// LinkCreatorSvc creates a shortened link.
type LinkCreatorSvc interface {
	CreateLink(ctx context.Context, ownerID, rawURL, customSlug string) (*domain.Link, error)
}

// LinkListerSvc lists links owned by a user.
type LinkListerSvc interface {
	ListLinks(ctx context.Context, userID string, page, perPage int) ([]domain.Link, int, error)
}

// LinkDeleterSvc deletes a link by slug with ownership check.
type LinkDeleterSvc interface {
	DeleteLink(ctx context.Context, userID, slug string) error
}

// LinkHandler handles HTTP requests for link endpoints.
type LinkHandler struct {
	creator LinkCreatorSvc
	lister  LinkListerSvc
	deleter LinkDeleterSvc
	baseURL string
	logger  *zap.Logger
}

// NewLinkHandler creates a new LinkHandler.
func NewLinkHandler(
	creator LinkCreatorSvc,
	lister LinkListerSvc,
	deleter LinkDeleterSvc,
	baseURL string,
	logger *zap.Logger,
) *LinkHandler {
	return &LinkHandler{
		creator: creator,
		lister:  lister,
		deleter: deleter,
		baseURL: baseURL,
		logger:  logger,
	}
}

type createLinkRequest struct {
	URL  string `json:"url"`
	Slug string `json:"slug,omitempty"`
}

type createLinkResponse struct {
	Slug        string     `json:"slug"`
	ShortURL    string     `json:"short_url"`
	OriginalURL string     `json:"original_url"`
	ExpiresAt   *time.Time `json:"expires_at"`
}

type linkItem struct {
	Slug        string     `json:"slug"`
	ShortURL    string     `json:"short_url"`
	OriginalURL string     `json:"original_url"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type listLinksResponse struct {
	Items []linkItem `json:"items"`
}

// CreateLink handles POST /v1/links.
func (h *LinkHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	req, err := decodeJSON[createLinkRequest](w, r)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, ErrCodeValidation, "invalid request body")
		return
	}

	ownerID := getUserID(r.Context())

	link, err := h.creator.CreateLink(r.Context(), ownerID, req.URL, req.Slug)
	if err != nil {
		status, code, msg := mapError(err)
		h.logError(err, status)
		respondError(w, r, status, code, msg)
		return
	}

	respondData(w, r, http.StatusCreated, createLinkResponse{
		Slug:        link.Slug,
		ShortURL:    h.baseURL + "/" + link.Slug,
		OriginalURL: link.OriginalURL,
		ExpiresAt:   link.ExpiresAt,
	})
}

// ListLinks handles GET /v1/links.
func (h *LinkHandler) ListLinks(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	if userID == "" {
		respondError(w, r, http.StatusUnauthorized, ErrCodeUnauthorized, "authentication required")
		return
	}

	page := queryInt(r, "page", 1)
	perPage := queryInt(r, "per_page", 20)

	links, total, err := h.lister.ListLinks(r.Context(), userID, page, perPage)
	if err != nil {
		h.logError(err, http.StatusInternalServerError)
		respondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "internal error")
		return
	}

	items := make([]linkItem, 0, len(links))
	for _, l := range links {
		items = append(items, linkItem{
			Slug:        l.Slug,
			ShortURL:    h.baseURL + "/" + l.Slug,
			OriginalURL: l.OriginalURL,
			ExpiresAt:   l.ExpiresAt,
			CreatedAt:   l.CreatedAt,
		})
	}

	respondDataWithMeta(w, r, http.StatusOK, listLinksResponse{Items: items}, map[string]any{
		"page":     page,
		"per_page": perPage,
		"total":    total,
	})
}

// DeleteLink handles DELETE /v1/links/{slug}.
func (h *LinkHandler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r.Context())
	if userID == "" {
		respondError(w, r, http.StatusUnauthorized, ErrCodeUnauthorized, "authentication required")
		return
	}

	slug := chi.URLParam(r, "slug")

	if err := h.deleter.DeleteLink(r.Context(), userID, slug); err != nil {
		status, code, msg := mapError(err)
		h.logError(err, status)
		respondError(w, r, status, code, msg)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *LinkHandler) logError(err error, status int) {
	if status >= http.StatusInternalServerError {
		h.logger.Error("internal error", zap.Error(err))
	}
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return defaultVal
	}
	return v
}

