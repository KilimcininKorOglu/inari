// Command implementation for `inari find`.
//
// `inari find <query>` finds code by intent using full-text search.
// Searches the symbol index for symbols matching a natural-language query.
// Uses FTS5 with BM25 ranking to return the most relevant results.
//
// In workspace mode, performs sequential FTS5 queries per member
// and merges results by score.
package commands

import (
	"path/filepath"
	"sort"

	"github.com/KilimcininKorOglu/inari/internal/core"
	"github.com/KilimcininKorOglu/inari/internal/output"
	"github.com/spf13/cobra"
)

// newFindCmd creates the `inari find` command.
func newFindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Find code by intent using full-text search",
		Long: `Find code by intent using full-text search.

Searches the symbol index for symbols matching a natural-language query.
Uses FTS5 with BM25 ranking to return the most relevant results.

In workspace mode, fans out to all members and merges results by score.

Examples:
  inari find "handles authentication errors"
  inari find "payment processing" --kind method
  inari find "validates user input" --limit 5 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSingleProjectCommand(cmd, func(root string) error {
				query := args[0]
				jsonFlag, _ := cmd.Flags().GetBool("json")
				kindFilter, _ := cmd.Flags().GetString("kind")
				limit, _ := cmd.Flags().GetInt("limit")

				dbPath := filepath.Join(root, ".inari", "graph.db")
				searcher, err := core.OpenSearcher(dbPath)
				if err != nil {
					return err
				}
				defer searcher.Close()

				results, err := searcher.Search(query, limit, kindFilter)
				if err != nil {
					return err
				}

				if jsonFlag {
					return printJSONOutput("find", "", results, false, len(results))
				}

				output.PrintSearchResults(results, query)
				return nil
			})
		},
	}

	cmd.Flags().StringP("kind", "", "", "Filter by symbol kind: function, class, method, interface")
	cmd.Flags().IntP("limit", "", 10, "Maximum number of results to show")
	cmd.Flags().BoolP("json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}

// runFindWorkspace runs find across all workspace members (sequential FTS5 per member).
// This is called from workspace-aware command dispatching.
func runFindWorkspace(query, kind string, limit int, jsonFlag bool, members []workspaceMemberRef) error {
	var allResults []output.WorkspaceSearchResult

	for _, m := range members {
		dbPath := filepath.Join(m.root, ".inari", "graph.db")
		searcher, err := core.OpenSearcher(dbPath)
		if err != nil {
			continue
		}

		results, err := searcher.Search(query, limit, kind)
		searcher.Close()
		if err != nil {
			continue
		}

		for _, r := range results {
			allResults = append(allResults, output.WorkspaceSearchResult{
				Project: m.name,
				Result:  r,
			})
		}
	}

	// Sort by score descending, truncate to limit.
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Result.Score > allResults[j].Result.Score
	})
	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	total := len(allResults)

	if jsonFlag {
		return printJSONOutput("find", "", allResults, false, total)
	}

	output.PrintWorkspaceSearchResults(allResults, query)
	return nil
}

// workspaceMemberRef is a lightweight reference to a workspace member for
// workspace-level queries.
type workspaceMemberRef struct {
	name string
	root string
}
