// Context resolution for Inari CLI commands.
//
// Determines whether the CLI operates in single-project mode (default)
// or workspace mode (--workspace / --project flags). Mirrors the Rust
// Context enum and resolve_context function from main.rs.
package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/KilimcininKorOglu/inari/internal/config"
)

// Context is the resolved execution context: single project or workspace.
type Context interface {
	// IsSingleProject returns true when operating on a single project root.
	IsSingleProject() bool
	// IsWorkspace returns true when operating across a workspace manifest.
	IsWorkspace() bool
}

// SingleProjectContext is the standard single-project mode.
// CWD has a .inari/ directory (or will create one via `inari init`).
type SingleProjectContext struct {
	// Root is the absolute path to the project root.
	Root string
}

// IsSingleProject returns true.
func (c *SingleProjectContext) IsSingleProject() bool { return true }

// IsWorkspace returns false.
func (c *SingleProjectContext) IsWorkspace() bool { return false }

// WorkspaceContext is the workspace mode.
// An inari-workspace.toml was found (via --workspace or --project flags).
type WorkspaceContext struct {
	// ManifestPath is the absolute path to the inari-workspace.toml file.
	ManifestPath string
	// WorkspaceRoot is the directory containing the manifest.
	WorkspaceRoot string
	// Config is the parsed workspace configuration.
	Config *config.WorkspaceConfig
}

// IsSingleProject returns false.
func (c *WorkspaceContext) IsSingleProject() bool { return false }

// IsWorkspace returns true.
func (c *WorkspaceContext) IsWorkspace() bool { return true }

// ResolveContext resolves the execution context based on CLI flags.
//
// Rules:
//   - No flags: returns SingleProjectContext with CWD as root.
//   - --project <name>: finds workspace manifest, then targets the named member
//     as a SingleProjectContext (root = member directory).
//   - --workspace: finds workspace manifest, returns WorkspaceContext.
func ResolveContext(workspaceFlag bool, projectFlag string) (Context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// --project implies workspace context; resolve the named member.
	if projectFlag != "" {
		manifestPath, err := FindWorkspaceManifest(cwd)
		if err != nil {
			return nil, err
		}

		workspaceRoot := filepath.Dir(manifestPath)
		cfg, err := config.LoadWorkspaceConfig(manifestPath)
		if err != nil {
			return nil, err
		}
		if err := cfg.Validate(workspaceRoot); err != nil {
			return nil, err
		}

		// Find the named member.
		for _, member := range cfg.Workspace.Members {
			name := config.ResolveMemberName(&member)
			if name == projectFlag {
				memberRoot := filepath.Join(workspaceRoot, member.Path)
				return &SingleProjectContext{Root: memberRoot}, nil
			}
		}

		// Member not found; build list for error message.
		available := make([]string, 0, len(cfg.Workspace.Members))
		for _, m := range cfg.Workspace.Members {
			available = append(available, config.ResolveMemberName(&m))
		}
		return nil, fmt.Errorf(
			"project '%s' not found in workspace. Available: %s",
			projectFlag,
			joinNames(available),
		)
	}

	// --workspace flag: return full workspace context.
	if workspaceFlag {
		manifestPath, err := FindWorkspaceManifest(cwd)
		if err != nil {
			return nil, err
		}

		workspaceRoot := filepath.Dir(manifestPath)
		cfg, err := config.LoadWorkspaceConfig(manifestPath)
		if err != nil {
			return nil, err
		}
		if err := cfg.Validate(workspaceRoot); err != nil {
			return nil, err
		}

		return &WorkspaceContext{
			ManifestPath:  manifestPath,
			WorkspaceRoot: workspaceRoot,
			Config:        cfg,
		}, nil
	}

	// Default: single project with CWD.
	return &SingleProjectContext{Root: cwd}, nil
}

// ProjectRootFromContext extracts a single project root from the context.
//
// For SingleProjectContext, returns the root directly.
// For WorkspaceContext, returns an error with guidance to use --project.
func ProjectRootFromContext(ctx Context) (string, error) {
	switch c := ctx.(type) {
	case *SingleProjectContext:
		return c.Root, nil
	case *WorkspaceContext:
		return "", fmt.Errorf(
			"this command operates on a single project. " +
				"Use --project <name> to target a workspace member",
		)
	default:
		return "", fmt.Errorf("unknown context type")
	}
}

// FindWorkspaceManifest walks upward from startDir looking for
// inari-workspace.toml. Returns the absolute path to the manifest,
// or an error if none is found.
func FindWorkspaceManifest(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve start directory: %w", err)
	}

	for {
		candidate := filepath.Join(dir, "inari-workspace.toml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding a manifest.
			return "", fmt.Errorf(
				"no inari-workspace.toml found in %s or any parent directory. "+
					"Run 'inari workspace init' to create one",
				startDir,
			)
		}
		dir = parent
	}
}

// joinNames joins a slice of names with ", " for display in error messages.
func joinNames(names []string) string {
	if len(names) == 0 {
		return "(none)"
	}
	result := names[0]
	for i := 1; i < len(names); i++ {
		result += ", " + names[i]
	}
	return result
}
