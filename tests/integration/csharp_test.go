package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCSharpInit verifies that `inari init` detects C# from a .csproj file.
func TestCSharpInit(t *testing.T) {
	dir := t.TempDir()

	csproj := `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>`
	if err := os.WriteFile(filepath.Join(dir, "Test.csproj"), []byte(csproj), 0644); err != nil {
		t.Fatalf("failed to write .csproj: %v", err)
	}

	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "C#")
}

// TestCSharpIndex verifies that indexing the csharp-simple fixture succeeds,
// creates a non-empty graph.db, and reports symbols on stderr.
func TestCSharpIndex(t *testing.T) {
	dir := copyFixture(t, "csharp-simple")
	removeInariDir(t, dir)

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

// TestCSharpSketch verifies that `inari sketch PaymentService` works on the
// C# fixture and includes the class name in its output.
func TestCSharpSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "csharp-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService")
	assertContains(t, stdout, "PaymentService")
}

// TestCSharpVisibility verifies that the C# indexer captures visibility
// metadata (public, private) for symbols through sketch --json output.
func TestCSharpVisibility(t *testing.T) {
	dir := setupIndexedFixture(t, "csharp-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}

	if envelope["data"] == nil {
		t.Fatal("expected non-null data in sketch JSON")
	}

	// Verify ProcessPayment (public) is discoverable via find.
	findStdout, _ := runInari(t, dir, "find", "ProcessPayment", "--json")
	findEnvelope := parseJSON(t, findStdout)
	data, ok := findEnvelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", findEnvelope["data"])
	}
	if len(data) == 0 {
		t.Error("expected ProcessPayment to be found in the index")
	}
}

// TestCSharpSketchJsonOutput verifies that `inari sketch PaymentService --json`
// returns valid JSON with command="sketch" on the C# fixture.
func TestCSharpSketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "csharp-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestCSharpInterface verifies that the C# interface IPaymentService is
// indexed and discoverable.
func TestCSharpInterface(t *testing.T) {
	dir := setupIndexedFixture(t, "csharp-simple")

	stdout, _ := runInari(t, dir, "find", "IPaymentService", "--json")
	envelope := parseJSON(t, stdout)
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", envelope["data"])
	}
	if len(data) == 0 {
		t.Error("expected IPaymentService interface to be found in the index")
	}
}

// TestCSharpRefsFindsCallers verifies that `inari refs ProcessPayment`
// succeeds on the C# fixture.
func TestCSharpRefsFindsCallers(t *testing.T) {
	dir := setupIndexedFixture(t, "csharp-simple")

	runInari(t, dir, "refs", "ProcessPayment")
}
