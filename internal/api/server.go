package api

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/logan/cloudcode/internal/api/handler"
	"github.com/logan/cloudcode/internal/api/middleware"
	"github.com/logan/cloudcode/internal/config"
	"github.com/logan/cloudcode/internal/service"
)

// Services bundles all service dependencies for the router.
type Services struct {
	Instance *service.InstanceService
	Auth     *service.AuthService
	Billing  *service.BillingService // nil if Stripe not configured
	DB      *sql.DB
	Version string
	Logger  *slog.Logger
}

// NewRouter creates the Chi router with all routes and middleware.
func NewRouter(cfg *config.Config, svcs *Services) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.OTELHttp("cloudcode"))
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Security(cfg.BaseURL))
	r.Use(middleware.BodyLimit(1 << 20)) // 1MB
	if svcs.Logger != nil {
		r.Use(middleware.TraceLog(svcs.Logger))
	}

	if cfg.FrontendURL != "" {
		r.Use(middleware.CORS(cfg.FrontendURL))
	}

	// Health check (no auth)
	r.Get("/healthz", handler.Health(svcs.DB, svcs.Version))

	// Prometheus metrics (no auth)
	r.Handle("/metrics", promhttp.Handler())

	// Connect script (no user auth — supports Bearer, cookie, or ?user_id)
	ch := handler.NewConnectHandler(svcs.Instance, cfg.JWTSecret)
	r.Get("/connect.sh", ch.ServeScript)

	// Install script (no auth)
	ih := handler.NewInstallHandler(cfg.BaseURL)
	r.Get("/install.sh", ih.ServeScript)

	// Auth routes (no auth required) — strict rate limit
	ah := handler.NewAuthHandler(svcs.Auth, cfg.FrontendURL, cfg.Environment != "production")
	r.Group(func(r chi.Router) {
		r.Use(middleware.RateLimit(5.0/60.0, 5)) // 5 req/min
		r.Post("/auth/login", ah.Login)
	})
	r.Get("/auth/verify", ah.Verify)

	// Billing webhook (no user auth — verified by Stripe signature)
	var bh *handler.BillingHandler
	if svcs.Billing != nil {
		bh = handler.NewBillingHandler(svcs.Billing)
		r.Post("/billing/webhook", bh.Webhook)
	}

	// Proxy handler for instance terminal/chat/files
	proxyH := handler.NewProxyHandler(svcs.Instance, cfg.JWTSecret)

	// Authenticated routes (dual-mode: JWT + API key)
	r.Group(func(r chi.Router) {
		r.Use(middleware.UserAuth(cfg.JWTSecret, cfg.APIKey))
		r.Use(middleware.RateLimit(1, 60)) // 60 req/min burst

		// Auth (me)
		r.Get("/auth/me", ah.Me)

		// Instance routes
		instH := handler.NewInstanceHandler(svcs.Instance)
		r.Route("/instances", func(r chi.Router) {
			r.Post("/", instH.Create)
			r.Get("/mine", handler.GetMine(svcs.Instance))
			r.Get("/{id}", instH.Get)
			r.Delete("/{id}", instH.Delete)
			r.Post("/{id}/pause", instH.Pause)
			r.Post("/{id}/wake", instH.Wake)

			// Proxy routes to instance services
			r.Get("/{id}/terminal", proxyH.Terminal)
			r.Get("/{id}/chat", proxyH.Chat)
			r.Get("/{id}/files", proxyH.Files)
			r.Get("/{id}/files/read", proxyH.FilesRead)
			r.Get("/{id}/projects", proxyH.Projects)
			r.Post("/{id}/projects/clone", proxyH.ProjectsClone)
		})

		// Billing routes (authed)
		if bh != nil {
			r.Post("/billing/checkout", bh.CreateCheckout)
			r.Get("/billing/portal", bh.GetPortal)
			r.Get("/billing/usage", bh.GetUsage)
		}
	})

	return r
}
