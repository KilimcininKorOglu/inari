package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// InitWorkDir copies a fixture directory into a destination work directory.
// This sets up an isolated copy where the agent can make changes without
// affecting the original fixture.
func InitWorkDir(src, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dest, err)
	}

	return copyDirRecursive(src, dest)
}

// CreateBranch creates and checks out a new git branch in the given directory.
func CreateBranch(dir, branch string) error {
	cmd := exec.Command("git", "checkout", "-b", branch)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout -b %s failed in %s: %s: %w", branch, dir, string(output), err)
	}
	return nil
}

// CommitAll stages all changes and creates a commit with the given message.
func CommitAll(dir, message string) error {
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = dir
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add -A failed in %s: %s: %w", dir, string(output), err)
	}

	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = dir
	if output, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed in %s: %s: %w", dir, string(output), err)
	}

	return nil
}

// DiffStat returns the output of `git diff --stat` for the given directory.
func DiffStat(dir string) (string, error) {
	cmd := exec.Command("git", "diff", "--stat")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff --stat failed in %s: %w", dir, err)
	}
	return string(output), nil
}

// ResetCorpus resets a corpus directory to its clean state.
// Runs `git reset --hard HEAD` and `git clean -fd`.
func ResetCorpus(corpusPath string) error {
	resetCmd := exec.Command("git", "reset", "--hard", "HEAD")
	resetCmd.Dir = corpusPath
	if err := resetCmd.Run(); err != nil {
		return fmt.Errorf("git reset --hard HEAD failed in %s: %w", corpusPath, err)
	}

	cleanCmd := exec.Command("git", "clean", "-fd")
	cleanCmd.Dir = corpusPath
	if err := cleanCmd.Run(); err != nil {
		return fmt.Errorf("git clean -fd failed in %s: %w", corpusPath, err)
	}

	return nil
}

// BackupInariIndex backs up the .inari/ index directory before a benchmark run.
// Returns the path to the backup directory.
func BackupInariIndex(corpusPath string) (string, error) {
	inariDir := filepath.Join(corpusPath, ".inari")
	if !dirExists(inariDir) {
		return "", fmt.Errorf("no .inari/ directory found in %s. Build the index first with 'inari index'", corpusPath)
	}

	backupDir := filepath.Join(os.TempDir(), fmt.Sprintf("inari-benchmark-backup-%d", os.Getpid()))

	if dirExists(backupDir) {
		if err := os.RemoveAll(backupDir); err != nil {
			return "", fmt.Errorf("failed to clean old backup at %s: %w", backupDir, err)
		}
	}

	if err := copyDirRecursive(inariDir, backupDir); err != nil {
		return "", fmt.Errorf("failed to back up .inari/ to %s: %w", backupDir, err)
	}

	return backupDir, nil
}

// RestoreInariIndex restores the .inari/ index from a backup.
func RestoreInariIndex(corpusPath, backupPath string) error {
	inariDir := filepath.Join(corpusPath, ".inari")

	if dirExists(inariDir) {
		if err := os.RemoveAll(inariDir); err != nil {
			return fmt.Errorf("failed to remove .inari/ at %s: %w", inariDir, err)
		}
	}

	if err := copyDirRecursive(backupPath, inariDir); err != nil {
		return fmt.Errorf("failed to restore .inari/ from %s: %w", backupPath, err)
	}

	return nil
}
