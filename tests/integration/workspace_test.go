package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// initInariProject creates a minimal .inari/config.toml in the given directory
// so that workspace discovery recognises it as an inari project.
func initInariProject(t *testing.T, projectDir, name string) {
	t.Helper()
	inariDir := filepath.Join(projectDir, ".inari")
	if err := os.MkdirAll(inariDir, 0755); err != nil {
		t.Fatalf("failed to create .inari dir: %v", err)
	}

	config := "[project]\nname = \"" + name + "\"\nlanguages = [\"typescript\"]\n\n[index]\nignore = [\"node_modules\"]\n"
	if err := os.WriteFile(filepath.Join(inariDir, "config.toml"), []byte(config), 0644); err != nil {
		t.Fatalf("failed to write config.toml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(inariDir, ".gitignore"), []byte("*\n"), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}
}

// initAndIndexProject creates a TypeScript project with a source file, runs
// inari init + inari index --full, so the project has a complete graph.db.
func initAndIndexProject(t *testing.T, projectDir, name string) {
	t.Helper()

	srcDir := filepath.Join(projectDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	mainContent := "export class " + name + "Service {\n  process(): void {}\n}\n"
	if err := os.WriteFile(filepath.Join(srcDir, "main.ts"), []byte(mainContent), 0644); err != nil {
		t.Fatalf("failed to write main.ts: %v", err)
	}

	tsconfigContent := `{"compilerOptions": {"target": "es2020"}}`
	if err := os.WriteFile(filepath.Join(projectDir, "tsconfig.json"), []byte(tsconfigContent), 0644); err != nil {
		t.Fatalf("failed to write tsconfig.json: %v", err)
	}

	runInari(t, projectDir, "init")
	runInari(t, projectDir, "index", "--full")
}

// TestWorkspaceInit verifies that `inari workspace init` discovers inari
// projects in subdirectories and creates an inari-workspace.toml manifest.
func TestWorkspaceInit(t *testing.T) {
	dir := t.TempDir()

	// Create two projects with .inari/config.toml.
	apiDir := filepath.Join(dir, "api")
	workerDir := filepath.Join(dir, "worker")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}
	if err := os.MkdirAll(workerDir, 0755); err != nil {
		t.Fatalf("failed to create worker dir: %v", err)
	}
	initInariProject(t, apiDir, "api")
	initInariProject(t, workerDir, "worker")

	_, stderr := runInari(t, dir, "workspace", "init")
	assertContains(t, stderr, "Found 2 projects")

	// Verify the manifest was created.
	manifestPath := filepath.Join(dir, "inari-workspace.toml")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("inari-workspace.toml should be created: %v", err)
	}

	manifest := string(content)
	assertContains(t, manifest, "[workspace]")
	assertContains(t, manifest, "api")
	assertContains(t, manifest, "worker")
}

// TestWorkspaceList verifies that `inari workspace list` shows workspace
// members with their status.
func TestWorkspaceList(t *testing.T) {
	dir := t.TempDir()

	// Create and index one project.
	apiDir := filepath.Join(dir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}
	initAndIndexProject(t, apiDir, "Api")

	// Create another project that is initialised but not indexed.
	workerDir := filepath.Join(dir, "worker")
	if err := os.MkdirAll(workerDir, 0755); err != nil {
		t.Fatalf("failed to create worker dir: %v", err)
	}
	initInariProject(t, workerDir, "worker")

	// Write workspace manifest.
	manifest := `[workspace]
name = "test-ws"
version = 1

[[workspace.members]]
path = "api"
name = "api"

[[workspace.members]]
path = "worker"
name = "worker"
`
	if err := os.WriteFile(filepath.Join(dir, "inari-workspace.toml"), []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	stdout, _ := runInari(t, dir, "workspace", "list")
	assertContains(t, stdout, "test-ws")
	assertContains(t, stdout, "api")
	assertContains(t, stdout, "worker")
}

// TestWorkspaceIndex verifies that `inari workspace init` followed by
// workspace-level indexing works with real projects built from fixtures.
func TestWorkspaceIndex(t *testing.T) {
	dir := t.TempDir()

	// Create two subdirectories and set up real projects in each.
	frontendDir := filepath.Join(dir, "frontend")
	backendDir := filepath.Join(dir, "backend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("failed to create frontend dir: %v", err)
	}
	if err := os.MkdirAll(backendDir, 0755); err != nil {
		t.Fatalf("failed to create backend dir: %v", err)
	}

	// Copy the typescript-simple fixture into the frontend directory.
	fixtureDir := filepath.Join(getProjectRoot(), "tests", "fixtures", "typescript-simple")
	copyDirAll(t, fixtureDir, frontendDir)
	runInari(t, frontendDir, "init")
	runInari(t, frontendDir, "index", "--full")

	// Create a second project from scratch in the backend directory.
	initAndIndexProject(t, backendDir, "Backend")

	// Run workspace init from the parent directory.
	_, stderr := runInari(t, dir, "workspace", "init")
	assertContains(t, stderr, "Found 2 projects")

	// Verify the manifest was created.
	manifestPath := filepath.Join(dir, "inari-workspace.toml")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("inari-workspace.toml should be created: %v", err)
	}
	manifest := string(content)
	assertContains(t, manifest, "[workspace]")
	assertContains(t, manifest, "frontend")
	assertContains(t, manifest, "backend")
}

// TestWorkspaceInitAlreadyExists verifies that running `inari workspace init`
// when a manifest already exists fails with an appropriate error.
func TestWorkspaceInitAlreadyExists(t *testing.T) {
	dir := t.TempDir()

	manifest := "[workspace]\nname = \"test\"\n\n[[workspace.members]]\npath = \"a\"\n"
	if err := os.WriteFile(filepath.Join(dir, "inari-workspace.toml"), []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	_, stderr := runInariExpectFail(t, dir, "workspace", "init")
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "already") {
		t.Errorf("expected stderr to mention 'already', got: %s", stderr)
	}
}

// TestWorkspaceInitNoProjects verifies that running `inari workspace init`
// in an empty directory fails because no inari projects are found.
func TestWorkspaceInitNoProjects(t *testing.T) {
	dir := t.TempDir()

	_, stderr := runInariExpectFail(t, dir, "workspace", "init")
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "no inari projects") && !strings.Contains(lower, "no projects") {
		t.Errorf("expected stderr to mention 'no inari projects', got: %s", stderr)
	}
}

