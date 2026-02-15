package handler

import (
	"net/http"

	"github.com/logan/cloudcode/internal/api/response"
)

// Health returns a health check handler.
func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
