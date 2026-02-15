package middleware

import (
	"net/http"

	"github.com/logan/cloudcode/internal/api/response"
)

// APIKeyAuth returns middleware that validates the X-API-Key header against the expected key.
func APIKeyAuth(expectedKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				response.Error(w, http.StatusUnauthorized, "missing API key")
				return
			}
			if key != expectedKey {
				response.Error(w, http.StatusUnauthorized, "invalid API key")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
