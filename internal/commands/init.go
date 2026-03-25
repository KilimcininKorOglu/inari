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

	// Ruby detection.
	if fileExists(filepath.Join(projectRoot, "Gemfile")) ||
		fileExists(filepath.Join(projectRoot, "Rakefile")) ||
		fileExists(filepath.Join(projectRoot, "config.ru")) {
		languages = append(languages, "ruby")
	}

	// PHP detection.
	if fileExists(filepath.Join(projectRoot, "composer.json")) ||
		fileExists(filepath.Join(projectRoot, "artisan")) {
		languages = append(languages, "php")
	}

	// Swift detection.
	if fileExists(filepath.Join(projectRoot, "Package.swift")) ||
		hasXcodeProject(projectRoot) {
		languages = append(languages, "swift")
	}

	// C++ detection.
	if hasCppFiles(projectRoot) {
		languages = append(languages, "cpp")
	}

	// C detection.
	if hasCSourceFiles(projectRoot) {
		languages = append(languages, "c")
	}

	// Bash/Shell detection.
	if hasBashFiles(projectRoot) {
		languages = append(languages, "bash")
	}

	// SQL detection.
	if hasSQLFiles(projectRoot) {
		languages = append(languages, "sql")
	}

	// Protobuf detection.
	if fileExists(filepath.Join(projectRoot, "buf.yaml")) ||
		fileExists(filepath.Join(projectRoot, "buf.gen.yaml")) ||
		fileExists(filepath.Join(projectRoot, "buf.work.yaml")) ||
		hasProtoFiles(projectRoot) {
		languages = append(languages, "protobuf")
	}

	// Lua detection.
	if fileExists(filepath.Join(projectRoot, ".luarc.json")) ||
		fileExists(filepath.Join(projectRoot, ".luarc.jsonc")) ||
		fileExists(filepath.Join(projectRoot, "init.lua")) ||
		fileExists(filepath.Join(projectRoot, "conf.lua")) ||
		hasLuarockFiles(projectRoot) {
		languages = append(languages, "lua")
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

// hasCppFiles checks if the project root contains .cpp/.cc/.cxx files.
func hasCppFiles(projectRoot string) bool {
	cppExts := []string{".cpp", ".cc", ".cxx"}
	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		for _, ext := range cppExts {
			if strings.HasSuffix(entry.Name(), ext) {
				return true
			}
		}
	}
	srcDir := filepath.Join(projectRoot, "src")
	if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
		srcEntries, err := os.ReadDir(srcDir)
		if err == nil {
			for _, entry := range srcEntries {
				for _, ext := range cppExts {
					if strings.HasSuffix(entry.Name(), ext) {
						return true
					}
				}
			}
		}
	}
	return false
}

// hasCSourceFiles checks if the project root contains .c source files.
func hasCSourceFiles(projectRoot string) bool {
	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		return false
	}
	// Check for Makefile + .c files or CMakeLists.txt.
	if fileExists(filepath.Join(projectRoot, "CMakeLists.txt")) {
		return true
	}
	hasMakefile := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "Makefile" || name == "makefile" {
			hasMakefile = true
		}
		if strings.HasSuffix(name, ".c") {
			if hasMakefile {
				return true
			}
		}
	}
	// If there's a src/ dir with .c files, detect.
	srcDir := filepath.Join(projectRoot, "src")
	if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
		srcEntries, err := os.ReadDir(srcDir)
		if err == nil {
			for _, entry := range srcEntries {
				if strings.HasSuffix(entry.Name(), ".c") {
					return true
				}
			}
		}
	}
	return false
}

// hasBashFiles checks if the project root contains .sh files.
func hasBashFiles(projectRoot string) bool {
	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".sh") {
			return true
		}
	}
	return false
}

// hasXcodeProject checks if the project root contains .xcodeproj or .xcworkspace directories.
func hasXcodeProject(projectRoot string) bool {
	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".xcodeproj") || strings.HasSuffix(name, ".xcworkspace") {
			return true
		}
	}
	return false
}

// hasLuarockFiles checks if the project root contains .rockspec files or a .luarocks directory.
func hasLuarockFiles(projectRoot string) bool {
	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == ".luarocks" {
				return true
			}
			continue
		}
		if strings.HasSuffix(entry.Name(), ".rockspec") {
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
	case "ruby":
		return "Ruby"
	case "php":
		return "PHP"
	case "lua":
		return "Lua"
	case "swift":
		return "Swift"
	case "bash":
		return "Bash"
	case "c":
		return "C"
	case "cpp":
		return "C++"
	case "protobuf":
		return "Protocol Buffers"
	case "sql":
		return "SQL"
	default:
		return lang
	}
}

// hasProtoFiles checks if the project root or proto/ subdirectory contains .proto files.
func hasProtoFiles(projectRoot string) bool {
	dirs := []string{projectRoot, filepath.Join(projectRoot, "proto"), filepath.Join(projectRoot, "src")}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".proto") {
				return true
			}
		}
	}
	return false
}

// hasSQLFiles checks if the project contains .sql files in common locations or migration markers.
func hasSQLFiles(projectRoot string) bool {
	// Check for migration tool markers.
	markers := []string{
		"flyway.conf",
		"knexfile.js", "knexfile.ts",
		"alembic.ini",
		"dbmate.yml",
	}
	for _, m := range markers {
		if fileExists(filepath.Join(projectRoot, m)) {
			return true
		}
	}

	// Check common SQL directories.
	dirs := []string{
		filepath.Join(projectRoot, "sql"),
		filepath.Join(projectRoot, "migrations"),
		filepath.Join(projectRoot, "db"),
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
				return true
			}
		}
	}
	return false
}
