// Package output provides JSON output envelope for all Inari commands.
//
// Every --json output uses JsonOutput[T] as the wrapper, ensuring
// a consistent schema across all commands.
package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// JsonOutput is the standard JSON envelope for all command output.
//
// Example:
//
//	{
//	  "command": "refs",
//	  "symbol": "processPayment",
//	  "data": [...],
//	  "truncated": false,
//	  "total": 11
//	}
type JsonOutput[T any] struct {
	// The command that produced this output (e.g. "sketch", "refs").
	Command string `json:"command"`
	// The symbol name that was queried, if applicable.
	Symbol string `json:"symbol,omitempty"`
	// The command-specific data payload.
	Data T `json:"data"`
	// Whether the output was truncated due to a limit.
	Truncated bool `json:"truncated"`
	// The total count of results before truncation.
	Total int `json:"total"`
}

// PrintJSON marshals a JsonOutput envelope to pretty-printed JSON and writes
// it to stdout. Returns an error if marshalling fails.
func PrintJSON[T any](output JsonOutput[T]) error {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON output: %w", err)
	}
	_, err = fmt.Fprintf(os.Stdout, "%s\n", data)
	return err
}
