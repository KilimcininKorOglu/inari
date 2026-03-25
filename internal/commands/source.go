// Command implementation for `inari source`.
//
// `inari source <symbol>` fetches the full source code of a specific symbol.
// Reads the symbol's file and extracts lines from line_start to line_end,
// displaying them with line numbers.
package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KilimcininKorOglu/inari/internal/output"
	"github.com/spf13/cobra"
)

// sourceLineData holds a single line of source code for JSON output.
type sourceLineData struct {
	LineNumber int    `json:"line_number"`
	Text       string `json:"text"`
}

// sourceJSONData holds the JSON data payload for the source command.
type sourceJSONData struct {
	Lines     []sourceLineData `json:"lines"`
	FilePath  string           `json:"file_path"`
	LineStart uint32           `json:"line_start"`
	LineEnd   uint32           `json:"line_end"`
}

// newSourceCmd creates the `inari source` command.
func newSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "source <symbol>",
		Short: "Fetch full source of a specific symbol",
		Long: `Fetch full source of a specific symbol.

Returns the exact source code of the symbol, including its full definition.
Only call this when ready to read or edit the implementation.

Examples:
  inari source processPayment
  inari source PaymentService.validateCard
  inari source processPayment --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSingleProjectCommand(cmd, func(root string) error {
				symbolName := args[0]
				jsonFlag, _ := cmd.Flags().GetBool("json")

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

				// Read the source file.
				fullPath := filepath.Join(root, symbol.FilePath)
				data, err := os.ReadFile(fullPath)
				if err != nil {
					return fmt.Errorf("failed to read source file '%s': %w", symbol.FilePath, err)
				}

				allLines := strings.Split(string(data), "\n")

				// Extract lines from line_start to line_end (1-indexed).
				startIdx := int(symbol.LineStart) - 1
				endIdx := int(symbol.LineEnd)
				if startIdx < 0 {
					startIdx = 0
				}
				if endIdx > len(allLines) {
					endIdx = len(allLines)
				}
				if startIdx >= endIdx {
					return fmt.Errorf("invalid line range %d-%d for symbol '%s'",
						symbol.LineStart, symbol.LineEnd, symbolName)
				}

				sourceLines := allLines[startIdx:endIdx]

				if jsonFlag {
					lines := make([]sourceLineData, len(sourceLines))
					for i, line := range sourceLines {
						lines[i] = sourceLineData{
							LineNumber: int(symbol.LineStart) + i,
							Text:       strings.TrimRight(line, "\r"),
						}
					}
					return printJSONOutput("source", symbolName, sourceJSONData{
						Lines:     lines,
						FilePath:  symbol.FilePath,
						LineStart: symbol.LineStart,
						LineEnd:   symbol.LineEnd,
					}, false, len(lines))
				}

				// Human-readable output with line numbers.
				path := output.NormalizePath(symbol.FilePath)
				lineRange := output.FormatLineRange(symbol.LineStart, symbol.LineEnd)
				fmt.Printf("%-50s%s  %s:%s\n", symbol.Name, symbol.Kind, path, lineRange)
				fmt.Println(output.Separator)

				// Calculate the width needed for line numbers.
				maxLineNum := int(symbol.LineEnd)
				lineNumWidth := len(fmt.Sprintf("%d", maxLineNum))

				for i, line := range sourceLines {
					lineNum := int(symbol.LineStart) + i
					trimmedLine := strings.TrimRight(line, "\r")
					fmt.Printf("%*d | %s\n", lineNumWidth, lineNum, trimmedLine)
				}

				return nil
			})
		},
	}

	cmd.Flags().BoolP("json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}
