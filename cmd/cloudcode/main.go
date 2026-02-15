package main

import (
	"context"
	"log"
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

func main() {
	cfg := config.Load()
	logger := log.New(os.Stdout, "", log.LstdFlags)

	// Database
	db, err := ent.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run auto-migration
	if err := db.Schema.Create(context.Background(), migrate.WithDropIndex(true), migrate.WithDropColumn(true)); err != nil {
		logger.Fatalf("failed to run migrations: %v", err)
	}
	logger.Println("database migrations applied")

	// Provider
	prov, err := factory.NewProvisioner(cfg)
	if err != nil {
		logger.Fatalf("failed to create provisioner: %v", err)
	}
	logger.Printf("provider: %s", cfg.Provider)

	// Service layer
	instanceSvc := service.NewInstanceService(db, prov)

	// Netbird (Hetzner only)
	var cronSvc *service.CronService
	if cfg.Provider == "hetzner" && cfg.NetbirdAPIToken != "" {
		nbClient := netbird.New(cfg.NetbirdAPIURL, cfg.NetbirdAPIToken)
		nbSvc := service.NewNetbirdService(nbClient, logger)
		instanceSvc.SetNetbirdService(nbSvc)
		logger.Println("netbird: enabled")

		// Cron for expired key cleanup
		cronInterval := 30 * time.Minute
		cronSvc = service.NewCronService(nbSvc, logger, cronInterval)
		cronSvc.Start()
	}

	// Mailer
	var mailer service.Mailer
	if cfg.SMTPHost != "" {
		mailer = service.NewSMTPMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
		logger.Println("mailer: smtp")
	} else {
		mailer = service.NewLogMailer(logger)
		logger.Println("mailer: log (dev mode)")
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
		logger.Println("billing: stripe enabled")
	} else {
		logger.Println("billing: disabled (no STRIPE_SECRET_KEY)")
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
	}
	router := api.NewRouter(cfg, svcs)

	// HTTP Server
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Printf("listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
	}()

	<-done
	logger.Println("shutting down...")

	actSvc.Stop()
	if cronSvc != nil {
		cronSvc.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalf("shutdown error: %v", err)
	}
	logger.Println("server stopped")
}
