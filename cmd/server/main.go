package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"beam/internal/server/api"
	"beam/internal/server/config"
	"beam/internal/server/database"
	"beam/internal/server/service"
	"beam/internal/server/storage"
)

func main() {
	// Structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load config
	cfg := config.Load()
	slog.Info("configuration loaded",
		"port", cfg.Port,
		"storage_path", cfg.StoragePath,
		"max_file_size", cfg.MaxFileSize,
		"default_expiry", cfg.DefaultExpiry,
	)

	// Connect to database
	ctx := context.Background()
	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Run migrations
	if err := db.RunMigrations(ctx); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("database migrations complete")

	// Initialize storage
	store := storage.NewFileSystemStore(cfg.StoragePath)
	if err := store.EnsureDir(); err != nil {
		slog.Error("failed to initialize storage", "error", err)
		os.Exit(1)
	}
	slog.Info("file storage initialized", "path", cfg.StoragePath)

	// Initialize repository and service
	repo := database.NewRepository(db)
	svc := service.NewUploadService(repo, store, cfg)

	// Start cleanup service
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	cleanup := storage.NewCleanupService(repo, store, cfg.CleanupInterval)
	cleanup.Start(cleanupCtx)

	// Setup HTTP router
	handler := api.NewHandler(svc, db)
	e := api.SetupRouter(handler, cfg)

	// Start server in a goroutine
	go func() {
		addr := fmt.Sprintf(":%s", cfg.Port)
		slog.Info("starting server", "addr", addr, "base_url", cfg.BaseURL)
		if err := e.Start(addr); err != nil {
			slog.Info("server stopped", "reason", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("shutting down", "signal", sig)

	// Stop accepting new requests, finish in-flight with 30s timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	// Stop cleanup service
	cleanupCancel()
	cleanup.Wait()

	slog.Info("server exited cleanly")
}
