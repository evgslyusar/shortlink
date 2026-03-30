package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/evgslyusar/shortlink/internal/domain"
)

const (
	ErrCodeValidation   = "VALIDATION_ERROR"
	ErrCodeConflict     = "CONFLICT"
	ErrCodeUnauthorized = "UNAUTHORIZED"
	ErrCodeNotFound     = "NOT_FOUND"
	ErrCodeForbidden    = "FORBIDDEN"
	ErrCodeInternal     = "INTERNAL_ERROR"
)

type successResponse struct {
	Data any            `json:"data"`
	Meta map[string]any `json:"meta"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorBody      `json:"error"`
	Meta  map[string]any `json:"meta"`
}

func respondData(w http.ResponseWriter, r *http.Request, statusCode int, data any) {
	resp := successResponse{
		Data: data,
		Meta: map[string]any{
			"request_id": getRequestID(r),
		},
	}
	writeJSON(w, statusCode, resp)
}

func respondError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string) {
	resp := errorResponse{
		Error: errorBody{
			Code:    code,
			Message: message,
		},
		Meta: map[string]any{
			"request_id": getRequestID(r),
		},
	}
	writeJSON(w, statusCode, resp)
}

// mapError maps domain errors to HTTP status code, error code, and message.
func mapError(err error) (status int, code, message string) {
	switch {
	case errors.Is(err, domain.ErrAlreadyExists):
		return http.StatusConflict, ErrCodeConflict, "already exists"
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, ErrCodeNotFound, "not found"
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden, ErrCodeForbidden, "forbidden"
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized, ErrCodeUnauthorized, "invalid credentials"
	default:
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			return http.StatusUnprocessableEntity, ErrCodeValidation, ve.Error()
		}
		return http.StatusInternalServerError, ErrCodeInternal, "internal error"
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	// Encoding to a ResponseWriter can only fail if the type is not serializable
	// (e.g., chan, func), which does not apply to our response structs, or if the
	// connection is already closed, in which case logging is also unreliable.
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
