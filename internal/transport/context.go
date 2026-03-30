package transport

import "context"

type ctxKey string

const ctxKeyUserID ctxKey = "user_id"

// getUserID extracts the user ID from context. Returns empty string if not set.
// The auth middleware (T-06) will set this value.
func getUserID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyUserID).(string)
	return id
}
