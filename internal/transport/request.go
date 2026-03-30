package transport

import (
	"encoding/json"
	"net/http"

	"github.com/evgslyusar/shortlink/internal/domain"
	mw "github.com/evgslyusar/shortlink/internal/transport/middleware"
)

func decodeJSON[T any](r *http.Request) (T, error) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, &domain.ValidationError{
			Field:   "body",
			Message: "invalid JSON",
		}
	}
	return v, nil
}

func getRequestID(r *http.Request) string {
	return mw.GetRequestID(r.Context())
}
