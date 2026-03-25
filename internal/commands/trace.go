// Command implementation for `inari trace`.
//
// `inari trace <symbol>` shows how requests reach a symbol.
// Traces the call graph backward from the target to find entry points
// (symbols with no incoming calls). Shows every path from an entry
// point through intermediate callers to the target.
package commands

import (
	"fmt"

	"github.com/KilimcininKorOglu/inari/internal/output"
	"github.com/spf13/cobra"
)

// newTraceCmd creates the `inari trace` command.
func newTraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace <symbol>",
		Short: "Show how requests reach a symbol",
		Long: `Show how requests reach a symbol.

Traces the call graph backward from the target to find entry points
(symbols with no incoming calls). Shows every path from an entry
point through intermediate callers to the target.

Use this to understand how a bug is triggered or how a method is
reached from API endpoints, workers, or event handlers.

Examples:
  inari trace processPayment
  inari trace SubscriptionService.processRenewal`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSingleProjectCommand(cmd, func(root string) error {
				symbolName := args[0]
				jsonFlag, _ := cmd.Flags().GetBool("json")
				maxDepth, _ := cmd.Flags().GetInt("max-depth")
				limit, _ := cmd.Flags().GetInt("limit")

				graph, err := openProjectGraph(root)
				if err != nil {
					return err
				}
				defer graph.Close()

				// Find the symbol.
				symbol, err := graph.FindSymbol(symbolName)
				if err != nil {
					return err
				}
				if symbol == nil {
					return fmt.Errorf(
						"Symbol '%s' not found in index.\n"+
							"Tip: Check spelling, or use 'inari find \"%s\"' for semantic search.",
						symbolName, symbolName,
					)
				}

				// Find call paths from entry points to the target.
				result, err := graph.FindCallPaths(symbol.ID, symbol.Name, maxDepth)
				if err != nil {
					return err
				}

				total := len(result.Paths)
				truncated := total > limit

				if truncated {
					result.Paths = result.Paths[:limit]
				}

				if jsonFlag {
					return printJSONOutput("trace", symbolName, result, truncated, total)
				}

				output.PrintTrace(symbolName, result, total, truncated)
				return nil
			})
		},
	}

	cmd.Flags().IntP("max-depth", "", 10, "Maximum call chain depth to search")
	cmd.Flags().IntP("limit", "", 20, "Maximum number of call paths to display")
	cmd.Flags().BoolP("json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}
