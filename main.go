package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"filamenttracker/internal/bootstrap"
	"filamenttracker/internal/config"
	httpdelivery "filamenttracker/internal/delivery/http"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewApp(cfg)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			log.Printf("app close: %v", err)
		}
	}()
	app.Start(ctx)

	handler := httpdelivery.NewServer(app)
	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("api server failed: %v", err)
			stop()
		}
	}()

	log.Printf("filament-tracker started on %s (env=%s)", addr, cfg.AppEnv)
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "graceful shutdown failed: %v\n", err)
	}
	log.Println("filament-tracker stopped")
}
