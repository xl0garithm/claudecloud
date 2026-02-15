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

// Services bundles all service dependencies for the router.
type Services struct {
	Instance *service.InstanceService
	Auth     *service.AuthService
}

// NewRouter creates the Chi router with all routes and middleware.
func NewRouter(cfg *config.Config, svcs *Services) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	if cfg.FrontendURL != "" {
		r.Use(middleware.CORS(cfg.FrontendURL))
	}

	// Health check (no auth)
	r.Get("/healthz", handler.Health())

	// Connect script (no auth — script is fetched by curl)
	ch := handler.NewConnectHandler(svcs.Instance)
	r.Get("/connect.sh", ch.ServeScript)

	// Auth routes (no auth required)
	ah := handler.NewAuthHandler(svcs.Auth, cfg.FrontendURL)
	r.Post("/auth/login", ah.Login)
	r.Get("/auth/verify", ah.Verify)

	// Authenticated routes (dual-mode: JWT + API key)
	r.Group(func(r chi.Router) {
		r.Use(middleware.UserAuth(cfg.JWTSecret, cfg.APIKey))

		// Auth (me)
		r.Get("/auth/me", ah.Me)

		// Instance routes
		ih := handler.NewInstanceHandler(svcs.Instance)
		r.Route("/instances", func(r chi.Router) {
			r.Post("/", ih.Create)
			r.Get("/{id}", ih.Get)
			r.Delete("/{id}", ih.Delete)
			r.Post("/{id}/pause", ih.Pause)
			r.Post("/{id}/wake", ih.Wake)
		})
	})

	return r
}
