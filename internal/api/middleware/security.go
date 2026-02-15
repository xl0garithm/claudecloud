package middleware

import (
	"net/http"
	"strings"
)

// Security returns middleware that sets security headers on all responses.
func Security(baseURL string) func(http.Handler) http.Handler {
	isHTTPS := strings.HasPrefix(baseURL, "https")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("X-XSS-Protection", "0")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=()")

			if isHTTPS {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}
