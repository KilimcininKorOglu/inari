// `inari sketch <symbol>` -- show structural overview of a symbol.
//
// Returns the class/function signature, dependencies, methods with caller
// counts, and type information. Use this before `inari source` to understand
// structure first.
//
// Examples:
//
//	inari sketch PaymentService              -- sketch a class
//	inari sketch PaymentService.processPayment  -- sketch a method
//	inari sketch src/payments/service.ts     -- sketch a whole file
package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/KilimcininKorOglu/inari/internal/core"
	"github.com/KilimcininKorOglu/inari/internal/output"
)

// newSketchCmd creates the cobra command for `inari sketch`.
func newSketchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sketch <symbol>",
		Short: "Show structural overview of a symbol without reading full source",
		Long: "Returns the class/function signature, dependencies, methods with caller\n" +
			"counts, and type information. Use this before `inari source` to understand\n" +
			"structure first.\n\n" +
			"Pass a class name to see its methods, deps, and inheritance.\n" +
			"Pass a method name to see its signature, callers, and callees.\n" +
			"Pass Class.method for qualified lookup.\n" +
			"Pass a file path to see all symbols in that file.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSingleProjectCommand(cmd, func(root string) error {
				jsonFlag, _ := cmd.Flags().GetBool("json")
				limit, _ := cmd.Flags().GetInt("limit")
				noDocs, _ := cmd.Flags().GetBool("no-docs")
				return runSketch(root, args[0], jsonFlag, limit, noDocs)
			})
		},
	}
	cmd.Flags().BoolP("json", "j", false, "Output as JSON instead of human-readable format")
	cmd.Flags().Int("limit", 50, "Maximum number of methods to show (default: all)")
	cmd.Flags().Bool("no-docs", false, "Suppress docstring display in sketch output")
	return cmd
}

// runSketch performs the sketch logic for either a symbol or file path.
func runSketch(projectRoot, symbolArg string, jsonFlag bool, limit int, noDocs bool) error {
	inariDir := filepath.Join(projectRoot, ".inari")
	dbPath := filepath.Join(inariDir, "graph.db")

	if !fileExists(dbPath) {
		return fmt.Errorf("no index found. Run 'inari index' to build one first")
	}

	graph, err := core.Open(dbPath)
	if err != nil {
		return err
	}
	defer graph.Close()

	if LooksLikeFilePath(symbolArg) {
		return runFileSketch(graph, symbolArg, jsonFlag)
	}

	return runSymbolSketch(graph, symbolArg, jsonFlag, limit, noDocs)
}

// runSymbolSketch sketches a single symbol (class, method, interface, etc.).
func runSymbolSketch(
	graph *core.Graph,
	symbolArg string,
	jsonFlag bool,
	limit int,
	noDocs bool,
) error {
	symbol, err := graph.FindSymbol(symbolArg)
	if err != nil {
		return err
	}
	if symbol == nil {
		return fmt.Errorf(
			"symbol '%s' not found in index.\n"+
				"Tip: Check spelling, or use 'inari find \"%s\"' for semantic search",
			symbolArg, symbolArg,
		)
	}

	switch symbol.Kind {
	case "class", "struct":
		return sketchClass(graph, symbol, jsonFlag, limit, noDocs)
	case "method", "function":
		return sketchMethod(graph, symbol, jsonFlag)
	case "interface":
		return sketchInterface(graph, symbol, jsonFlag, limit)
	default:
		return sketchGeneric(symbol, jsonFlag)
	}
}

// sketchClass sketches a class or struct: methods, relationships, caller counts.
func sketchClass(
	graph *core.Graph,
	symbol *core.Symbol,
	jsonFlag bool,
	limit int,
	noDocs bool,
) error {
	methods, err := graph.GetMethods(symbol.ID)
	if err != nil {
		return err
	}

	relationships, err := graph.GetClassRelationships(symbol.ID)
	if err != nil {
		return err
	}

	// Batch-fetch caller counts for all methods.
	methodIDs := make([]string, len(methods))
	for i, m := range methods {
		methodIDs[i] = m.ID
	}
	callerCounts, err := graph.GetCallerCounts(methodIDs)
	if err != nil {
		return err
	}

	if jsonFlag {
		data := map[string]interface{}{
			"symbol":        symbol,
			"methods":       methods,
			"caller_counts": callerCounts,
			"relationships": relationships,
		}
		return output.PrintJSON(output.JsonOutput[interface{}]{
			Command:   "sketch",
			Symbol:    symbol.Name,
			Data:      data,
			Truncated: len(methods) > limit,
			Total:     len(methods),
		})
	}

	output.PrintClassSketch(symbol, methods, callerCounts, relationships, limit, !noDocs)
	return nil
}

