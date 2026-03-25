// Self-update logic for Inari CLI.
//
// Checks GitHub Releases for newer versions and downloads the matching
// platform binary. Used by both the explicit `inari update` command
// and the background update check in PersistentPreRun.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Platform variables (can be overridden in tests).
var (
	runtimeOS   = runtime.GOOS
	runtimeArch = runtime.GOARCH
)

const (
	githubRepo      = "KilimcininKorOglu/inari"
	githubAPIBase   = "https://api.github.com"
	updateCheckFile = ".inari-update-check"
	checkCooldown   = 24 * time.Hour
	httpTimeout     = 15 * time.Second
	bgCheckTimeout  = 3 * time.Second
)

// githubRelease represents the relevant fields from GitHub's release API.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

// githubAsset represents a single release asset.
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// checkLatestVersion fetches the latest release from GitHub.
func checkLatestVersion(ctx context.Context) (*githubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIBase, githubRepo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "inari-cli/"+Version)

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release JSON: %w", err)
	}

	return &release, nil
}

// compareVersions compares two semantic version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareVersions(a, b string) int {
	parseVersion := func(v string) (int, int, int) {
		v = strings.TrimPrefix(v, "v")
		parts := strings.SplitN(v, ".", 3)
		major, minor, patch := 0, 0, 0
		if len(parts) > 0 {
			major, _ = strconv.Atoi(parts[0])
		}
		if len(parts) > 1 {
			minor, _ = strconv.Atoi(parts[1])
		}
		if len(parts) > 2 {
			// Strip pre-release suffix (e.g., "1-beta")
			patchStr := strings.SplitN(parts[2], "-", 2)[0]
			patch, _ = strconv.Atoi(patchStr)
		}
		return major, minor, patch
	}

	aMaj, aMin, aPat := parseVersion(a)
	bMaj, bMin, bPat := parseVersion(b)

	if aMaj != bMaj {
		if aMaj < bMaj {
			return -1
		}
		return 1
	}
	if aMin != bMin {
		if aMin < bMin {
			return -1
		}
		return 1
	}
	if aPat != bPat {
		if aPat < bPat {
			return -1
		}
		return 1
	}
	return 0
}

// findAssetForPlatform finds the release asset matching the current OS/arch.
func findAssetForPlatform(assets []githubAsset, version string) *githubAsset {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}

	expected := fmt.Sprintf("inari-%s-%s-%s%s", version, goos, goarch, ext)

	for i := range assets {
		if assets[i].Name == expected {
			return &assets[i]
		}
	}
	return nil
}

// downloadAndReplace downloads a binary from url and replaces the running binary.
func downloadAndReplace(url string) error {
	// Download to temp file.
	tmpFile, err := os.CreateTemp("", "inari-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write update binary: %w", err)
	}
	tmpFile.Close()

	// Make executable (Unix).
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to set executable permission: %w", err)
		}
	}

	// Find current binary path.
	currentPath, err := os.Executable()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to determine current binary path: %w", err)
	}
	currentPath, err = filepath.EvalSymlinks(currentPath)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to resolve binary symlinks: %w", err)
	}

	// Atomic-ish swap: rename current → .old, move new → current.
	backupPath := currentPath + ".old"
	os.Remove(backupPath) // clean up any previous .old

	if err := os.Rename(currentPath, backupPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	if err := os.Rename(tmpPath, currentPath); err != nil {
		// Restore backup on failure.
		os.Rename(backupPath, currentPath)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	os.Remove(backupPath)
	return nil
}

// shouldCheckForUpdate returns true if enough time has passed since the last check.
func shouldCheckForUpdate() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	path := filepath.Join(home, updateCheckFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return true // never checked
	}

	ts, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return true
	}
	return time.Since(time.Unix(ts, 0)) > checkCooldown
}

// markUpdateChecked writes the current timestamp to the check file.
func markUpdateChecked() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := filepath.Join(home, updateCheckFile)
	os.WriteFile(path, []byte(strconv.FormatInt(time.Now().Unix(), 10)), 0o644)
}

// backgroundUpdateCheck runs a non-blocking update check and prints a notice to stderr.
func backgroundUpdateCheck(currentVersion string) {
	if !shouldCheckForUpdate() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), bgCheckTimeout)
	defer cancel()

	release, err := checkLatestVersion(ctx)
	if err != nil {
		return // fail silently
	}

	markUpdateChecked()

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	if compareVersions(current, latestVersion) < 0 {
		fmt.Fprintf(os.Stderr, "A new version of inari is available: v%s (current: v%s). Run 'inari update' to upgrade.\n", latestVersion, current)
	}
}
