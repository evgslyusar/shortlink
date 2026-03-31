package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS returns middleware that handles Cross-Origin Resource Sharing.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
