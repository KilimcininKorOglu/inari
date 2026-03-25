// Command implementation for `inari similar`.
//
// `inari similar <symbol>` finds structurally similar symbols using
// embedding text and FTS5 search. Builds a rich text representation
// of the target symbol (including callers, callees, and structural
// context) and searches the index for similar symbols.
package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/KilimcininKorOglu/inari/internal/core"
	"github.com/KilimcininKorOglu/inari/internal/output"
	"github.com/spf13/cobra"
)

// newSimilarCmd creates the `inari similar` command.
func newSimilarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "similar <symbol>",
		Short: "Find structurally similar symbols",
		Long: `Find structurally similar symbols.

Uses embeddings to find symbols with similar structure or semantics.
Builds a text representation of the target symbol including its callers,
callees, and structural context, then searches the FTS5 index for matches.

Useful for discovering existing implementations before writing new code.

Examples:
  inari similar processPayment
  inari similar PaymentService --kind method
  inari similar validateInput --limit 5 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSingleProjectCommand(cmd, func(root string) error {
				symbolName := args[0]
				jsonFlag, _ := cmd.Flags().GetBool("json")
				kindFilter, _ := cmd.Flags().GetString("kind")
				limit, _ := cmd.Flags().GetInt("limit")

				dbPath := filepath.Join(root, ".inari", "graph.db")

				graph, err := openProjectGraph(root)
				if err != nil {
					return err
				}
				defer graph.Close()

				searcher, err := core.OpenSearcher(dbPath)
				if err != nil {
					return err
				}
				defer searcher.Close()

				// Find the target symbol.
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

				// Get callers and callees from graph.
				callerInfos, err := graph.GetIncomingCallers(symbol.ID)
				if err != nil {
					return fmt.Errorf("failed to get callers: %w", err)
				}
				callerNames := make([]string, len(callerInfos))
				for i, c := range callerInfos {
					callerNames[i] = c.Name
				}

				calleeIDs, err := graph.GetOutgoingCalls(symbol.ID)
				if err != nil {
					return fmt.Errorf("failed to get callees: %w", err)
				}
				// Extract names from callee IDs (format: "file::name::kind").
				calleeNames := make([]string, 0, len(calleeIDs))
				for _, id := range calleeIDs {
					name := symbolNameFromID(id)
					if name != "" {
						calleeNames = append(calleeNames, name)
					}
				}

				// Get importance score for richer embedding.
				callerCount, _ := graph.GetCallerCount(symbol.ID)
				importance := 0.0
				if callerCount > 10 {
					importance = 0.8
				} else if callerCount > 3 {
					importance = 0.5
				} else if callerCount > 0 {
					importance = 0.2
				}

				// Build embedding text for the target symbol.
				embeddingText := core.BuildEmbeddingText(symbol, callerNames, calleeNames, importance)

				// Extract key search terms from the embedding text.
				keyTerms := extractKeyTerms(embeddingText)
				if keyTerms == "" {
					return fmt.Errorf("could not extract meaningful search terms for '%s'", symbolName)
				}

				// Search FTS5 with double the limit to have room after filtering.
				searchLimit := limit * 2
				if searchLimit < 10 {
					searchLimit = 10
				}
				results, err := searcher.Search(keyTerms, searchLimit, kindFilter)
				if err != nil {
					return fmt.Errorf("similarity search failed: %w", err)
				}

				// Filter out the original symbol from results.
				filtered := make([]core.SearchResult, 0, len(results))
				for _, r := range results {
					if r.ID != symbol.ID && r.Name != symbolName {
						filtered = append(filtered, r)
					}
				}

				// Truncate to limit.
				if len(filtered) > limit {
					filtered = filtered[:limit]
				}

				if jsonFlag {
					return printJSONOutput("similar", symbolName, filtered, false, len(filtered))
				}

				printSimilarResults(symbolName, filtered)
				return nil
			})
		},
	}

	cmd.Flags().StringP("kind", "", "", "Filter by symbol kind: function, class, method")
	cmd.Flags().IntP("limit", "", 10, "Maximum number of results to show")
	cmd.Flags().BoolP("json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}

// extractKeyTerms extracts meaningful search terms from embedding text.
//
// Takes the first few sections of the embedding text (separated by " | ")
// and extracts individual words, filtering out very short tokens and
// structural prefixes.
func extractKeyTerms(embeddingText string) string {
	sections := strings.Split(embeddingText, " | ")

	// Use at most the first 4 sections for key terms (kind+name, split name,
	// signature, docstring).
	maxSections := 4
	if len(sections) < maxSections {
		maxSections = len(sections)
	}

	var terms []string
	seen := make(map[string]bool)
	skipPrefixes := []string{"importance", "path", "called-by", "calls", "in"}

	for _, section := range sections[:maxSections] {
		// Skip structural sections like "importance high core" or "path payments services".
		shouldSkip := false
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(section, prefix+" ") {
				shouldSkip = true
				break
			}
		}
		if shouldSkip {
			continue
		}

		words := strings.Fields(section)
		for _, w := range words {
			lower := strings.ToLower(w)
			// Skip very short words and common prefixes.
			if len(lower) < 3 {
				continue
			}
			if seen[lower] {
				continue
			}
			seen[lower] = true
			terms = append(terms, lower)
		}
	}

	// Limit to 8 terms to keep the query focused.
	if len(terms) > 8 {
		terms = terms[:8]
	}

	return strings.Join(terms, " ")
}

// symbolNameFromID extracts the symbol name from a symbol ID.
// Symbol IDs follow the format "file_path::name::kind".
func symbolNameFromID(id string) string {
	parts := strings.Split(id, "::")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return ""
}

// printSimilarResults prints similar symbol results with scores.
func printSimilarResults(symbolName string, results []core.SearchResult) {
	fmt.Printf("Symbols similar to: %s\n", symbolName)
	fmt.Println(output.Separator)

	if len(results) == 0 {
		fmt.Println("(no similar symbols found)")
		return
	}

	for _, r := range results {
		path := output.NormalizePath(r.FilePath)
		location := fmt.Sprintf("%s:%d", path, r.LineStart)
		fmt.Printf("%.2f  %-40s%-36s  %s\n", r.Score, r.Name, location, r.Kind)
	}
}
