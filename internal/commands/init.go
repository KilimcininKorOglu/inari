// `inari init` -- initialise Inari for a project.
//
// Creates the .inari/ directory with a default config.toml and .gitignore.
// Auto-detects languages from project markers (tsconfig.json, .csproj, etc.).
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/KilimcininKorOglu/inari/internal/config"
)

// newInitCmd creates the cobra command for `inari init`.
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialise Inari for this project",
		Long: "Creates a .inari/ directory with default configuration.\n" +
			"Auto-detects languages from project markers (tsconfig.json, .csproj).\n" +
			"Run this once per project before running `inari index`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSingleProjectCommand(cmd, func(root string) error {
				jsonFlag, _ := cmd.Flags().GetBool("json")
				return runInit(root, jsonFlag)
			})
		},
	}
	cmd.Flags().BoolP("json", "j", false, "Output as JSON instead of human-readable format")
	return cmd
}

// runInit performs the init logic: detect languages, write config, write .gitignore.
func runInit(projectRoot string, jsonFlag bool) error {
	inariDir := filepath.Join(projectRoot, ".inari")

	// Check if already initialised.
	if _, err := os.Stat(inariDir); err == nil {
		return fmt.Errorf(
			".inari/ already exists in %s. Remove it first to re-initialise",
			projectRoot,
		)
	}

	// Create .inari/ directory.
	if err := os.MkdirAll(inariDir, 0o755); err != nil {
		return fmt.Errorf("failed to create .inari/ directory: %w", err)
	}

	// Detect languages from project markers.
	languages := detectLanguages(projectRoot)

	// Derive project name from directory name.
	projectName := filepath.Base(projectRoot)
	if projectName == "" || projectName == "." || projectName == "/" {
		projectName = "unknown"
	}

	// Write config.toml.
	cfg := config.DefaultFor(projectName, languages)
	if err := cfg.Save(inariDir); err != nil {
		// Clean up on failure.
		os.RemoveAll(inariDir)
		return fmt.Errorf("failed to write config.toml: %w", err)
	}

	// Write .gitignore inside .inari/.
	gitignoreContent := "# Ignore all Inari index files\ngraph.db\nvectors/\nfile_hashes.db\nmodels/\n"
	if err := os.WriteFile(filepath.Join(inariDir, ".gitignore"), []byte(gitignoreContent), 0o644); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}

	// Output result.
	if jsonFlag {
		result := map[string]interface{}{
			"command":      "init",
			"project_name": projectName,
			"languages":    languages,
			"inari_dir":    ".inari/",
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Printf("%s\n", data)
		return nil
	}

	// Human-readable output.
	fmt.Printf("Initialised .inari/ for project: %s\n", projectName)
	if len(languages) == 0 {
		fmt.Println("Detected languages: (none)")
	} else {
		displayNames := make([]string, len(languages))
		for i, lang := range languages {
			displayNames[i] = languageDisplayName(lang)
		}
		fmt.Printf("Detected languages: %s\n", strings.Join(displayNames, ", "))
	}
	fmt.Println("Run `inari index` to build the index.")

	return nil
}

// detectLanguages auto-detects languages from project marker files.
func detectLanguages(projectRoot string) []string {
	var languages []string

	// TypeScript / JavaScript detection.
	if fileExists(filepath.Join(projectRoot, "tsconfig.json")) ||
		fileExists(filepath.Join(projectRoot, "package.json")) {
		languages = append(languages, "typescript")
	}

	// C# detection: look for .csproj or .sln files.
	if hasCSharpFiles(projectRoot) {
		languages = append(languages, "csharp")
	}

	// Python detection.
	if fileExists(filepath.Join(projectRoot, "pyproject.toml")) ||
		fileExists(filepath.Join(projectRoot, "setup.py")) ||
		fileExists(filepath.Join(projectRoot, "requirements.txt")) ||
		fileExists(filepath.Join(projectRoot, "Pipfile")) {
		languages = append(languages, "python")
	}

	// Rust detection.
	if fileExists(filepath.Join(projectRoot, "Cargo.toml")) {
		languages = append(languages, "rust")
	}

	// Go detection.
	if fileExists(filepath.Join(projectRoot, "go.mod")) {
		languages = append(languages, "go")
	}

	// Java detection (Maven or Gradle).
	if fileExists(filepath.Join(projectRoot, "pom.xml")) ||
		fileExists(filepath.Join(projectRoot, "build.gradle")) ||
		fileExists(filepath.Join(projectRoot, "build.gradle.kts")) {
		languages = append(languages, "java")
	}

	// Kotlin detection (Kotlin DSL build files).
	if fileExists(filepath.Join(projectRoot, "build.gradle.kts")) ||
		fileExists(filepath.Join(projectRoot, "settings.gradle.kts")) {
		languages = append(languages, "kotlin")
	}

	return languages
}

// hasCSharpFiles checks if the project root contains .csproj or .sln files.
func hasCSharpFiles(projectRoot string) bool {
	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".csproj") || strings.HasSuffix(name, ".sln") {
			return true
		}
	}
	return false
}

// fileExists returns true if the given path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// languageDisplayName returns the human-readable display name for a language key.
func languageDisplayName(lang string) string {
	switch lang {
	case "typescript":
		return "TypeScript"
	case "csharp":
		return "C#"
	case "python":
		return "Python"
	case "go":
		return "Go"
	case "java":
		return "Java"
	case "kotlin":
		return "Kotlin"
	case "rust":
		return "Rust"
	default:
		return lang
	}
}
