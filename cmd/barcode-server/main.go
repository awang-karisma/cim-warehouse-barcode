// Package main is the entrypoint for the barcode-server application.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/adapter"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/official"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/provider/web"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/session"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/api"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/config"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
)

func main() {
	// 1. Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// 2. Ensure DB directory exists and initialize SQLite storage
	dbDir := filepath.Dir(cfg.DBPath)
	if dbDir != "" && dbDir != "." {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			log.Fatalf("failed to create database directory %s: %v", dbDir, err)
		}
	}

	store, err := storage.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to initialize database at %s: %v", cfg.DBPath, err)
	}
	defer store.Close()

	// 3. Initialize Accurate integration (if configured)
	var accProv accurate.Provider
	var accSession *session.Manager
	var accRegistry *adapter.Registry

	switch accurate.ProviderKind(cfg.AccurateProvider) {
	case accurate.ProviderWeb:
		accProv = web.New(cfg.AccurateEmail, cfg.AccuratePassword, nil)
		accSession = session.NewManager(store, accProv)
		accRegistry = adapter.NewRegistry(accProv)
		log.Printf("Accurate integration initialized with provider: web (%s)", cfg.AccurateEmail)
	case accurate.ProviderOfficial:
		var opts []official.Option
		if cfg.AccurateAPISignatureSecret != "" {
			opts = append(opts, official.WithSignatureSecret(cfg.AccurateAPISignatureSecret))
		}
		accProv = official.New(cfg.AccurateAPIToken, opts...)
		accSession = session.NewManager(store, accProv)
		accRegistry = adapter.NewRegistry(accProv)
		log.Println("Accurate integration initialized with provider: official")
	default:
		log.Println("Accurate integration is disabled (ACCURATE_PROVIDER is unset)")
	}

	// 4. Start Fiber v3 HTTP server
	srv := api.New(api.Config{
		Store:            store,
		AppConfig:        cfg,
		AccurateProvider: accProv,
		AccurateSession:  accSession,
		AccurateRegistry: accRegistry,
	})

	if embeddedFS != nil {
		log.Println("Serving embedded SolidJS frontend")
		srv.SetupStaticFS(embeddedFS)
	} else {
		distPath := "web/dist"
		if info, err := os.Stat(distPath); err == nil && info.IsDir() {
			log.Printf("Serving SolidJS frontend from %s", distPath)
			srv.SetupStaticFiles(distPath)
		}
	}

	app := srv.App()

	serverErrors := make(chan error, 1)
	addr := ":" + cfg.Port
	go func() {
		log.Printf("Starting HTTP server listening on %s (Scalar Docs: http://localhost:%s/docs, OpenAPI: http://localhost:%s/openapi.json)", addr, cfg.Port, cfg.Port)
		if err := app.Listen(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 5. Handle graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("HTTP server listen error: %v", err)
	case sig := <-quit:
		log.Printf("Received shutdown signal %v, terminating gracefully...", sig)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("warning: server shutdown error: %v", err)
	}
	if err := store.Close(); err != nil {
		log.Printf("warning: storage close error: %v", err)
	}
	log.Println("Server gracefully stopped.")
}
