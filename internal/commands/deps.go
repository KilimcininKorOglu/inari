// Command implementation for `inari deps`.
//
// `inari deps <symbol>` shows what a symbol depends on.
// Lists direct imports, calls, and type references. Use --depth 2
// for transitive dependencies. Pass a file path to see dependencies
// of all symbols in that file.
package commands

import (
	"github.com/KilimcininKorOglu/inari/internal/output"
	"github.com/spf13/cobra"
)

// newDepsCmd creates the `inari deps` command.
func newDepsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps <symbol>",
		Short: "Show what a symbol depends on",
		Long: `Show what a symbol depends on.

Lists direct imports, calls, and type references. Use --depth 2
for transitive dependencies. Pass a file path to see dependencies
of all symbols in that file.

Examples:
  inari deps PaymentService               — direct dependencies of a class
  inari deps PaymentService --depth 2     — transitive dependencies
  inari deps src/payments/service.ts      — dependencies of a whole file`,
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

				if LooksLikeFilePath(symbolName) {
					filePath := output.NormalizePath(symbolName)
					deps, err := graph.FindFileDeps(filePath, depth)
					if err != nil {
						return err
					}
					if jsonFlag {
						return printJSONOutput("deps", filePath, deps, false, len(deps))
					}
					output.PrintFileDeps(filePath, deps, depth)
					return nil
				}

				deps, err := graph.FindDeps(symbolName, depth)
				if err != nil {
					return err
				}
				if jsonFlag {
					return printJSONOutput("deps", symbolName, deps, false, len(deps))
				}
				output.PrintDeps(symbolName, deps, depth)
				return nil
			})
		},
	}

	cmd.Flags().IntP("depth", "", 1, "Transitive dependency depth (1 = direct only)")
	cmd.Flags().BoolP("json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}
