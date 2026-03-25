package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPhpInit verifies that `inari init` detects PHP from project marker files.
func TestPhpInit(t *testing.T) {
	markers := []struct {
		filename string
		content  string
	}{
		{"composer.json", `{"name":"test/app"}`},
		{"artisan", "#!/usr/bin/env php\n"},
	}
	for _, m := range markers {
		t.Run(m.filename, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, m.filename), []byte(m.content), 0644); err != nil {
				t.Fatalf("failed to write %s: %v", m.filename, err)
			}
			stdout, _ := runInari(t, dir, "init")
			assertContains(t, stdout, "PHP")
		})
	}
}

// TestPhpIndex verifies that indexing the php-simple fixture succeeds
// and creates a non-empty graph.db.
func TestPhpIndex(t *testing.T) {
	dir := copyFixture(t, "php-simple")

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

// TestPhpSketch verifies that `inari sketch PaymentService` works on the
// PHP fixture and includes the class name in its output.
func TestPhpSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "php-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService")
	assertContains(t, stdout, "PaymentService")
	assertContains(t, stdout, "class")
}

// TestPhpSketchJsonOutput verifies that `inari sketch PaymentService --json`
// returns valid JSON with command="sketch" on the PHP fixture.
func TestPhpSketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "php-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestPhpFindClasses verifies that key PHP classes are indexed and
// discoverable via `inari find`.
func TestPhpFindClasses(t *testing.T) {
	dir := setupIndexedFixture(t, "php-simple")

	symbols := []string{"PaymentService", "OrderController", "Logger"}
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

// TestPhpRefs verifies that `inari refs processPayment` finds callers
// in the PHP fixture.
func TestPhpRefs(t *testing.T) {
	dir := setupIndexedFixture(t, "php-simple")

	stdout, _ := runInari(t, dir, "refs", "processPayment")
	assertContains(t, stdout, "OrderController.php")
}

// TestPhpMap verifies that `inari map` produces a valid overview for
// the PHP fixture.
func TestPhpMap(t *testing.T) {
	dir := setupIndexedFixture(t, "php-simple")

	stdout, _ := runInari(t, dir, "map")
	assertContains(t, stdout, "php")
	assertContains(t, stdout, "PaymentService")
}

// TestPhpDeps verifies that `inari deps PaymentService` returns
// expected dependencies.
func TestPhpDeps(t *testing.T) {
	dir := setupIndexedFixture(t, "php-simple")

	stdout, _ := runInari(t, dir, "deps", "PaymentService")
	assertContains(t, stdout, "Logger")
}

// TestPhpNamespaces verifies that PHP namespaces are indexed as modules.
func TestPhpNamespaces(t *testing.T) {
	dir := setupIndexedFixture(t, "php-simple")

	stdout, _ := runInari(t, dir, "find", "App\\Payments", "--json")
	envelope := parseJSON(t, stdout)
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", envelope["data"])
	}
	if len(data) == 0 {
		t.Errorf("expected App\\Payments namespace to be found in the index")
	}
}
