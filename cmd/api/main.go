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
	defer func() { _ = app.Close() }()
	app.Start(ctx)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpdelivery.NewServer(app),
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
	}

	go func() {
		log.Printf("API listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server failed: %v\n", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}
