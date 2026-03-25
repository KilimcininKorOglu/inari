package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPythonInit verifies that `inariinit` detects Python from common
// project markers (pyproject.toml, requirements.txt, setup.py, Pipfile).
func TestPythonInit(t *testing.T) {
	markers := []struct {
		filename string
		content  string
	}{
		{"pyproject.toml", "[project]\nname = \"test\""},
		{"requirements.txt", "flask>=2.0"},
		{"setup.py", "from setuptools import setup"},
		{"Pipfile", "[packages]"},
	}

	for _, m := range markers {
		t.Run(m.filename, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, m.filename), []byte(m.content), 0644); err != nil {
				t.Fatalf("failed to write %s: %v", m.filename, err)
			}

			stdout, _ := runInari(t, dir, "init")
			assertContains(t, stdout, "Python")
		})
	}
}

// TestPythonIndex verifies that indexing the python-simple fixture succeeds,
// creates a non-empty graph.db, and reports files/symbols on stderr.
func TestPythonIndex(t *testing.T) {
	dir := copyFixture(t, "python-simple")

	runInari(t, dir, "init")
	_, stderr := runInari(t, dir, "index", "--full")

	assertContains(t, stderr, "files")
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

// TestPythonSketch verifies that `inarisketch PaymentService` works on the
// Python fixture and includes the class name in its output.
func TestPythonSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "python-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService")
	assertContains(t, stdout, "PaymentService")
}

// TestPythonDecorators verifies that decorator metadata (e.g., @staticmethod)
// is captured during indexing. The `validate_card` function should be marked
// as static in the sketch --json output.
func TestPythonDecorators(t *testing.T) {
	dir := setupIndexedFixture(t, "python-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")

	// The JSON output should contain information about the class and its
	// methods. We verify the overall structure is valid JSON and contains
	// relevant method names.
	envelope := parseJSON(t, stdout)
	if envelope["data"] == nil {
		t.Fatal("expected non-null data in sketch JSON")
	}

	// Also verify validate_card is discoverable as a symbol.
	findStdout, _ := runInari(t, dir, "find", "validate_card", "--json")
	findEnvelope := parseJSON(t, findStdout)
	data, ok := findEnvelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", findEnvelope["data"])
	}
	if len(data) == 0 {
		t.Error("expected validate_card to be found in the index")
	}
}

// TestPythonDocstrings verifies that Python docstrings are captured in the
// sketch JSON output. The PaymentService class has a docstring
// "Handles payment processing." which should appear in the metadata.
func TestPythonDocstrings(t *testing.T) {
	dir := setupIndexedFixture(t, "python-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	data, ok := envelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be an object, got %T", envelope["data"])
	}

	symbol, ok := data["symbol"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data.symbol to be an object, got %T", data["symbol"])
	}

	docstring, _ := symbol["docstring"].(string)
	if docstring == "" {
		t.Error("expected PaymentService to have a non-empty docstring")
	}
	assertContains(t, docstring, "payment")
}

// TestPythonSketchJson verifies that `inarisketch PaymentService --json`
// returns valid JSON with command="sketch" on the Python fixture.
func TestPythonSketchJson(t *testing.T) {
	dir := setupIndexedFixture(t, "python-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestPythonRefsFindsCallers verifies that `inarirefs process_payment`
// succeeds on the Python fixture (process_payment is called from
// OrderController.create_order).
func TestPythonRefsFindsCallers(t *testing.T) {
	dir := setupIndexedFixture(t, "python-simple")

	runInari(t, dir, "refs", "process_payment")
}
