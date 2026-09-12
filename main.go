package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"filamenttracker/internal/config"
	httpdelivery "filamenttracker/internal/delivery/http"
	"filamenttracker/internal/worker"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	scheduler := worker.NewScheduler()
	if err := scheduler.Run(ctx); err != nil {
		log.Fatalf("worker failed: %v", err)
	}

	mux := httpdelivery.NewRouter()
	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("api server failed: %v", err)
			stop()
		}
	}()

	log.Printf("filament-tracker started on %s", addr)
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "graceful shutdown failed: %v\n", err)
	}
	log.Println("filament-tracker stopped")
}
