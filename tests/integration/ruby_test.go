package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRubyInit verifies that `inari init` detects Ruby from project marker files.
func TestRubyInit(t *testing.T) {
	markers := []struct {
		filename string
		content  string
	}{
		{"Gemfile", `source "https://rubygems.org"`},
		{"Rakefile", "task :default"},
		{"config.ru", "run MyApp"},
	}
	for _, m := range markers {
		t.Run(m.filename, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, m.filename), []byte(m.content), 0644); err != nil {
				t.Fatalf("failed to write %s: %v", m.filename, err)
			}
			stdout, _ := runInari(t, dir, "init")
			assertContains(t, stdout, "Ruby")
		})
	}
}

// TestRubyIndex verifies that indexing the ruby-simple fixture succeeds
// and creates a non-empty graph.db.
func TestRubyIndex(t *testing.T) {
	dir := copyFixture(t, "ruby-simple")

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

// TestRubySketch verifies that `inari sketch PaymentService` works on the
// Ruby fixture and includes the class name in its output.
func TestRubySketch(t *testing.T) {
	dir := setupIndexedFixture(t, "ruby-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService")
	assertContains(t, stdout, "PaymentService")
	assertContains(t, stdout, "class")
}

// TestRubySketchJsonOutput verifies that `inari sketch PaymentService --json`
// returns valid JSON with command="sketch" on the Ruby fixture.
func TestRubySketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "ruby-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestRubyFindClasses verifies that key Ruby classes are indexed and
// discoverable via `inari find`.
func TestRubyFindClasses(t *testing.T) {
	dir := setupIndexedFixture(t, "ruby-simple")

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

// TestRubyRefs verifies that `inari refs process_payment` finds callers
// in the Ruby fixture.
func TestRubyRefs(t *testing.T) {
	dir := setupIndexedFixture(t, "ruby-simple")

	stdout, _ := runInari(t, dir, "refs", "process_payment")
	assertContains(t, stdout, "order_controller.rb")
}

// TestRubyMap verifies that `inari map` produces a valid overview for
// the Ruby fixture.
func TestRubyMap(t *testing.T) {
	dir := setupIndexedFixture(t, "ruby-simple")

	stdout, _ := runInari(t, dir, "map")
	assertContains(t, stdout, "ruby")
	assertContains(t, stdout, "PaymentService")
}

// TestRubyDeps verifies that `inari deps PaymentService` returns
// expected dependencies.
func TestRubyDeps(t *testing.T) {
	dir := setupIndexedFixture(t, "ruby-simple")

	stdout, _ := runInari(t, dir, "deps", "PaymentService")
	assertContains(t, stdout, "Logger")
}

// TestRubyModules verifies that Ruby modules are indexed and findable.
func TestRubyModules(t *testing.T) {
	dir := setupIndexedFixture(t, "ruby-simple")

	stdout, _ := runInari(t, dir, "find", "Payments", "--json")
	envelope := parseJSON(t, stdout)
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", envelope["data"])
	}
	if len(data) == 0 {
		t.Errorf("expected Payments module to be found in the index")
	}
}
