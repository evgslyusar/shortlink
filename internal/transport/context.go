package transport

import (
	"context"

	mw "github.com/evgslyusar/shortlink/internal/transport/middleware"
)

// getUserID extracts the user ID from context, set by the auth middleware.
func getUserID(ctx context.Context) string {
	return mw.UserIDFromContext(ctx)
}
