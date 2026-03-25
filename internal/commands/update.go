package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/KilimcininKorOglu/inari/internal/output"
	"github.com/spf13/cobra"
)

// newUpdateCmd creates the `inari update` command.
func newUpdateCmd() *cobra.Command {
	var checkOnly bool
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and install updates",
		Long: `Check for and install updates from GitHub Releases.

By default, downloads and installs the latest version. Use --check
to only check without installing.`,
		Example: `  inari update          # download and install latest version
  inari update --check  # check for updates without installing
  inari update --json   # JSON output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(checkOnly, jsonFlag)
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "Check for updates without installing")
	cmd.Flags().BoolVarP(&jsonFlag, "json", "j", false, "Output as JSON")

	return cmd
}

func runUpdate(checkOnly bool, jsonFlag bool) error {
	currentVersion := strings.TrimPrefix(Version, "v")

	fmt.Fprintf(os.Stderr, "Checking for updates...\n")

	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	release, err := checkLatestVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	updateAvailable := compareVersions(currentVersion, latestVersion) < 0

	if !updateAvailable {
		data := output.UpdateData{
			CurrentVersion:  currentVersion,
			LatestVersion:   latestVersion,
			UpdateAvailable: false,
			Updated:         false,
		}
		if jsonFlag {
			return output.PrintJSON(output.JsonOutput[output.UpdateData]{
				Command: "update",
				Data:    data,
			})
		}
		fmt.Printf("Already up to date (v%s).\n", currentVersion)
		return nil
	}

	// Find the asset for the current platform.
	asset := findAssetForPlatform(release.Assets, release.TagName)
	if asset == nil {
		return fmt.Errorf("no release binary found for your platform (%s/%s)", runtimeGOOS(), runtimeGOARCH())
	}

	if checkOnly {
		data := output.UpdateData{
			CurrentVersion:  currentVersion,
			LatestVersion:   latestVersion,
			UpdateAvailable: true,
			DownloadURL:     asset.BrowserDownloadURL,
			Updated:         false,
		}
		if jsonFlag {
			return output.PrintJSON(output.JsonOutput[output.UpdateData]{
				Command: "update",
				Data:    data,
			})
		}
		fmt.Printf("Update available: v%s -> v%s\n", currentVersion, latestVersion)
		fmt.Printf("Run 'inari update' to install.\n")
		return nil
	}

	// Download and replace.
	fmt.Fprintf(os.Stderr, "Downloading v%s...\n", latestVersion)

	if err := downloadAndReplace(asset.BrowserDownloadURL); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	markUpdateChecked()

	data := output.UpdateData{
		CurrentVersion:  currentVersion,
		LatestVersion:   latestVersion,
		UpdateAvailable: true,
		Updated:         true,
	}
	if jsonFlag {
		return output.PrintJSON(output.JsonOutput[output.UpdateData]{
			Command: "update",
			Data:    data,
		})
	}
	fmt.Printf("Updated inari v%s -> v%s\n", currentVersion, latestVersion)
	return nil
}

// runtimeGOOS returns the current OS (wrapper for testability).
func runtimeGOOS() string {
	return runtimeOS
}

// runtimeGOARCH returns the current architecture (wrapper for testability).
func runtimeGOARCH() string {
	return runtimeArch
}
