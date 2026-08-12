package data

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWriteSQLiteDSNRequiresExistingDatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.db")
	dsn, err := readWriteSQLiteDSN(path)
	if err != nil {
		t.Fatalf("readWriteSQLiteDSN: %v", err)
	}
	if !strings.Contains(dsn, "mode=rw") {
		t.Fatalf("DSN does not enforce mode=rw: %s", dsn)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err == nil {
		t.Fatal("mode=rw unexpectedly created a missing database")
	}
}
