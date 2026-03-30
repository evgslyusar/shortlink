package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	mw "github.com/evgslyusar/shortlink/internal/transport/middleware"
)

const maxBodySize = 1 << 20 // 1 MB

// errMalformedJSON is returned when the request body is not valid JSON.
var errMalformedJSON = errors.New("malformed JSON in request body")

func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var v T
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, errMalformedJSON
	}
	return v, nil
}

func getRequestID(r *http.Request) string {
	return mw.GetRequestID(r.Context())
}
