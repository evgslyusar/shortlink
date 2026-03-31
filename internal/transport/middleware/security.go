package middleware

import "net/http"

// Security adds common security headers to all responses.
// CSP is not set here — use APISecurityHeaders or WebSecurityHeaders for route-specific policies.
func Security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// APISecurityHeaders adds a strict Content-Security-Policy for API-only routes.
func APISecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		next.ServeHTTP(w, r)
	})
}