// TestWorkspaceInitRootProject verifies that `inari workspace init` discovers
// the root project (path=".") when the workspace root itself is an Inari project.
func TestWorkspaceInitRootProject(t *testing.T) {
	dir := t.TempDir()

	// Root is itself an Inari project.
	initInariProject(t, dir, "backend")

	// Also create a subdirectory project.
	webDir := filepath.Join(dir, "web")
	if err := os.MkdirAll(webDir, 0755); err != nil {
		t.Fatalf("failed to create web dir: %v", err)
	}
	initInariProject(t, webDir, "web")

	_, stderr := runInari(t, dir, "workspace", "init")
	assertContains(t, stderr, "Found 2 projects")

	content, err := os.ReadFile(filepath.Join(dir, "inari-workspace.toml"))
	if err != nil {
		t.Fatalf("manifest should exist: %v", err)
	}
	manifest := string(content)
	assertContains(t, manifest, "backend")
	assertContains(t, manifest, "web")
	assertContains(t, manifest, `path = "."`)
}

// TestWorkspaceInitRootOnly verifies that `inari workspace init` discovers
// the root project even when no subdirectory projects exist.
func TestWorkspaceInitRootOnly(t *testing.T) {
	dir := t.TempDir()
	initInariProject(t, dir, "solo-project")

	_, stderr := runInari(t, dir, "workspace", "init")
	assertContains(t, stderr, "Found 1 project")

	content, err := os.ReadFile(filepath.Join(dir, "inari-workspace.toml"))
	if err != nil {
		t.Fatalf("manifest should exist: %v", err)
	}
	manifest := string(content)
	assertContains(t, manifest, "solo-project")
	assertContains(t, manifest, `path = "."`)
}

// TestWorkspaceListJson verifies that `inari workspace list --json` returns
// valid JSON with expected fields.
func TestWorkspaceListJson(t *testing.T) {
	dir := t.TempDir()

	// Create and index a project.
	apiDir := filepath.Join(dir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatalf("failed to create api dir: %v", err)
	}
	initAndIndexProject(t, apiDir, "Api")

	// Write workspace manifest.
	manifest := `[workspace]
name = "test-ws"

[[workspace.members]]
path = "api"
name = "api"
`
	if err := os.WriteFile(filepath.Join(dir, "inari-workspace.toml"), []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	stdout, _ := runInari(t, dir, "workspace", "list", "--json")

	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nraw: %s", err, stdout)
	}

	cmd, _ := envelope["command"].(string)
	if cmd != "workspace list" {
		t.Errorf("expected command='workspace list', got %s", cmd)
	}

	data, ok := envelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be an object, got %T", envelope["data"])
	}

	wsName, _ := data["workspace_name"].(string)
	if wsName != "test-ws" {
		t.Errorf("expected workspace_name=test-ws, got %s", wsName)
	}

	members, ok := data["members"].([]interface{})
	if !ok {
		t.Fatalf("expected data.members to be an array, got %T", data["members"])
	}
	if len(members) == 0 {
		t.Error("expected at least one member in workspace list")
	}
}

// TestWorkspaceListFailsWithoutManifest verifies that `inari workspace list`
// fails when there is no inari-workspace.toml.
func TestWorkspaceListFailsWithoutManifest(t *testing.T) {
	dir := t.TempDir()

	_, stderr := runInariExpectFail(t, dir, "workspace", "list")
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "inari-workspace.toml") && !strings.Contains(lower, "no workspace") {
		t.Errorf("expected stderr to mention inari-workspace.toml, got: %s", stderr)
	}
}
