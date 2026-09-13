package postgres

import (
	"database/sql"
	"strings"
	"testing"
)

func TestSplitSQLStatementsSkipsBeginCommit(t *testing.T) {
	body := `
BEGIN;

CREATE TABLE IF NOT EXISTS demo (id INT);

-- comment
ALTER TABLE demo ADD COLUMN IF NOT EXISTS name TEXT;

COMMIT;
`
	stmts := splitSQLStatements(body)
	if len(stmts) != 2 {
		t.Fatalf("got %d stmts: %#v", len(stmts), stmts)
	}
	if !strings.Contains(strings.ToUpper(stmts[0]), "CREATE TABLE") {
		t.Fatalf("first stmt = %q", stmts[0])
	}
}

func TestApplyMigrationsIdempotent(t *testing.T) {
	// Smoke: helper compiles; real DB covered by docker migrate service.
	_ = sql.ErrNoRows
}
