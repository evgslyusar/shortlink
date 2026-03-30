# T-07: Telegram Bot Wiring

**Status:** Todo
**Branch:** `feature/t-07-telegram-bot`
**Deps:** T-02, T-03, T-05

---

## Summary

Switch from raw types to `go-telegram-bot-api/v5`. Replace stub handlers with real service
calls. Add `telegram_accounts` persistence and account linking flow (BR-07).
Wire DB + Redis in slinkbot main.go.

---

## Files to Create

| File | Description |
|------|-------------|
| `internal/repository/telegram_account_postgres.go` | LinkTelegram, FindByTelegramID |
| `internal/service/telegram.go` | Account linking, command orchestration |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/telegram/handler.go` | Inject services, use real bot API, implement commands |
| `internal/telegram/types.go` | May be replaced by bot-api types |
| `cmd/slinkbot/main.go` | Create DB pool, Redis, wire services |

---

## Interfaces

```go
// service/telegram.go — consumer
type TelegramAccountLinker interface {
    LinkTelegram(ctx context.Context, userID string, telegramID int64, username string) error
}
type TelegramAccountByTelegramIDFinder interface {
    FindByTelegramID(ctx context.Context, telegramID int64) (*domain.TelegramAccount, error)
}
```

## Telegram Commands

| Command | Behavior | Auth Required |
|---------|----------|---------------|
| URL message | Create short link, return short URL | No (guest) or linked account |
| `/list` | 5 most recent links | Linked account |
| `/stats <slug>` | Click count for slug | Linked account |
| `/account` | Show account linking instructions | No |
| `/account connect email password` | Verify credentials, create telegram_accounts row | No |
| `/start` | Welcome message + command list | No |

## Account Linking Flow (BR-07)

1. User sends `/account` → bot replies with instructions
2. User sends `/account connect email password`
3. Bot verifies credentials via AuthService.Login
4. On success: create `telegram_accounts` row (user_id, telegram_id, username)
5. On error: reply with error message
6. Duplicate linking: reply with "already linked"

---

## Acceptance Criteria

- [ ] URL message → returns short link
- [ ] `/list` → 5 most recent links (requires linked account)
- [ ] `/stats <slug>` → click count
- [ ] `/account connect email password` → links Telegram to user
- [ ] Unlinked user gets prompt to link account
- [ ] Wrong credentials → error message
