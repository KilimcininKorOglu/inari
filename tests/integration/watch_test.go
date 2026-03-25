package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestWatchLockFile verifies that starting `inari index --watch` creates a
// lock file at .inari/.watch.lock containing a valid PID.
func TestWatchLockFile(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	lockPath := filepath.Join(dir, ".inari", ".watch.lock")

	// Start a watch process that we will kill after checking the lock file.
	cmd := exec.Command(inariBinary, "index", "--watch")
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start inari index --watch: %v", err)
	}

	// Ensure cleanup.
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Wait for the lock file to appear (poll up to 10 seconds).
	var found bool
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if _, err := os.Stat(lockPath); err == nil {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("lock file was not created within 10 seconds")
	}

	content, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("failed to read lock file: %v", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		t.Fatalf("lock file should contain a valid PID, got: %q", string(content))
	}
	if pid <= 0 {
		t.Errorf("PID should be positive, got %d", pid)
	}
}

// TestWatchLockPreventsSecond verifies that trying to start a second watcher
// while one is already running fails with an error about the existing watcher.
func TestWatchLockPreventsSecond(t *testing.T) {
	dir := setupIndexedFixture(t, "typescript-simple")

	lockPath := filepath.Join(dir, ".inari", ".watch.lock")

	// Start first watcher.
	cmd := exec.Command(inariBinary, "index", "--watch")
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start first watcher: %v", err)
	}

	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Wait for the lock file to appear.
	var found bool
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if _, err := os.Stat(lockPath); err == nil {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("first watcher did not create lock file within 10 seconds")
	}

	// Try to start a second watcher — should fail.
	_, stderr := runInariExpectFail(t, dir, "index", "--watch")
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "another watcher") && !strings.Contains(lower, "already running") && !strings.Contains(lower, "lock") {
		t.Errorf("expected error about existing watcher, got: %s", stderr)
	}
}
