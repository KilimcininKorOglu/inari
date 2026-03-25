package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSQLInit verifies that `inari init` detects SQL from .sql files in sql/ directory.
func TestSQLInit(t *testing.T) {
	dir := t.TempDir()
	sqlDir := filepath.Join(dir, "sql")
	if err := os.MkdirAll(sqlDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sqlDir, "schema.sql"), []byte("CREATE TABLE test (id INTEGER);\n"), 0644); err != nil {
		t.Fatal(err)
	}
	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "SQL")
}

// TestSQLInitFromMigrationMarker verifies detection via migration tool markers.
func TestSQLInitFromMigrationMarker(t *testing.T) {
	dir := t.TempDir()
	sqlDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(sqlDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sqlDir, "001_init.sql"), []byte("CREATE TABLE test (id INTEGER);\n"), 0644); err != nil {
		t.Fatal(err)
	}
	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "SQL")
}

// TestSQLIndex verifies that indexing the sql-simple fixture succeeds.
func TestSQLIndex(t *testing.T) {
	dir := copyFixture(t, "sql-simple")

	runInari(t, dir, "init")
	_, stderr := runInari(t, dir, "index", "--full")

	assertContains(t, stderr, "symbols")

	graphDB := filepath.Join(dir, ".inari", "graph.db")
	info, err := os.Stat(graphDB)
	if err != nil {
		t.Fatalf("graph.db should exist after indexing: %v", err)
	}
	if info.Size() == 0 {
		t.Error("graph.db should not be empty")
	}
}

// TestSQLSketch verifies sketch on the SQL fixture.
func TestSQLSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "sql-simple")

	stdout, _ := runInari(t, dir, "sketch", "payments")
	assertContains(t, stdout, "payments")
}

// TestSQLSketchJsonOutput verifies JSON output on the SQL fixture.
func TestSQLSketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "sql-simple")

	stdout, _ := runInari(t, dir, "sketch", "payments", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestSQLFindTables verifies that key SQL symbols are indexed.
func TestSQLFindTables(t *testing.T) {
	dir := setupIndexedFixture(t, "sql-simple")

	symbols := []string{"payments", "users", "payment_summary"}
	for _, sym := range symbols {
		t.Run(sym, func(t *testing.T) {
			stdout, _ := runInari(t, dir, "find", sym, "--json")
			envelope := parseJSON(t, stdout)
			data, ok := envelope["data"].([]interface{})
			if !ok {
				t.Fatalf("expected data to be an array, got %T", envelope["data"])
			}
			if len(data) == 0 {
				t.Errorf("expected %s to be found in the index", sym)
			}
		})
	}
}

// TestSQLRefs verifies that `inari refs users` finds foreign key references.
func TestSQLRefs(t *testing.T) {
	dir := setupIndexedFixture(t, "sql-simple")

	stdout, _ := runInari(t, dir, "refs", "users")
	assertContains(t, stdout, "schema.sql")
}

// TestSQLMap verifies that `inari map` produces a valid overview.
func TestSQLMap(t *testing.T) {
	dir := setupIndexedFixture(t, "sql-simple")

	stdout, _ := runInari(t, dir, "map")
	assertContains(t, stdout, "payments")
}
