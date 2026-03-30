package telegram

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go.uber.org/zap"
)

// Handler processes incoming Telegram webhook updates.
// Service dependencies will be injected here once the domain layer exists.
type Handler struct {
	logger *zap.Logger
	token  string
}

// NewHandler creates a webhook handler for the Telegram bot.
func NewHandler(logger *zap.Logger, token string) *Handler {
	return &Handler{
		logger: logger,
		token:  token,
	}
}

// HandleWebhook receives Telegram Bot API webhook updates and dispatches commands.
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var update Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		h.logger.Warn("failed to decode update", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if update.Message == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	msg := update.Message
	h.logger.Info("received message",
		zap.Int64("chat_id", msg.Chat.ID),
		zap.String("text", msg.Text),
	)

	reply := h.dispatch(msg)

	h.respondJSON(w, msg.Chat.ID, reply)
}

// dispatch routes the message to the appropriate command handler.
func (h *Handler) dispatch(msg *Message) string {
	text := strings.TrimSpace(msg.Text)

	switch {
	case strings.HasPrefix(text, "/start"):
		return "Welcome to Slink! Send me a URL and I'll shorten it.\n\nCommands:\n/list — your 5 most recent links\n/stats <slug> — click count\n/account — link your Telegram account"

	case strings.HasPrefix(text, "/stats"):
		return h.handleStats(text)

	case strings.HasPrefix(text, "/list"):
		return h.handleList()

	case strings.HasPrefix(text, "/account"):
		return h.handleAccount()

	case isURL(text):
		return h.handleShorten(text)

	default:
		return "Send me a URL to shorten, or use /start to see available commands."
	}
}

func (h *Handler) handleShorten(rawURL string) string {
	// TODO: call link service to create short link.
	return fmt.Sprintf("Shortening: %s\n(not yet implemented)", rawURL)
}

func (h *Handler) handleStats(text string) string {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return "Usage: /stats <slug>"
	}
	slug := parts[1]
	// TODO: call link service to get stats.
	return fmt.Sprintf("Stats for %s:\n(not yet implemented)", slug)
}

func (h *Handler) handleList() string {
	// TODO: call link service to list recent links.
	return "Your recent links:\n(not yet implemented)"
}

func (h *Handler) handleAccount() string {
	// TODO: call account linking service.
	return "Account linking:\n(not yet implemented)"
}

func isURL(s string) bool {
	u, err := url.ParseRequestURI(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

// respondJSON writes a sendMessage response using the webhook reply shortcut.
func (h *Handler) respondJSON(w http.ResponseWriter, chatID int64, text string) {
	resp := map[string]any{
		"method":  "sendMessage",
		"chat_id": chatID,
		"text":    text,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}
