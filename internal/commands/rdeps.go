// Command implementation for `inari rdeps`.
//
// `inari rdeps <symbol>` shows what depends on a symbol (reverse dependencies).
// Critical before any refactor or deletion. Shows all symbols and files
// that depend on the given symbol, grouped by depth level.
package commands

import (
	"fmt"

	"github.com/KilimcininKorOglu/inari/internal/core"
	"github.com/KilimcininKorOglu/inari/internal/output"
	"github.com/spf13/cobra"
)

// newRdepsCmd creates the `inari rdeps` command.
func newRdepsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rdeps <symbol>",
		Short: "Show what depends on a symbol (reverse dependencies)",
		Long: `Show what depends on a symbol (reverse dependencies).

Critical before any refactor or deletion. Shows all symbols and files
that depend on the given symbol, grouped by depth level.

Direct dependents are shown at depth 1. Use --depth 2+ for transitive
reverse dependencies. Test files are listed separately.

Examples:
  inari rdeps PaymentService             — direct reverse dependencies
  inari rdeps processPayment --depth 2   — transitive reverse dependencies
  inari rdeps src/types/payment.ts       — rdeps of all symbols in a file`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSingleProjectCommand(cmd, func(root string) error {
				symbolName := args[0]
				jsonFlag, _ := cmd.Flags().GetBool("json")
				depth, _ := cmd.Flags().GetInt("depth")

				graph, err := openProjectGraph(root)
				if err != nil {
					return err
				}
				defer graph.Close()

				// FindImpact IS reverse dependency traversal -- it finds
				// everything that depends on the given symbol.
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
					return printJSONOutput("rdeps", symbolName, result, false,
						result.TotalAffected+len(result.TestFiles))
				}

				printRdeps(symbolName, result)
				return nil
			})
		},
	}

	cmd.Flags().IntP("depth", "", 1, "Transitive reverse dependency depth (1 = direct only)")
	cmd.Flags().BoolP("json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}

// printRdeps prints reverse dependencies with depth grouping and separated
// test files, using rdeps-specific labels.
func printRdeps(symbolName string, result *core.ImpactResult) {
	fmt.Printf("Reverse dependencies: %s\n", symbolName)
	fmt.Println(output.Separator)

	if len(result.NodesByDepth) == 0 && len(result.TestFiles) == 0 {
		fmt.Println("(no reverse dependencies found)")
		return
	}

	for _, dg := range result.NodesByDepth {
		depthLabel := rdepsDepthLabel(dg.Depth)
		fmt.Printf("%s (%d):\n", depthLabel, len(dg.Nodes))

		maxDisplay := 10
		displayNodes := dg.Nodes
		if len(displayNodes) > maxDisplay {
			displayNodes = displayNodes[:maxDisplay]
		}

		for _, node := range displayNodes {
			path := output.NormalizePath(node.FilePath)
			fmt.Printf("  %-40s%-10s%s\n", node.Name, node.Kind, path)
		}

		if len(dg.Nodes) > maxDisplay {
			fmt.Printf("  ... (%d more)\n", len(dg.Nodes)-maxDisplay)
		}

		fmt.Println()
	}

	if len(result.TestFiles) > 0 {
		fmt.Printf("Test files affected: %d\n", len(result.TestFiles))

		maxDisplay := 10
		displayTests := result.TestFiles
		if len(displayTests) > maxDisplay {
			displayTests = displayTests[:maxDisplay]
		}

		for _, node := range displayTests {
			path := output.NormalizePath(node.FilePath)
			fmt.Printf("  %s\n", path)
		}

		if len(result.TestFiles) > maxDisplay {
			fmt.Printf("  ... (%d more)\n", len(result.TestFiles)-maxDisplay)
		}
	}
}

// rdepsDepthLabel returns a human-readable label for reverse dependency depth.
func rdepsDepthLabel(depth int) string {
	switch depth {
	case 1:
		return "Direct dependents (depth 1)"
	case 2:
		return "Transitive dependents (depth 2)"
	default:
		return fmt.Sprintf("Transitive dependents (depth %d)", depth)
	}
}
