# T-05: Click Tracking & Stats

**Status:** Done (PR #8)
**Branch:** `feature/t-05-click-tracking`
**Deps:** T-04

---

## Summary

Buffered channel (1000) + background worker. Flush to Postgres every 5s or at 100 items.
Channel full → drop + log warning. Stats endpoint `GET /v1/links/{slug}/stats`.

---

## Files to Create

| File | Description |
|------|-------------|
| `internal/repository/click_postgres.go` | `BatchInsert`, `CountByLink`, `CountByDay`, `CountByCountry` |
| `internal/service/click.go` | `ClickTracker` — buffered channel, background worker, flush logic |
| `internal/service/click_test.go` | Tests for recording, flushing, channel-full drop, stats |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/transport/redirect_handler.go` | Record click after resolve (non-blocking) |
| `internal/transport/link_handler.go` | Add Stats endpoint |
| `cmd/slinkapi/main.go` | Wire click repo, click service, start/stop worker, register stats route |

---

## Interfaces

```go
// service/click.go — consumer of persistence
type ClickBatchInserter interface {
    BatchInsert(ctx context.Context, clicks []domain.Click) error
}
type ClickStatsQuerier interface {
    CountByLink(ctx context.Context, linkID string) (int64, error)
    CountByDay(ctx context.Context, linkID string, days int) ([]DayStat, error)
    CountByCountry(ctx context.Context, linkID string) ([]CountryStat, error)
}

// Used by redirect handler
type ClickRecorder interface {
    Record(click domain.Click) // non-blocking send to channel
}
```

## Business Logic

- **Record(click):** non-blocking send to buffered channel (cap 1000). If full → drop + `logger.Warn`
- **Background worker:** loop with `select` on channel and ticker (5s)
  - Accumulate clicks in buffer
  - Flush when buffer reaches 100 items OR ticker fires
  - On context cancellation: drain remaining channel items, final flush
- **BatchInsert:** single `INSERT ... VALUES` with multiple rows
- **Stats:** aggregate queries on clicks table by link_id

## Stats Response

```json
// GET /v1/links/{slug}/stats (200)
{
  "data": {
    "total_clicks": 1234,
    "by_day": [
      { "date": "2026-03-29", "count": 42 },
      { "date": "2026-03-30", "count": 15 }
    ],
    "by_country": [
      { "country": "US", "count": 800 },
      { "country": "DE", "count": 200 }
    ]
  }
}
```

## Click Recording in Redirect

```go
// redirect_handler.go — after successful resolve
click := domain.Click{
    ID:        uuid.NewString(),
    LinkID:    link.ID,  // need link ID from resolve
    ClickedAt: time.Now().UTC(),
    Referer:   r.Header.Get("Referer"),
    UserAgent: r.Header.Get("User-Agent"),
}
h.recorder.Record(click)
// redirect response already sent — non-blocking
```

## Graceful Shutdown

Worker must drain remaining channel items on shutdown:
```go
// After context cancelled:
close(ch)
for click := range ch {
    buffer = append(buffer, click)
}
flush(buffer) // final flush
```

---

## Acceptance Criteria

- [x] Click appears in DB within flush interval after redirect
- [x] Redirect response time unaffected by click recording
- [x] `GET /v1/links/{slug}/stats` returns `{total_clicks, by_day, by_country}`
- [x] Stats for non-owned slug: 403; nonexistent: 404
- [x] Graceful shutdown drains remaining channel items
- [x] Channel full: event dropped, warning logged (not blocked)
