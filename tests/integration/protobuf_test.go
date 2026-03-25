package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProtobufInit verifies that `inari init` detects Protocol Buffers from buf.yaml.
func TestProtobufInit(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "proto"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "buf.yaml"), []byte("version: v1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proto", "test.proto"), []byte("syntax = \"proto3\";\nmessage Test {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "Protocol Buffers")
}

// TestProtobufInitFromFiles verifies that `inari init` detects protobuf from .proto files alone.
func TestProtobufInitFromFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "proto"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proto", "service.proto"), []byte("syntax = \"proto3\";\nservice Svc {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	stdout, _ := runInari(t, dir, "init")
	assertContains(t, stdout, "Protocol Buffers")
}

// TestProtobufIndex verifies that indexing the protobuf-simple fixture succeeds.
func TestProtobufIndex(t *testing.T) {
	dir := copyFixture(t, "protobuf-simple")

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

// TestProtobufSketch verifies sketch on the protobuf fixture.
func TestProtobufSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "protobuf-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentRequest")
	assertContains(t, stdout, "PaymentRequest")
}

// TestProtobufSketchJsonOutput verifies JSON output on the protobuf fixture.
func TestProtobufSketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "protobuf-simple")

	stdout, _ := runInari(t, dir, "sketch", "PaymentRequest", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestProtobufFindMessages verifies that key protobuf symbols are indexed.
func TestProtobufFindMessages(t *testing.T) {
	dir := setupIndexedFixture(t, "protobuf-simple")

	symbols := []string{"PaymentRequest", "PaymentResponse", "PaymentService"}
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

// TestProtobufRefs verifies that `inari refs PaymentRequest` finds references.
func TestProtobufRefs(t *testing.T) {
	dir := setupIndexedFixture(t, "protobuf-simple")

	stdout, _ := runInari(t, dir, "refs", "PaymentRequest")
	assertContains(t, stdout, "controller.proto")
}

// TestProtobufMap verifies that `inari map` produces a valid overview.
func TestProtobufMap(t *testing.T) {
	dir := setupIndexedFixture(t, "protobuf-simple")

	stdout, _ := runInari(t, dir, "map")
	assertContains(t, stdout, "PaymentRequest")
}

// TestProtobufEnumFields verifies that enum values are indexed as constants.
func TestProtobufEnumFields(t *testing.T) {
	dir := setupIndexedFixture(t, "protobuf-simple")

	stdout, _ := runInari(t, dir, "find", "PAYMENT_STATUS_PENDING", "--json")
	envelope := parseJSON(t, stdout)
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", envelope["data"])
	}
	if len(data) == 0 {
		t.Errorf("expected PAYMENT_STATUS_PENDING to be found as const")
	}
}
