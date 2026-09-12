package main

import (
	"fmt"
	"log"
	"os"

	"filamenttracker/internal/config"
	postgresrepo "filamenttracker/internal/repository/postgres"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	db, err := postgresrepo.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := postgresrepo.Migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("migrations complete")
}
