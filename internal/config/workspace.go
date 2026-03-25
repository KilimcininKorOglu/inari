// Workspace configuration stored in `inari-workspace.toml`.
//
// A workspace groups multiple Inari projects (each with its own `.inari/`
// directory) and allows federated queries across all members.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// WorkspaceConfig is the top-level workspace manifest parsed from inari-workspace.toml.
type WorkspaceConfig struct {
	// The [workspace] section.
	Workspace WorkspaceSection `toml:"workspace"`
}

// WorkspaceSection is the [workspace] section of inari-workspace.toml.
type WorkspaceSection struct {
	// Human-readable workspace name.
	Name string `toml:"name"`
	// Schema version for forward compatibility. Current: 1.
	Version *int `toml:"version,omitempty"`
	// Member project entries.
	Members []WorkspaceMemberEntry `toml:"members"`
	// Shared defaults inherited by all members unless overridden.
	Defaults *WorkspaceDefaults `toml:"defaults,omitempty"`
}

// WorkspaceMemberEntry is a single member entry in the workspace manifest.
type WorkspaceMemberEntry struct {
	// Path relative to the workspace root.
	Path string `toml:"path"`
	// Optional human-readable name. Defaults to directory basename.
	Name *string `toml:"name,omitempty"`
}

// WorkspaceDefaults holds shared defaults that apply to all workspace members
// unless overridden at the project level.
type WorkspaceDefaults struct {
	// Default index settings.
	Index *IndexDefaults `toml:"index,omitempty"`
	// Default output settings.
	Output *OutputDefaults `toml:"output,omitempty"`
}

// IndexDefaults holds default index settings inherited by workspace members.
type IndexDefaults struct {
	// Glob patterns to ignore during indexing (additive with project-level ignores).
	Ignore []string `toml:"ignore,omitempty"`
}

// OutputDefaults holds default output settings inherited by workspace members.
type OutputDefaults struct {
	// Maximum number of refs to display before truncating.
	MaxRefs *int `toml:"max_refs,omitempty"`
	// Maximum depth for impact analysis traversal.
	MaxDepth *int `toml:"max_depth,omitempty"`
}

// MemberInfo holds path and name for generating a workspace manifest.
type MemberInfo struct {
	// Path relative to the workspace root.
	Path string
	// Human-readable display name.
	Name string
}

// LoadWorkspaceConfig loads and parses a inari-workspace.toml from the given path.
// The path should point to the inari-workspace.toml file itself.
func LoadWorkspaceConfig(path string) (*WorkspaceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var cfg WorkspaceConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse inari-workspace.toml: %w", err)
	}

	// Validate schema version.
	if cfg.Workspace.Version != nil && *cfg.Workspace.Version > 1 {
		return nil, fmt.Errorf(
			"unsupported workspace version %d. This version of Inari supports version 1",
			*cfg.Workspace.Version,
		)
	}

	return &cfg, nil
}

// ResolveMemberName returns the display name for a member entry.
// Returns the explicit Name if set, otherwise the last component of Path
// (directory basename).
func ResolveMemberName(entry *WorkspaceMemberEntry) string {
	if entry.Name != nil && *entry.Name != "" {
		return *entry.Name
	}
	base := filepath.Base(entry.Path)
	if base == "" || base == "." || base == "/" {
		return "unknown"
	}
	return base
}

// Validate checks the workspace manifest against its root directory.
//
// Checks:
//   - All member paths exist on disk.
//   - All member paths are subdirectories of the workspace root.
//   - No overlapping member paths (one member is a prefix of another).
//   - No duplicate member names.
func (c *WorkspaceConfig) Validate(workspaceRoot string) error {
	canonicalRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return fmt.Errorf("cannot resolve workspace root %s: %w", workspaceRoot, err)
	}
	canonicalRoot, err = filepath.EvalSymlinks(canonicalRoot)
	if err != nil {
		return fmt.Errorf("cannot canonicalize workspace root %s: %w", workspaceRoot, err)
	}

	type resolvedMember struct {
		name          string
		canonicalPath string
	}

	seenNames := make(map[string]bool)
	resolved := make([]resolvedMember, 0, len(c.Workspace.Members))

	for _, entry := range c.Workspace.Members {
		name := ResolveMemberName(&entry)

		// Check unique names.
		if seenNames[name] {
			return fmt.Errorf("duplicate member name '%s'", name)
		}
		seenNames[name] = true

		// Resolve and check existence.
		memberPath := filepath.Join(workspaceRoot, entry.Path)
		info, err := os.Stat(memberPath)
		if err != nil {
			return fmt.Errorf("member path '%s' does not exist", entry.Path)
		}
		if !info.IsDir() {
			return fmt.Errorf("member path '%s' is not a directory", entry.Path)
		}

		// Canonicalize and check it is under the workspace root.
		canonicalMember, err := filepath.Abs(memberPath)
		if err != nil {
			return fmt.Errorf("cannot resolve member path %s: %w", memberPath, err)
		}
		canonicalMember, err = filepath.EvalSymlinks(canonicalMember)
		if err != nil {
			return fmt.Errorf("cannot canonicalize member path %s: %w", memberPath, err)
		}

		rootWithSep := canonicalRoot + string(filepath.Separator)
		if canonicalMember != canonicalRoot && !strings.HasPrefix(canonicalMember, rootWithSep) {
			return fmt.Errorf(
				"member '%s' is outside workspace root. Set allow_external = true to permit this",
				name,
			)
		}

		resolved = append(resolved, resolvedMember{
			name:          name,
			canonicalPath: canonicalMember,
		})
	}

	// Check no overlapping paths (one is prefix of another).
	for i := 0; i < len(resolved); i++ {
		for j := i + 1; j < len(resolved); j++ {
			pathA := resolved[i].canonicalPath + string(filepath.Separator)
			pathB := resolved[j].canonicalPath + string(filepath.Separator)

			if strings.HasPrefix(pathA, pathB) || strings.HasPrefix(pathB, pathA) ||
				resolved[i].canonicalPath == resolved[j].canonicalPath {
				return fmt.Errorf(
					"member '%s' path overlaps with member '%s'. "+
						"Each member must have a distinct, non-overlapping directory",
					resolved[i].name,
					resolved[j].name,
				)
			}
		}
	}

	return nil
}

// GenerateTOML generates a TOML string for a workspace manifest.
// Used by `inari workspace init` to write the initial file.
func GenerateTOML(name string, members []MemberInfo) string {
	var b strings.Builder

	b.WriteString("# inari-workspace.toml — workspace manifest for multi-project Inari queries.\n")
	b.WriteString("# Place this file at the root of your workspace.\n\n")
	b.WriteString("[workspace]\n")
	b.WriteString(fmt.Sprintf("name = %q\n", name))
	b.WriteString("version = 1\n\n")

	for _, m := range members {
		b.WriteString("[[workspace.members]]\n")
		// Always use forward slashes in the manifest.
		normalized := strings.ReplaceAll(m.Path, "\\", "/")
		b.WriteString(fmt.Sprintf("path = %q\n", normalized))
		b.WriteString(fmt.Sprintf("name = %q\n\n", m.Name))
	}

	return b.String()
}
