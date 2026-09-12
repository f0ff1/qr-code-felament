package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"filamenttracker/internal/config"
	httpdelivery "filamenttracker/internal/delivery/http"
)

func main() {
	cfg := config.Load()
	mux := httpdelivery.NewRouter()
	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("API listening on %s", server.Addr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server failed: %v\n", err)
		os.Exit(1)
	}
}
