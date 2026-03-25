package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLuaInit verifies that `inari init` detects Lua from project marker files.
func TestLuaInit(t *testing.T) {
	markers := []struct {
		filename string
		content  string
	}{
		{".luarc.json", `{"runtime.version":"Lua 5.4"}`},
		{"init.lua", "-- entry point\n"},
		{"conf.lua", "-- config\n"},
	}
	for _, m := range markers {
		t.Run(m.filename, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, m.filename), []byte(m.content), 0644); err != nil {
				t.Fatalf("failed to write %s: %v", m.filename, err)
			}
			stdout, _ := runInari(t, dir, "init")
			assertContains(t, stdout, "Lua")
		})
	}
}

// TestLuaIndex verifies that indexing the lua-simple fixture succeeds
// and creates a non-empty graph.db.
func TestLuaIndex(t *testing.T) {
	dir := copyFixture(t, "lua-simple")

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

// TestLuaSketch verifies that `inari sketch processPayment` works on the
// Lua fixture.
func TestLuaSketch(t *testing.T) {
	dir := setupIndexedFixture(t, "lua-simple")

	stdout, _ := runInari(t, dir, "sketch", "processPayment")
	assertContains(t, stdout, "processPayment")
	assertContains(t, stdout, "function")
}

// TestLuaSketchJsonOutput verifies that `inari sketch processPayment --json`
// returns valid JSON with command="sketch" on the Lua fixture.
func TestLuaSketchJsonOutput(t *testing.T) {
	dir := setupIndexedFixture(t, "lua-simple")

	stdout, _ := runInari(t, dir, "sketch", "processPayment", "--json")
	envelope := parseJSON(t, stdout)

	cmd, _ := envelope["command"].(string)
	if cmd != "sketch" {
		t.Errorf("expected command=sketch, got %s", cmd)
	}
}

// TestLuaFindFunctions verifies that key Lua functions are indexed and
// discoverable via `inari find`.
func TestLuaFindFunctions(t *testing.T) {
	dir := setupIndexedFixture(t, "lua-simple")

	symbols := []string{"processPayment", "createOrder", "info"}
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

// TestLuaRefs verifies that `inari refs processPayment` finds callers
// in the Lua fixture.
func TestLuaRefs(t *testing.T) {
	dir := setupIndexedFixture(t, "lua-simple")

	stdout, _ := runInari(t, dir, "refs", "processPayment")
	assertContains(t, stdout, "order_controller.lua")
}

// TestLuaMap verifies that `inari map` produces a valid overview for
// the Lua fixture.
func TestLuaMap(t *testing.T) {
	dir := setupIndexedFixture(t, "lua-simple")

	stdout, _ := runInari(t, dir, "map")
	assertContains(t, stdout, "lua")
	assertContains(t, stdout, "processPayment")
}

// TestLuaDeps verifies that `inari deps processPayment` returns
// expected dependencies.
func TestLuaDeps(t *testing.T) {
	dir := setupIndexedFixture(t, "lua-simple")

	stdout, _ := runInari(t, dir, "deps", "processPayment")
	assertContains(t, stdout, "logger")
}

// TestLuaLocalFunctions verifies that local functions are indexed.
func TestLuaLocalFunctions(t *testing.T) {
	dir := setupIndexedFixture(t, "lua-simple")

	stdout, _ := runInari(t, dir, "find", "executeTransaction", "--json")
	envelope := parseJSON(t, stdout)
	data, ok := envelope["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be an array, got %T", envelope["data"])
	}
	if len(data) == 0 {
		t.Errorf("expected executeTransaction local function to be found in the index")
	}
}
