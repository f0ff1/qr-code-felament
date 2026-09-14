package bootstrap

import (
	"database/sql"
	"fmt"
	"log"

	"filamenttracker/internal/config"
	"filamenttracker/internal/infrastructure/cache"
	postgresrepo "filamenttracker/internal/repository/postgres"
)

type Runtime struct {
	DB    *sql.DB
	Redis *cache.Client
}

func NewRuntime() *Runtime {
	return NewRuntimeFromSettings(config.Load())
}

func NewRuntimeFromSettings(cfg config.Settings) *Runtime {
	runtime := &Runtime{}

	if cfg.DatabaseURL == "" {
		log.Println("DATABASE_URL is empty; using in-memory repositories (data will not persist)")
	} else {
		db, err := postgresrepo.Open(cfg.DatabaseURL)
		if err != nil {
			if cfg.RequirePostgres {
				log.Fatalf("postgres unavailable: %v", err)
			}
			log.Printf("postgres unavailable, using in-memory fallback: %v", err)
		} else {
			runtime.DB = db
			log.Println("postgres initialized (schema must be applied via migrations / docker compose)")
		}
	}

	if cfg.RedisAddr != "" {
		client, err := cache.NewClientWithOptions(cache.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
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
