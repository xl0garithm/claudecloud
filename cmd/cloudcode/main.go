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

	// Service + Router
	svc := service.NewInstanceService(db, prov)
	router := api.NewRouter(cfg, svc)

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalf("shutdown error: %v", err)
	}
	logger.Println("server stopped")
}
