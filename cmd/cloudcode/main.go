package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/logan/cloudcode/internal/api"
	"github.com/logan/cloudcode/internal/config"
	"github.com/logan/cloudcode/internal/ent"
	"github.com/logan/cloudcode/internal/ent/migrate"
	"github.com/logan/cloudcode/internal/netbird"
	"github.com/logan/cloudcode/internal/provider/factory"
	"github.com/logan/cloudcode/internal/service"
)

// version is set by -ldflags at build time.
var version = "dev"

func main() {
	cfg := config.Load()

	// Validate config (fatal in production)
	if err := cfg.Validate(); err != nil {
		// Use a temporary logger for validation errors
		slog.Error("configuration invalid", "error", err)
		os.Exit(1)
	}

	// Structured logger: JSON in production, text in development
	var handler slog.Handler
	if cfg.Environment == "production" {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		handler = slog.NewTextHandler(os.Stdout, nil)
	}
	logger := slog.New(handler)

	// Database
	sqlDB, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	db, err := ent.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Run auto-migration
	if err := db.Schema.Create(context.Background(), migrate.WithDropIndex(true), migrate.WithDropColumn(true)); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("database migrations applied")

	// Provider
	prov, err := factory.NewProvisioner(cfg)
	if err != nil {
		logger.Error("failed to create provisioner", "error", err)
		os.Exit(1)
	}
	logger.Info("provider initialized", "provider", cfg.Provider)

	// Service layer
	instanceSvc := service.NewInstanceService(db, prov, cfg.AnthropicAPIKey)

	// Netbird (Hetzner only)
	var cronSvc *service.CronService
	if cfg.Provider == "hetzner" && cfg.NetbirdAPIToken != "" {
		nbClient := netbird.New(cfg.NetbirdAPIURL, cfg.NetbirdAPIToken)
		nbSvc := service.NewNetbirdService(nbClient, logger)
		instanceSvc.SetNetbirdService(nbSvc)
		logger.Info("netbird enabled")

		// Cron for expired key cleanup
		cronInterval := 30 * time.Minute
		cronSvc = service.NewCronService(nbSvc, logger, cronInterval)
		cronSvc.Start()
	}

	// Mailer
	var mailer service.Mailer
	if cfg.SMTPHost != "" {
		mailer = service.NewSMTPMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
		logger.Info("mailer initialized", "type", "smtp")
	} else {
		mailer = service.NewLogMailer(logger)
		logger.Info("mailer initialized", "type", "log")
	}

	// Auth service
	authSvc := service.NewAuthService(db, cfg.JWTSecret, cfg.BaseURL, cfg.FrontendURL, mailer)

	// Billing service (only if Stripe is configured)
	var billingSvc *service.BillingService
	if cfg.StripeSecretKey != "" {
		billingSvc = service.NewBillingService(
			db, instanceSvc,
			cfg.StripeSecretKey, cfg.StripeWebhookSecret,
			cfg.StripePriceStarter, cfg.StripePricePro,
			cfg.FrontendURL, logger,
		)
		logger.Info("billing enabled", "provider", "stripe")
	} else {
		logger.Info("billing disabled", "reason", "no STRIPE_SECRET_KEY")
	}

	// Activity service
	activityInterval, err := time.ParseDuration(cfg.ActivityCheckInterval)
	if err != nil {
		activityInterval = 5 * time.Minute
	}
	idleThreshold, err := time.ParseDuration(cfg.IdleThreshold)
	if err != nil {
		idleThreshold = 2 * time.Hour
	}
	actSvc := service.NewActivityService(db, prov, logger, activityInterval, idleThreshold)

	// Usage tracker hooks into activity checks
	usageTracker := service.NewUsageTracker(db, activityInterval, logger)
	actSvc.SetOnActive(usageTracker.RecordActive)

	actSvc.Start()

	// Router
	svcs := &api.Services{
		Instance: instanceSvc,
		Auth:     authSvc,
		Billing:  billingSvc,
		DB:       sqlDB,
		Version:  version,
	}
	router := api.NewRouter(cfg, svcs)

	// HTTP Server
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	logger.Info("shutting down")

	actSvc.Stop()
	if cronSvc != nil {
		cronSvc.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}