// sketchMethod sketches a method or function: outgoing calls and incoming callers.
func sketchMethod(
	graph *core.Graph,
	symbol *core.Symbol,
	jsonFlag bool,
) error {
	outgoingCalls, err := graph.GetOutgoingCalls(symbol.ID)
	if err != nil {
		return err
	}

	incomingCallers, err := graph.GetIncomingCallers(symbol.ID)
	if err != nil {
		return err
	}

	if jsonFlag {
		data := map[string]interface{}{
			"symbol":    symbol,
			"calls":     outgoingCalls,
			"called_by": incomingCallers,
		}
		return output.PrintJSON(output.JsonOutput[interface{}]{
			Command:   "sketch",
			Symbol:    symbol.Name,
			Data:      data,
			Truncated: false,
			Total:     1,
		})
	}

	output.PrintMethodSketch(symbol, outgoingCalls, incomingCallers)
	return nil
}

// sketchInterface sketches an interface: methods and implementors.
func sketchInterface(
	graph *core.Graph,
	symbol *core.Symbol,
	jsonFlag bool,
	limit int,
) error {
	methods, err := graph.GetMethods(symbol.ID)
	if err != nil {
		return err
	}

	implementors, err := graph.GetImplementors(symbol.ID)
	if err != nil {
		return err
	}

	if jsonFlag {
		data := map[string]interface{}{
			"symbol":       symbol,
			"methods":      methods,
			"implementors": implementors,
		}
		return output.PrintJSON(output.JsonOutput[interface{}]{
			Command:   "sketch",
			Symbol:    symbol.Name,
			Data:      data,
			Truncated: len(methods) > limit,
			Total:     len(methods),
		})
	}

	output.PrintInterfaceSketch(symbol, methods, implementors, limit)
	return nil
}

// sketchGeneric sketches a generic symbol (enum, const, type).
func sketchGeneric(symbol *core.Symbol, jsonFlag bool) error {
	if jsonFlag {
		data := map[string]interface{}{
			"symbol": symbol,
		}
		return output.PrintJSON(output.JsonOutput[interface{}]{
			Command:   "sketch",
			Symbol:    symbol.Name,
			Data:      data,
			Truncated: false,
			Total:     1,
		})
	}

	output.PrintGenericSketch(symbol)
	return nil
}

// runFileSketch sketches all symbols in a file.
func runFileSketch(graph *core.Graph, filePath string, jsonFlag bool) error {
	normalizedPath := output.NormalizePath(filePath)
	symbols, err := graph.GetFileSymbols(normalizedPath)
	if err != nil {
		return err
	}

	if len(symbols) == 0 {
		return fmt.Errorf(
			"no symbols found for file '%s'.\n"+
				"Tip: Check the path is relative to the project root. Run 'inari index' if the file is new",
			filePath,
		)
	}

	// Batch-fetch caller counts for all symbols in the file.
	symbolIDs := make([]string, len(symbols))
	for i, s := range symbols {
		symbolIDs[i] = s.ID
	}
	callerCounts, err := graph.GetCallerCounts(symbolIDs)
	if err != nil {
		return err
	}

	if jsonFlag {
		data := map[string]interface{}{
			"file_path":     normalizedPath,
			"symbols":       symbols,
			"caller_counts": callerCounts,
		}
		return output.PrintJSON(output.JsonOutput[interface{}]{
			Command:   "sketch",
			Symbol:    normalizedPath,
			Data:      data,
			Truncated: false,
			Total:     len(symbols),
		})
	}

	output.PrintFileSketch(normalizedPath, symbols, callerCounts)
	return nil
}
