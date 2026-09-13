package postgres

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ApplyMigrations runs versioned *.sql files from dir (alphabetical order).
// Versions are tracked in schema_migrations. Subfolders (e.g. rollback/) are ignored.
func ApplyMigrations(db *sql.DB, dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = "db/migrations"
	}
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("migrations dir %q: %w", dir, err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".sql") {
			continue
		}
		files = append(files, name)
	}
	sort.Strings(files)

	applied := 0
	for _, name := range files {
		ok, err := migrationApplied(db, name)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if sqlDebug() {
			log.Printf("sql migrate apply: %s", name)
		}
		stmts := splitSQLStatements(string(body))
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin %s: %w", name, err)
		}
		for _, stmt := range stmts {
			if _, err := tx.Exec(stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply %s: %w\nstatement: %s", name, err, truncate(stmt, 180))
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit %s: %w", name, err)
		}
		applied++
		log.Printf("migration applied: %s", name)
	}

	if applied == 0 {
		log.Printf("migrations up to date (%d files checked)", len(files))
	} else {
		log.Printf("migrations complete: applied %d of %d files", applied, len(files))
	}
	return nil
}

func migrationApplied(db *sql.DB, version string) (bool, error) {
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return exists, nil
}

func splitSQLStatements(body string) []string {
	lines := strings.Split(body, "\n")
	var cleaned []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "--") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	joined := strings.Join(cleaned, "\n")
	parts := strings.Split(joined, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		stmt := strings.TrimSpace(part)
		if stmt == "" {
			continue
		}
		upper := strings.ToUpper(stmt)
		if upper == "BEGIN" || upper == "COMMIT" || upper == "ROLLBACK" {
			continue
		}
		out = append(out, stmt)
	}
	return out
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
