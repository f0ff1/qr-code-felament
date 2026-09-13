package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

func Open(dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("empty database DSN")
	}
	dsn = normalizeDSN(dsn)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(64)
	db.SetMaxIdleConns(16)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	stats := db.Stats()
	log.Printf("postgres connected host=%s max_open=%d open=%d idle=%d", redactedHost(dsn), stats.MaxOpenConnections, stats.OpenConnections, stats.Idle)
	return db, nil
}

func normalizeDSN(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Scheme == "" {
		return dsn
	}
	query := parsed.Query()
	if query.Get("sslmode") == "" {
		host := parsed.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "postgres" {
			query.Set("sslmode", "disable")
		} else {
			query.Set("sslmode", "require")
		}
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return dsn
}

func redactedHost(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Host == "" {
		return "(unparsed)"
	}
	return parsed.Host
}

func sqlDebug() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("SQL_DEBUG")))
	return v == "1" || v == "true" || v == "yes"
}
