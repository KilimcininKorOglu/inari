package integration_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var inariBinary string

func TestMain(m *testing.M) {
	// Build the inari binary once before all tests.
	tmpDir, err := os.MkdirTemp("", "inari-test-bin")
	if err != nil {
		panic("failed to create temp dir for binary: " + err.Error())
	}
	inariBinary = filepath.Join(tmpDir, "inari")

	cmd := exec.Command("go", "build", "-o", inariBinary, "./cmd/inari/")
	cmd.Dir = getProjectRoot()
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("failed to build inari: " + string(out))
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// getProjectRoot returns the absolute path to the repository root.
// It walks upward from the current working directory until it finds
// a directory that contains go.mod.
func getProjectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic("cannot determine working directory: " + err.Error())
	}
	dir := wd
	for {
		candidate := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(candidate); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find project root from " + wd)
		}
		dir = parent
	}
}

// copyFixture copies the named fixture directory into a new temporary
// directory and returns the path. Cleanup is registered via t.Cleanup.
func copyFixture(t *testing.T, fixtureName string) string {
	t.Helper()

	fixtureDir := filepath.Join(getProjectRoot(), "tests", "fixtures", fixtureName)
	if _, err := os.Stat(fixtureDir); err != nil {
		t.Fatalf("fixture %q not found at %s: %v", fixtureName, fixtureDir, err)
	}

	tmpDir := t.TempDir() // automatically cleaned up
	copyDirAll(t, fixtureDir, tmpDir)
	return tmpDir
}

// copyDirAll recursively copies the contents of src into dest.
func copyDirAll(t *testing.T, src, dest string) {
	t.Helper()

	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("reading directory %s: %v", src, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				t.Fatalf("creating directory %s: %v", destPath, err)
			}
			copyDirAll(t, srcPath, destPath)
		} else {
			srcFile, err := os.Open(srcPath)
			if err != nil {
				t.Fatalf("opening %s: %v", srcPath, err)
			}
			destFile, err := os.Create(destPath)
			if err != nil {
				srcFile.Close()
				t.Fatalf("creating %s: %v", destPath, err)
			}
			if _, err := io.Copy(destFile, srcFile); err != nil {
				srcFile.Close()
				destFile.Close()
				t.Fatalf("copying %s -> %s: %v", srcPath, destPath, err)
			}
			srcFile.Close()
			destFile.Close()
		}
	}
}

// runInari executes the inari binary in the given directory with the supplied
// arguments. It asserts a zero exit code and returns stdout and stderr.
func runInari(t *testing.T, dir string, args ...string) (stdout, stderr string) {
	t.Helper()

	cmd := exec.Command(inariBinary, args...)
	cmd.Dir = dir

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		t.Fatalf("inari %v failed (exit %v)\nstdout: %s\nstderr: %s",
			args, err, outBuf.String(), errBuf.String())
	}
	return outBuf.String(), errBuf.String()
}

// runInariExpectFail executes the inari binary in the given directory with the
// supplied arguments. It asserts a non-zero exit code and returns stdout and
// stderr.
func runInariExpectFail(t *testing.T, dir string, args ...string) (stdout, stderr string) {
	t.Helper()

	cmd := exec.Command(inariBinary, args...)
	cmd.Dir = dir

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected inari %v to fail, but it succeeded\nstdout: %s\nstderr: %s",
			args, outBuf.String(), errBuf.String())
	}
	return outBuf.String(), errBuf.String()
}

// setupIndexedFixture copies the named fixture, runs `inari init` and
// `inari index --full`, and returns the working directory path.
func setupIndexedFixture(t *testing.T, fixtureName string) string {
	t.Helper()

	dir := copyFixture(t, fixtureName)
	removeInariDir(t, dir)
	runInari(t, dir, "init")
	runInari(t, dir, "index", "--full")
	return dir
}

// assertContains fails the test if output does not contain substring.
func assertContains(t *testing.T, output, substring string) {
	t.Helper()
	if !strings.Contains(output, substring) {
		t.Errorf("expected output to contain %q, but it did not.\noutput:\n%s",
			substring, truncate(output, 1000))
	}
}

// assertNotContains fails the test if output contains substring.
func assertNotContains(t *testing.T, output, substring string) {
	t.Helper()
	if strings.Contains(output, substring) {
		t.Errorf("expected output NOT to contain %q, but it did.\noutput:\n%s",
			substring, truncate(output, 1000))
	}
}

// removeInariDir removes the .inari/ directory from the given path if it
// exists. This is needed for fixtures that ship with a pre-initialised
// .inari/ so that `inari init` can run cleanly.
func removeInariDir(t *testing.T, dir string) {
	t.Helper()
	inariDir := filepath.Join(dir, ".inari")
	if _, err := os.Stat(inariDir); err == nil {
		if err := os.RemoveAll(inariDir); err != nil {
			t.Fatalf("failed to remove .inari dir: %v", err)
		}
	}
}

// truncate shortens s to at most maxLen characters, appending "..." if trimmed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("... (%d bytes total)", len(s))
}
