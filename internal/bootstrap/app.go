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
	cfg := config.Load()
	runtime := &Runtime{}

	if cfg.DatabaseURL != "" {
		db, err := postgresrepo.Open(cfg.DatabaseURL)
		if err != nil {
			log.Printf("postgres unavailable, using in-memory fallback: %v", err)
			return runtime
		}
		if err := postgresrepo.Migrate(db); err != nil {
			_ = db.Close()
			log.Printf("postgres migration failed, using in-memory fallback: %v", err)
			return runtime
		}
		runtime.DB = db
		log.Println("postgres initialized")
	}

	if cfg.RedisAddr != "" {
		client, err := cache.NewClient(cfg.RedisAddr)
		if err != nil {
			log.Printf("redis unavailable; continuing in-memory mode: %v", err)
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
