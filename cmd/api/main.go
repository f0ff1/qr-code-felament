package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	httpdelivery "filamenttracker/internal/delivery/http"
)

func main() {
	mux := httpdelivery.NewRouter()
	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("API listening on %s", server.Addr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server failed: %v\n", err)
		os.Exit(1)
	}
}
