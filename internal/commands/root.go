// Package commands provides the cobra CLI command tree for Inari.
//
// Each subcommand is registered in NewRootCommand and implemented in its
// own file. Context resolution, helpers, and shared types live here too.
package commands

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/KilimcininKorOglu/inari/internal/core"
	"github.com/KilimcininKorOglu/inari/internal/output"
)

// NewRootCommand creates the root cobra command for the Inari CLI.
//
// Registers all subcommands and sets up global persistent flags:
//   - --verbose: enable debug logging
//   - --workspace: query across all workspace members
//   - --project: target a specific workspace member by name
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "inari",
		Short: "Code intelligence CLI for LLM coding agents",
		Long: "Inari builds a local code intelligence index and lets you query " +
			"it efficiently. Use it before editing any non-trivial code to " +
			"understand structure, dependencies, and blast radius.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			verbose, _ := cmd.Flags().GetBool("verbose")
			setupLogging(verbose)
		},
	}

	rootCmd.PersistentFlags().Bool("verbose", false, "Enable verbose logging")
	rootCmd.PersistentFlags().Bool("workspace", false,
		"Query across all workspace members (requires inari-workspace.toml)")
	rootCmd.PersistentFlags().String("project", "",
		"Target a specific workspace member by name")

	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newIndexCmd())
	rootCmd.AddCommand(newSketchCmd())
	rootCmd.AddCommand(newRefsCmd())
	rootCmd.AddCommand(newCallersCmd())
	rootCmd.AddCommand(newDepsCmd())
	rootCmd.AddCommand(newRdepsCmd())
	rootCmd.AddCommand(newImpactCmd())
	rootCmd.AddCommand(newFindCmd())
	rootCmd.AddCommand(newSimilarCmd())
	rootCmd.AddCommand(newSourceCmd())
	rootCmd.AddCommand(newTraceCmd())
	rootCmd.AddCommand(newEntrypointsCmd())
	rootCmd.AddCommand(newMapCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newWorkspaceCmd())

	return rootCmd
}

func setupLogging(verbose bool) {
	if verbose {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.Ltime | log.Lshortfile)
	} else {
		log.SetOutput(io.Discard)
	}
}

// ---------------------------------------------------------------------------
// Shared helpers used by subcommand files
// ---------------------------------------------------------------------------

// openProjectGraph opens the SQLite graph database for a project.
func openProjectGraph(root string) (*core.Graph, error) {
	dbPath := filepath.Join(root, ".inari", "graph.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no index found. Run 'inari index' to build one first")
	}
	return core.Open(dbPath)
}

// readSymbolSource reads the source lines for a symbol from disk.
func readSymbolSource(root string, sym *core.Symbol) (string, error) {
	absPath := filepath.Join(root, sym.FilePath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read source file %s: %w", sym.FilePath, err)
	}
	lines := strings.Split(string(data), "\n")
	start := int(sym.LineStart) - 1
	end := int(sym.LineEnd)
	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n"), nil
}

// printJSONOutput wraps data in the standard JsonOutput envelope and prints it.
func printJSONOutput(command, symbol string, data interface{}, truncated bool, total int) error {
	return output.PrintJSON(output.JsonOutput[interface{}]{
		Command:   command,
		Symbol:    symbol,
		Data:      data,
		Truncated: truncated,
		Total:     total,
	})
}

// runSingleProjectCommand resolves the execution context for a single-project
// command. It reads the --workspace and --project flags, resolves the context,
// extracts the project root, and calls the provided function.
func runSingleProjectCommand(cmd *cobra.Command, fn func(root string) error) error {
	workspaceFlag, _ := cmd.Flags().GetBool("workspace")
	projectFlag, _ := cmd.Flags().GetString("project")
	ctx, err := ResolveContext(workspaceFlag, projectFlag)
	if err != nil {
		return err
	}
	root, err := ProjectRootFromContext(ctx)
	if err != nil {
		return err
	}
	return fn(root)
}

// groupEntrypoints groups entrypoint results by symbol kind for display.
func groupEntrypoints(results []core.EntrypointResult) ([]output.EntrypointGroup, int) {
	groupMap := make(map[string][]output.EntrypointInfo)
	var order []string
	for _, r := range results {
		info := output.EntrypointInfo{
			Name:              r.Symbol.Name,
			FilePath:          r.Symbol.FilePath,
			Kind:              r.Symbol.Kind,
			OutgoingCallCount: r.FanOut,
		}
		if _, exists := groupMap[r.Symbol.Kind]; !exists {
			order = append(order, r.Symbol.Kind)
		}
		groupMap[r.Symbol.Kind] = append(groupMap[r.Symbol.Kind], info)
	}
	groups := make([]output.EntrypointGroup, 0, len(order))
	for _, kind := range order {
		groups = append(groups, output.EntrypointGroup{
			Name:    kind,
			Entries: groupMap[kind],
		})
	}
	return groups, len(results)
}

// countDistinctFiles counts distinct file paths from entrypoint results.
func countDistinctFiles(results []core.EntrypointResult) int {
	seen := make(map[string]struct{})
	for _, r := range results {
		seen[r.Symbol.FilePath] = struct{}{}
	}
	return len(seen)
}
