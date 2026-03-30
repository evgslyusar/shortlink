package transport

import (
	"encoding/json"
	"net/http"
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

func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
