// Command implementation for `inari refs` and `inari callers`.
//
// `inari refs <symbol>` finds all references to a symbol across the codebase.
// Returns call sites, imports, type annotations, and other references.
//
// `inari callers <symbol>` is shorthand for refs --kind calls. With --depth 2+
// it delegates to transitive impact analysis.
package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KilimcininKorOglu/inari/internal/core"
	"github.com/KilimcininKorOglu/inari/internal/output"
	"github.com/spf13/cobra"
)

// newRefsCmd creates the `inari refs` command.
func newRefsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refs <symbol>",
		Short: "Find all references to a symbol",
		Long: `Find all references to a symbol across the codebase.

Returns all call sites, imports, type annotations, and other references.
Use before changing a function signature to find all callers.

For class symbols, references are grouped by kind (instantiated, extended,
imported, used as type). For functions/methods, a flat list is shown.

Pass a file path to see references to all symbols in that file.

Examples:
  inari refs processPayment              — all references to a function
  inari refs PaymentService              — grouped references to a class
  inari refs PaymentService --kind calls — only call sites
  inari refs src/payments/service.ts     — all refs to symbols in a file`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSingleProjectCommand(cmd, func(root string) error {
				symbolName := args[0]
				jsonFlag, _ := cmd.Flags().GetBool("json")
				kindFilter, _ := cmd.Flags().GetString("kind")
				limit, _ := cmd.Flags().GetInt("limit")
				contextSize, _ := cmd.Flags().GetInt("context")

				graph, err := openProjectGraph(root)
				if err != nil {
					return err
				}
				defer graph.Close()

				if LooksLikeFilePath(symbolName) {
					return runFileRefs(symbolName, kindFilter, limit, contextSize, jsonFlag, graph, root)
				}

				return runSymbolRefs(symbolName, kindFilter, limit, contextSize, jsonFlag, graph, root)
			})
		},
	}

	cmd.Flags().StringP("kind", "", "", "Filter by edge kind: calls, imports, extends, implements, instantiates, references")
	cmd.Flags().IntP("limit", "", 20, "Maximum number of references to show")
	cmd.Flags().IntP("context", "c", 0, "Lines of surrounding code context to show per reference")
	cmd.Flags().BoolP("json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}

// newCallersCmd creates the `inari callers` command (alias for refs --kind calls).
func newCallersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "callers <symbol>",
		Short: "Find all callers of a symbol",
		Long: `Find all callers of a symbol (shorthand for refs --kind calls).

With --depth 1 (default), shows direct callers only.
With --depth 2+, performs transitive impact analysis to find indirect callers.

Examples:
  inari callers processPayment             — direct callers
  inari callers processPayment --depth 2   — transitive callers`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSingleProjectCommand(cmd, func(root string) error {
				symbolName := args[0]
				jsonFlag, _ := cmd.Flags().GetBool("json")
				depth, _ := cmd.Flags().GetInt("depth")
				limit, _ := cmd.Flags().GetInt("limit")
				contextSize, _ := cmd.Flags().GetInt("context")

				if depth > 1 {
					return runCallersTransitive(symbolName, root, depth, limit, jsonFlag, "callers")
				}

				// Depth 1: delegate to refs with kind=calls.
				graph, err := openProjectGraph(root)
				if err != nil {
					return err
				}
				defer graph.Close()

				return runSymbolRefs(symbolName, "calls", limit, contextSize, jsonFlag, graph, root)
			})
		},
	}

	cmd.Flags().IntP("depth", "", 1, "Traversal depth (1 = direct callers, 2+ = transitive)")
	cmd.Flags().IntP("limit", "", 20, "Maximum callers to show (only at depth 1)")
	cmd.Flags().IntP("context", "c", 0, "Lines of surrounding code context per caller (only at depth 1)")
	cmd.Flags().BoolP("json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}

// runSymbolRefs finds refs for a single symbol.
func runSymbolRefs(symbolName, kind string, limit, contextSize int, jsonFlag bool, graph *core.Graph, projectRoot string) error {
	var kinds []string
	if kind != "" {
		kinds = []string{kind}
	}

	// Check if this is a class-like symbol for grouped output.
	isClass, _ := graph.IsClassLike(symbolName)

	if isClass && len(kinds) == 0 {
		// Grouped output for class symbols.
		groups, total, err := graph.FindRefsGrouped(symbolName, limit)
		if err != nil {
			return err
		}

		// Enrich all refs in all groups with source snippets.
		for i := range groups {
			enrichRefsWithSnippets(groups[i].Refs, projectRoot, contextSize)
		}

		if jsonFlag {
			return printJSONOutput("refs", symbolName, groups, total > limit, total)
		}

		// Convert core.RefsGroup to output.ReferenceGroup for formatter.
		outGroups := make([]output.ReferenceGroup, len(groups))
		for i, g := range groups {
			outGroups[i] = output.ReferenceGroup{
				Kind: g.Kind,
				Refs: g.Refs,
			}
		}
		output.PrintRefsGrouped(symbolName, outGroups, total)
		return nil
	}

	// Flat output for functions/methods or filtered queries.
	refs, total, err := graph.FindRefs(symbolName, kinds, limit)
	if err != nil {
		return err
	}

	// Enrich refs with source snippets.
	enrichRefsWithSnippets(refs, projectRoot, contextSize)

	if jsonFlag {
		return printJSONOutput("refs", symbolName, refs, total > limit, total)
	}

	output.PrintRefs(symbolName, refs, total)
	return nil
}

// runFileRefs finds refs to all symbols in a file.
func runFileRefs(symbolName, kind string, limit, contextSize int, jsonFlag bool, graph *core.Graph, projectRoot string) error {
	filePath := output.NormalizePath(symbolName)
	var kinds []string
	if kind != "" {
		kinds = []string{kind}
	}

	refs, total, err := graph.FindFileRefs(filePath, kinds, limit)
	if err != nil {
		return err
	}

	// Enrich refs with source snippets.
	enrichRefsWithSnippets(refs, projectRoot, contextSize)

	if jsonFlag {
		return printJSONOutput("refs", filePath, refs, total > limit, total)
	}

	output.PrintFileRefs(filePath, refs, total)
	return nil
}

// enrichRefsWithSnippets reads source files and adds snippet lines to references.
//
// Groups refs by file path to avoid reading the same file multiple times.
// Sets SnippetLine to the source line at the reference location (always).
// Sets Snippet to surrounding context lines (only when contextLines > 0).
// Gracefully degrades: if a file cannot be read, leaves fields as nil.
func enrichRefsWithSnippets(refs []core.Reference, projectRoot string, contextLines int) {
	// Group ref indices by file_path.
	byFile := make(map[string][]int)
	for i, r := range refs {
		byFile[r.FilePath] = append(byFile[r.FilePath], i)
	}

	for filePath, indices := range byFile {
		fullPath := filepath.Join(projectRoot, filePath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue // graceful degradation
		}

		lines := strings.Split(string(data), "\n")

		for _, idx := range indices {
			r := &refs[idx]
			if r.Line == nil {
				continue
			}
			lineNum := int(*r.Line)
			lineIdx := lineNum - 1
			if lineIdx < 0 || lineIdx >= len(lines) {
				continue
			}

			// Always set snippet_line to the actual source line.
			trimmed := strings.TrimRight(lines[lineIdx], "\r\n")
			r.SnippetLine = &trimmed

			// Set multi-line context if requested.
			if contextLines > 0 {
				start := lineIdx - contextLines
				if start < 0 {
					start = 0
				}
				end := lineIdx + contextLines + 1
				if end > len(lines) {
					end = len(lines)
				}
				ctx := make([]string, 0, end-start)
				for _, l := range lines[start:end] {
					ctx = append(ctx, strings.TrimRight(l, "\r\n"))
				}
				r.Snippet = ctx
			}
		}
	}
}

// runCallersTransitive runs transitive caller analysis (depth > 1) using
// the impact graph query. The commandLabel is used in JSON output to identify
// the command (e.g. "callers" or "impact" for backward compatibility).
func runCallersTransitive(symbolName, projectRoot string, depth, limit int, jsonFlag bool, commandLabel string) error {
	graph, err := openProjectGraph(projectRoot)
	if err != nil {
		return err
	}
	defer graph.Close()

	var result *core.ImpactResult
	if LooksLikeFilePath(symbolName) {
		filePath := output.NormalizePath(symbolName)
		result, err = graph.FindFileImpact(filePath, depth)
	} else {
		result, err = graph.FindImpact(symbolName, depth)
	}
	if err != nil {
		return err
	}

	if jsonFlag {
		return printJSONOutput(commandLabel, symbolName, result, false,
			result.TotalAffected+len(result.TestFiles))
	}

	output.PrintImpact(symbolName, result)
	return nil
}

// printNoRefsFound prints the empty-result message for refs commands.
func printNoRefsFound(symbolName string, jsonFlag bool) error {
	if jsonFlag {
		return printJSONOutput("refs", symbolName, []interface{}{}, false, 0)
	}
	fmt.Printf("No references to '%s' found.\n", symbolName)
	return nil
}
