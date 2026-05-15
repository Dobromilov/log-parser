package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log-parser/internal/config"
	"log-parser/internal/httpserver"
	"log-parser/internal/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	configureLogger(cfg.LogLevel)

	var store *storage.Store
	if cfg.DatabaseURL != "" {
		var err error
		store, err = storage.New(ctx, cfg.DatabaseURL)
		if err != nil {
			slog.Error("connect database failed", "error", err)
			os.Exit(1)
		}
		defer store.Close()

		if err := store.RunMigrations(ctx, cfg.MigrationsDir); err != nil {
			slog.Error("run migrations failed", "error", err)
			os.Exit(1)
		}
	} else {
		slog.Warn("database url is empty, parsed logs will not be saved")
	}

	mux := http.NewServeMux()

	if store != nil {
		httpserver.Register(mux, cfg.DataDir, store)
	} else {
		httpserver.Register(mux, cfg.DataDir)
	}

	server := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           httpserver.LoggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("server started", "port", cfg.AppPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen and serve failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown app failed", "error", err)
		os.Exit(1)
	}
}

func configureLogger(level string) {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slogLevel,
	})))
}
