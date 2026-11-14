// Package main is the entrypoint for the barcode-server application.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/accounts"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/api"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/config"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/rbac"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/storage"
)

func main() {
	// 1. Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	store, err := storage.New(storage.Options{SQLitePath: cfg.DBPath, DatabaseURL: cfg.DatabaseURL, EncryptionKey: cfg.AccurateEncryptionKey})
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer store.Close()

	// 4. Start Fiber v3 HTTP server
	permCache := rbac.NewMemoryCache(cfg.RBACCacheTTL)
	srv := api.New(api.Config{
		Store:        store,
		AppConfig:    cfg,
		PermCache:    permCache,
		AccuratePool: accounts.NewPool(store, nil),
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
