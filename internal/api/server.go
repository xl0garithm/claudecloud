package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/logan/cloudcode/internal/api/handler"
	"github.com/logan/cloudcode/internal/api/middleware"
	"github.com/logan/cloudcode/internal/config"
	"github.com/logan/cloudcode/internal/service"
)

// NewRouter creates the Chi router with all routes and middleware.
func NewRouter(cfg *config.Config, svc *service.InstanceService) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// Health check (no auth)
	r.Get("/healthz", handler.Health())

	// Instance routes (API key auth)
	ih := handler.NewInstanceHandler(svc)
	r.Route("/instances", func(r chi.Router) {
		r.Use(middleware.APIKeyAuth(cfg.APIKey))
		r.Post("/", ih.Create)
		r.Get("/{id}", ih.Get)
		r.Delete("/{id}", ih.Delete)
		r.Post("/{id}/pause", ih.Pause)
		r.Post("/{id}/wake", ih.Wake)
	})

	return r
}
