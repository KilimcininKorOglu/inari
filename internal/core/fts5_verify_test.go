package core

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestFTS5Support(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Test FTS5 virtual table creation
	_, err = db.Exec("CREATE VIRTUAL TABLE IF NOT EXISTS test_fts USING fts5(name, body, tokenize = 'porter unicode61')")
	if err != nil {
		t.Fatalf("FTS5 not supported: %v", err)
	}

	// Test FTS5 insert + query
	_, err = db.Exec("INSERT INTO test_fts(name, body) VALUES ('PaymentService', 'class PaymentService handles payment processing')")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	var name string
	err = db.QueryRow("SELECT name FROM test_fts WHERE test_fts MATCH 'payment' ORDER BY rank LIMIT 1").Scan(&name)
	if err != nil {
		t.Fatalf("FTS5 query failed: %v", err)
	}
	if name != "PaymentService" {
		t.Fatalf("expected PaymentService, got %s", name)
	}

	// Test WAL mode
	var mode string
	err = db.QueryRow("PRAGMA journal_mode=WAL").Scan(&mode)
	if err != nil {
		t.Fatalf("WAL: %v", err)
	}
	if mode != "wal" {
		t.Logf("WAL mode returned: %s (expected 'wal' for file-based DBs, memory may differ)", mode)
	}

	// Test recursive CTE
	var cnt int
	err = db.QueryRow("WITH RECURSIVE cnt(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM cnt WHERE x<10) SELECT MAX(x) FROM cnt").Scan(&cnt)
	if err != nil {
		t.Fatalf("recursive CTE: %v", err)
	}
	if cnt != 10 {
		t.Fatalf("expected 10, got %d", cnt)
	}

	// Test BM25 ranking function
	_, err = db.Exec("INSERT INTO test_fts(name, body) VALUES ('Logger', 'simple logger utility')")
	if err != nil {
		t.Fatalf("insert logger: %v", err)
	}

	var score float64
	err = db.QueryRow("SELECT bm25(test_fts, 5.0, 2.0) FROM test_fts WHERE test_fts MATCH 'payment' LIMIT 1").Scan(&score)
	if err != nil {
		t.Fatalf("BM25 not supported: %v", err)
	}
	t.Logf("BM25 score for 'payment': %f", score)
}
