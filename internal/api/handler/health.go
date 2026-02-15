package handler

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/logan/cloudcode/internal/api/response"
)

// Health returns a health check handler that pings the database and reports version.
func Health(db *sql.DB, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		statusCode := http.StatusOK

		if db != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()

			if err := db.PingContext(ctx); err != nil {
				dbStatus = "error"
				statusCode = http.StatusServiceUnavailable
			}
		}

		response.JSON(w, statusCode, map[string]string{
			"status":  dbStatus,
			"db":      dbStatus,
			"version": version,
		})
	}
}
