package main

import (
	"context"
	"log-parser/internal/config"
	"log-parser/internal/httpserver"
	"log-parser/internal/storage"

	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	var store *storage.Store
	if cfg.DatabaseURL != "" {
		var err error
		store, err = storage.New(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("connect database: %v", err)
		}
		defer store.Close()

		if err := store.RunMigrations(ctx, cfg.MigrationsDir); err != nil {
			log.Fatalf("run migrations: %v", err)
		}
	} else {
		log.Println("DATABASE_URL is empty, parsed logs will not be saved")
	}

	mux := http.NewServeMux()

	httpserver.Register(mux, cfg.DataDir, store)

	server := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Println("server started on :" + cfg.AppPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen and serve: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown app: %v", err)
	}
}
