package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFreshMigrationsAndIntegrity(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "blog.sqlite3")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Integrity(ctx, db); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&n); err != nil || n < 1 {
		t.Fatalf("migrations=%d err=%v", n, err)
	}
}
