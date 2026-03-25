package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestJavaInit verifies that `inari init` detects Java from a pom.xml file.
func TestJavaInit(t *testing.T) {
	dir := t.TempDir()

	pom := `<?xml version="1.0" encoding="UTF-8"?>
<project><modelVersion>4.0.0</modelVersion></project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pom), 0644); err != nil {
		t.Fatalf("failed to write pom.xml: %v", err)
	}

	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "Java")
}

// TestJavaInitGradle verifies that `inari init` detects Java from a build.gradle file.
func TestJavaInitGradle(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte("apply plugin: 'java'"), 0644); err != nil {
		t.Fatalf("failed to write build.gradle: %v", err)
	}

	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "Java")
}

// TestJavaIndex verifies that indexing the java-simple fixture succeeds
// and creates a non-empty graph.db.
func TestJavaIndex(t *testing.T) {
	dir := copyFixture(t, "java-simple")

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

// TestJavaSketch verifies that `inari sketch PaymentService` works on the
// Java fixture and includes the class name in its output.
func TestJavaSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "java-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService")
	assertContains(t, stdout, "PaymentService")
	assertContains(t, stdout, "class")
}

// TestJavaSketchJsonOutput verifies that `inari sketch PaymentService --json`
// returns valid JSON with command="sketch" on the Java fixture.
func TestJavaSketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "java-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestJavaFindClasses verifies that key Java classes are indexed and
// discoverable via `inari find`.
func TestJavaFindClasses(t *testing.T) {
	dir := setupIndexedFixture(t, "java-simple")

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

// TestJavaRefs verifies that `inari refs processPayment` finds callers
// in the Java fixture.
func TestJavaRefs(t *testing.T) {
	dir := setupIndexedFixture(t, "java-simple")

	stdout, _ := runInari(t, dir, "refs", "processPayment")
	assertContains(t, stdout, "OrderController")
}

// TestJavaMap verifies that `inari map` produces a valid overview for
// the Java fixture.
func TestJavaMap(t *testing.T) {
	dir := setupIndexedFixture(t, "java-simple")

	stdout, _ := runInari(t, dir, "map")
	assertContains(t, stdout, "java")
	assertContains(t, stdout, "PaymentService")
}

// TestJavaDeps verifies that `inari deps PaymentService` returns the
// expected dependency list.
func TestJavaDeps(t *testing.T) {
	dir := setupIndexedFixture(t, "java-simple")

	stdout, _ := runInari(t, dir, "deps", "PaymentService")
	assertContains(t, stdout, "IPaymentService")
}
