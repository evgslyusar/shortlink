package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// Handler processes incoming Telegram webhook updates.
type Handler struct {
	svc    *Service
	logger *zap.Logger
}

// NewHandler creates a webhook handler for the Telegram bot.
func NewHandler(svc *Service, logger *zap.Logger) *Handler {
	return &Handler{
		svc:    svc,
		logger: logger,
	}
}

// HandleWebhook receives Telegram Bot API webhook updates and dispatches commands.
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var update tgbotapi.Update
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
	if msg.From == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	h.logger.Info("received message",
		zap.Int64("chat_id", msg.Chat.ID),
		zap.Int64("from_id", msg.From.ID),
	)

	ctx := r.Context()
	reply := h.dispatch(ctx, msg)

	respondJSON(w, msg.Chat.ID, reply)
}

// dispatch routes the message to the appropriate command handler.
func (h *Handler) dispatch(ctx context.Context, msg *tgbotapi.Message) string {
	text := strings.TrimSpace(msg.Text)
	telegramID := msg.From.ID
	username := msg.From.UserName

	switch {
	case strings.HasPrefix(text, "/start"):
		return "Welcome to Slink! Send me a URL and I'll shorten it.\n\nCommands:\n/list — your 5 most recent links\n/stats <slug> — click count\n/account — link your Telegram account"

	case strings.HasPrefix(text, "/stats"):
		parts := strings.Fields(text)
		if len(parts) < 2 {
			return "Usage: /stats <slug>"
		}
		return h.svc.Stats(ctx, telegramID, parts[1])

	case strings.HasPrefix(text, "/list"):
		return h.svc.List(ctx, telegramID)

	case strings.HasPrefix(text, "/account connect "):
		return h.handleAccountConnect(ctx, text, telegramID, username)

	case strings.HasPrefix(text, "/account"):
		return h.svc.AccountInfo(ctx, telegramID)

	case isURL(text):
		return h.svc.Shorten(ctx, telegramID, text)

	default:
		return "Send me a URL to shorten, or use /start to see available commands."
	}
}

func (h *Handler) handleAccountConnect(ctx context.Context, text string, telegramID int64, username string) string {
	// Expected format: /account connect <email> <password>
	parts := strings.Fields(text)
	if len(parts) < 4 {
		return "Usage: /account connect <email> <password>"
	}
	email := parts[2]
	password := parts[3]
	return h.svc.AccountConnect(ctx, telegramID, username, email, password)
}

func isURL(s string) bool {
	u, err := url.ParseRequestURI(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

func respondJSON(w http.ResponseWriter, chatID int64, text string) {
	resp := tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID: chatID,
		},
		Text: text,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"method":  "sendMessage",
		"chat_id": resp.ChatID,
		"text":    resp.Text,
	})
}
