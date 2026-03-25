package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIndexRespectsGitignore verifies that the indexer skips files matched
// by a .gitignore in the project root. Creates a TypeScript fixture with
// a .gitignore that excludes a specific file, indexes, and checks that
// the ignored file's symbols do not appear.
func TestIndexRespectsGitignore(t *testing.T) {
	dir := copyFixture(t, "typescript-simple")

	// Create a .gitignore that ignores the utils directory.
	gitignore := "src/utils/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}

	runInari(t, dir, "init")
	runInari(t, dir, "index", "--full")

	// Logger is defined in src/utils/logger.ts — it should be excluded.
	// FTS5 may still return partial matches (e.g., "logger" property in
	// service.ts). We verify none of the results come from src/utils/.
	stdout, _ := runInari(t, dir, "find", "Logger", "--json")
	envelope := parseJSON(t, stdout)
	data, _ := envelope["data"].([]interface{})
	for _, item := range data {
		result, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		filePath, _ := result["file_path"].(string)
		if strings.Contains(filePath, "src/utils/") {
			t.Errorf("expected symbols from src/utils/ to be excluded by .gitignore, but found: %s", filePath)
		}
	}
}

// TestGitignoreMultiplePatterns verifies that multiple .gitignore patterns
// are applied correctly. We ignore src/utils/ and src/controllers/ and verify
// only src/payments/ symbols remain.
func TestGitignoreMultiplePatterns(t *testing.T) {
	dir := copyFixture(t, "typescript-simple")

	gitignore := "src/utils/\nsrc/controllers/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}

	runInari(t, dir, "init")
	runInari(t, dir, "index", "--full")

	// PaymentService is in src/payments/ — should still be indexed.
	stdout, _ := runInari(t, dir, "find", "PaymentService", "--json")
	envelope := parseJSON(t, stdout)
	data, _ := envelope["data"].([]interface{})
	if len(data) == 0 {
		t.Error("expected PaymentService from src/payments/ to remain in the index")
	}

	// OrderController is in src/controllers/ — should be excluded.
	stdout2, _ := runInari(t, dir, "find", "OrderController", "--json")
	envelope2 := parseJSON(t, stdout2)
	data2, _ := envelope2["data"].([]interface{})
	for _, item := range data2 {
		result, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		filePath, _ := result["file_path"].(string)
		if strings.Contains(filePath, "src/controllers/") {
			t.Errorf("expected symbols from src/controllers/ to be excluded, but found: %s", filePath)
		}
	}
}
