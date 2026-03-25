// Command implementation for `inari impact` (deprecated).
//
// `inari impact <symbol>` analyses blast radius if a symbol changes.
// This command is deprecated in favour of `inari callers <symbol> --depth N`.
//
// Performs transitive reverse dependency traversal, showing direct callers,
// second-degree dependents, and affected test files.
package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newImpactCmd creates the `inari impact` command (deprecated).
func newImpactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "impact <symbol>",
		Short: "Analyse blast radius if a symbol changes (deprecated)",
		Long: `Analyse blast radius if a symbol changes.

DEPRECATED: Use 'inari callers <symbol> --depth N' instead.

Performs transitive reverse dependency traversal, showing direct callers,
second-degree dependents, and affected test files.

Examples:
  inari impact processPayment             — who breaks if this changes
  inari impact PaymentConfig              — blast radius of config change
  inari impact src/types/payment.ts       — impact of changing a types file`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSingleProjectCommand(cmd, func(root string) error {
				symbolName := args[0]
				jsonFlag, _ := cmd.Flags().GetBool("json")
				depth, _ := cmd.Flags().GetInt("depth")

				// Print deprecation notice to stderr.
				fmt.Fprintf(os.Stderr,
					"Note: 'inari impact' is deprecated. Use 'inari callers %s --depth %d' instead.\n",
					symbolName, depth,
				)

				// Delegate to callers transitive with "impact" command label.
				return runCallersTransitive(symbolName, root, depth, 20, jsonFlag, "impact")
			})
		},
	}

	cmd.Flags().IntP("depth", "", 3, "Maximum traversal depth")
	cmd.Flags().BoolP("json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}
