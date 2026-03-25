// Project configuration stored in `.inari/config.toml`.
//
// This is the user-facing configuration that controls indexing behaviour,
// language selection, and output defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ProjectConfig is the top-level project configuration.
type ProjectConfig struct {
	// Project-level settings.
	Project ProjectSection `toml:"project"`
	// Index behaviour settings.
	Index IndexSection `toml:"index"`
	// Embedding provider settings.
	Embeddings EmbeddingsSection `toml:"embeddings"`
	// Output formatting defaults.
	Output OutputSection `toml:"output"`
}

// ProjectSection is the [project] section of config.toml.
type ProjectSection struct {
	// Human-readable project name.
	Name string `toml:"name"`
	// Languages to index.
	Languages []string `toml:"languages"`
}

// IndexSection is the [index] section of config.toml.
type IndexSection struct {
	// Glob patterns to ignore during indexing.
	Ignore []string `toml:"ignore"`
	// Whether to include test files in refs/impact output.
	IncludeTests bool `toml:"include_tests"`
}

// EmbeddingsSection is the [embeddings] section of config.toml.
type EmbeddingsSection struct {
	// Embedding provider: "local", "voyage", or "openai".
	Provider string `toml:"provider"`
	// Model name for the embedding provider.
	Model string `toml:"model"`
}

// OutputSection is the [output] section of config.toml.
type OutputSection struct {
	// Maximum number of refs to display before truncating.
	MaxRefs int `toml:"max_refs"`
	// Maximum depth for impact analysis traversal.
	MaxDepth int `toml:"max_depth"`
}

// defaultIgnorePatterns returns the default list of ignore patterns.
func defaultIgnorePatterns() []string {
	return []string{"node_modules", "dist", "build", ".git"}
}

// defaultIndexSection returns an IndexSection with default values.
func defaultIndexSection() IndexSection {
	return IndexSection{
		Ignore:       defaultIgnorePatterns(),
		IncludeTests: true,
	}
}

// defaultEmbeddingsSection returns an EmbeddingsSection with default values.
func defaultEmbeddingsSection() EmbeddingsSection {
	return EmbeddingsSection{
		Provider: "local",
		Model:    "nomic-embed-code",
	}
}

// defaultOutputSection returns an OutputSection with default values.
func defaultOutputSection() OutputSection {
	return OutputSection{
		MaxRefs:  20,
		MaxDepth: 3,
	}
}

// LoadProjectConfig loads configuration from a .inari/config.toml file.
// inariDir should be the path to the .inari directory.
func LoadProjectConfig(inariDir string) (*ProjectConfig, error) {
	configPath := filepath.Join(inariDir, "config.toml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	// Start with defaults so that missing sections get sensible values.
	cfg := &ProjectConfig{
		Index:      defaultIndexSection(),
		Embeddings: defaultEmbeddingsSection(),
		Output:     defaultOutputSection(),
	}

	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config.toml: %w", err)
	}

	return cfg, nil
}

// Save writes the configuration to .inari/config.toml.
// inariDir should be the path to the .inari directory.
func (c *ProjectConfig) Save(inariDir string) error {
	configPath := filepath.Join(inariDir, "config.toml")

	// Ensure the directory exists.
	if err := os.MkdirAll(inariDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", inariDir, err)
	}

	f, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", configPath, err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to write config.toml: %w", err)
	}

	return nil
}

// DefaultFor creates a default ProjectConfig for a project with the given
// name and detected languages.
func DefaultFor(name string, languages []string) *ProjectConfig {
	return &ProjectConfig{
		Project: ProjectSection{
			Name:      name,
			Languages: languages,
		},
		Index:      defaultIndexSection(),
		Embeddings: defaultEmbeddingsSection(),
		Output:     defaultOutputSection(),
	}
}
