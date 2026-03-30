package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

const headerXRequestID = "X-Request-ID"

// Correlation injects a unique request ID (UUID v4) into the request context
// and sets the X-Request-ID response header. If the incoming request already
// carries X-Request-ID, that value is reused.
func Correlation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(headerXRequestID)
		if id == "" {
			id = uuid.NewString()
		}

		w.Header().Set(headerXRequestID, id)

		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from context. Returns empty string if
// no request ID is set.
func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
