package bootstrap

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"filamenttracker/internal/config"
	"filamenttracker/internal/infrastructure/cache"
	postgresrepo "filamenttracker/internal/repository/postgres"
)

type Runtime struct {
	DB    *sql.DB
	Redis *cache.Client
}

func allowInMemory() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("ALLOW_INMEMORY")))
	return v == "1" || v == "true" || v == "yes"
}

func requirePostgres() bool {
	if allowInMemory() {
		return false
	}
	if os.Getenv("RAILWAY_ENVIRONMENT") != "" || os.Getenv("RAILWAY_ENVIRONMENT_NAME") != "" {
		return true
	}
	v := strings.ToLower(strings.TrimSpace(os.Getenv("REQUIRE_POSTGRES")))
	return v == "1" || v == "true" || v == "yes"
}

func NewRuntime() *Runtime {
	cfg := config.Load()
	runtime := &Runtime{}

	if cfg.DatabaseURL == "" {
		log.Println("DATABASE_URL is empty; using in-memory repositories (data will not persist)")
	} else {
		db, err := postgresrepo.Open(cfg.DatabaseURL)
		if err != nil {
			if requirePostgres() {
				log.Fatalf("postgres unavailable: %v", err)
			}
			log.Printf("postgres unavailable, using in-memory fallback: %v", err)
		} else if err := postgresrepo.Migrate(db); err != nil {
			_ = db.Close()
			if requirePostgres() {
				log.Fatalf("postgres migration failed: %v", err)
			}
			log.Printf("postgres migration failed, using in-memory fallback: %v", err)
		} else {
			runtime.DB = db
			log.Println("postgres initialized")
		}
	}

	if cfg.RedisAddr != "" {
		client, err := cache.NewClient(cfg.RedisAddr)
		if err != nil {
			log.Printf("redis unavailable; continuing without cache: %v", err)
		} else {
			runtime.Redis = client
			log.Println("redis initialized")
		}
	}

	return runtime
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	var err error
	if r.DB != nil {
		if closeErr := r.DB.Close(); closeErr != nil {
			err = fmt.Errorf("close db: %w", closeErr)
		}
	}
	if r.Redis != nil {
		if closeErr := r.Redis.Close(); closeErr != nil {
			if err == nil {
				err = fmt.Errorf("close redis: %w", closeErr)
			} else {
				err = fmt.Errorf("%v; close redis: %w", err, closeErr)
			}
		}
	}
	return err
}
