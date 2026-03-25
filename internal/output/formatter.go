// Package output provides human-readable output formatting for all Inari commands.
//
// Rules:
//   - Separator line uses U+2500 (box drawing light horizontal), never '-' or '='
//   - File paths always use forward slashes, even on Windows
//   - Line ranges formatted as "start-end"
//   - Caller counts in square brackets: "[11 callers]", "[internal]"
//   - Similarity scores always 2 decimal places: "0.91"
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/KilimcininKorOglu/inari/internal/core"
)

// Separator is the line used between header and body in all command output.
const Separator = "──────────────────────────────────────────────────────────────────────────────"

// separatorCharCount is the number of runes in the Separator constant.
var separatorCharCount = utf8.RuneCountInString(Separator)

// ---------------------------------------------------------------------------
// Types used by formatter functions (defined here for the output package).
// ---------------------------------------------------------------------------

// MapStats holds aggregate statistics for a project or workspace map.
type MapStats struct {
	FileCount   int      `json:"file_count"`
	SymbolCount int      `json:"symbol_count"`
	EdgeCount   int      `json:"edge_count"`
	Languages   []string `json:"languages"`
}

// CoreSymbol represents a high-traffic symbol in the map output.
type CoreSymbol struct {
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	FilePath    string  `json:"file_path"`
	CallerCount int     `json:"caller_count"`
	Project     *string `json:"project,omitempty"`
}

// DirStats holds directory-level counts for the architecture section.
type DirStats struct {
	Directory   string `json:"directory"`
	FileCount   int    `json:"file_count"`
	SymbolCount int    `json:"symbol_count"`
}

// EntrypointInfo describes a single entry point symbol.
type EntrypointInfo struct {
	Name              string `json:"name"`
	FilePath          string `json:"file_path"`
	Kind              string `json:"kind"`
	MethodCount       int    `json:"method_count"`
	OutgoingCallCount int    `json:"outgoing_call_count"`
}

// StatusData holds index health information for a single project.
type StatusData struct {
	IndexExists         bool    `json:"index_exists"`
	SearchAvailable     bool    `json:"search_available"`
	SymbolCount         int     `json:"symbol_count"`
	FileCount           int     `json:"file_count"`
	EdgeCount           int     `json:"edge_count"`
	LastIndexedAt       *int64  `json:"last_indexed_at"`
	LastIndexedRelative *string `json:"last_indexed_relative"`
}

// WorkspaceStatusData holds workspace-wide status information.
type WorkspaceStatusData struct {
	WorkspaceName string             `json:"workspace_name"`
	Members       []MemberStatusData `json:"members"`
	Totals        StatusData         `json:"totals"`
}

// MemberStatusData holds per-member status for workspace status output.
type MemberStatusData struct {
	Name   string     `json:"name"`
	Status StatusData `json:"status"`
}

// MemberListEntry holds per-member info for workspace list output.
type MemberListEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Status      string `json:"status"`
	FileCount   int    `json:"file_count"`
	SymbolCount int    `json:"symbol_count"`
}

// IncrementalStats holds the result of an incremental index operation.
type IncrementalStats struct {
	Modified     []string `json:"modified"`
	Added        []string `json:"added"`
	Deleted      []string `json:"deleted"`
	DurationSecs float64  `json:"duration_secs"`
}

// WorkspaceRef is a reference tagged with its source project name.
type WorkspaceRef struct {
	Project   string         `json:"project"`
	Reference core.Reference `json:"reference"`
}

// WorkspaceSearchResult is a search result tagged with its source project.
type WorkspaceSearchResult struct {
	Project string            `json:"project"`
	Result  core.SearchResult `json:"result"`
}

// ---------------------------------------------------------------------------
// Path and range helpers.
// ---------------------------------------------------------------------------

// NormalizePath converts backslashes to forward slashes for display.
func NormalizePath(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

// FormatLineRange returns "start-end" or just "start" if start == end.
func FormatLineRange(start, end uint32) string {
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}

// ---------------------------------------------------------------------------
// printHeader prints the standard symbol header: name  kind  file:line_range
// ---------------------------------------------------------------------------

func printHeader(symbol *core.Symbol) {
	path := NormalizePath(symbol.FilePath)
	lineRange := FormatLineRange(symbol.LineStart, symbol.LineEnd)
	fmt.Printf("%-50s%s  %s:%s\n", symbol.Name, symbol.Kind, path, lineRange)
	fmt.Println(Separator)
}

// ---------------------------------------------------------------------------
// PrintClassSketch prints a class sketch with methods, fields, and deps.
// ---------------------------------------------------------------------------

func PrintClassSketch(
	symbol *core.Symbol,
	methods []core.Symbol,
	callerCounts map[string]int,
	relationships *core.ClassRelationships,
	limit int,
	showDocs bool,
) {
	printHeader(symbol)

	// Dependencies line.
	if len(relationships.Dependencies) > 0 {
		fmt.Printf("deps:     %s\n", strings.Join(relationships.Dependencies, ", "))
	}

	// Extends line.
	if len(relationships.Extends) > 0 {
		fmt.Printf("extends:  %s\n", strings.Join(relationships.Extends, ", "))
	}

	// Implements line.
	if len(relationships.Implements) > 0 {
		fmt.Printf("implements: %s\n", strings.Join(relationships.Implements, ", "))
	}

	// Partition into methods and fields.
	var methodSyms []*core.Symbol
	var fieldSyms []*core.Symbol
	for i := range methods {
		m := &methods[i]
		if m.Kind == "method" || m.Kind == "function" {
			methodSyms = append(methodSyms, m)
		} else {
			fieldSyms = append(fieldSyms, m)
		}
	}

	// Methods section.
	if len(methodSyms) > 0 {
		fmt.Println()
		fmt.Println("methods:")

		displayMethods := methodSyms
		if len(displayMethods) > limit {
			displayMethods = displayMethods[:limit]
		}

		for _, method := range displayMethods {
			// Show first line of docstring if available and docs are enabled.
			if showDocs && method.Docstring != nil {
				firstLine := strings.SplitN(*method.Docstring, "\n", 2)[0]
				clean := strings.TrimSpace(firstLine)
				clean = strings.TrimLeft(clean, "/")
				clean = strings.TrimLeft(clean, "*")
				clean = strings.TrimSpace(clean)
				if clean != "" {
					fmt.Printf("  /// %s\n", clean)
				}
			}

			sig := methodDisplayLine(method)
			count := callerCounts[method.ID]
			var countLabel string
			if count > 0 {
				suffix := "s"
				if count == 1 {
					suffix = ""
				}
				countLabel = fmt.Sprintf("[%d caller%s]", count, suffix)
			} else {
				countLabel = "[internal]"
			}

			sigLen := utf8.RuneCountInString(sig)
			countLen := utf8.RuneCountInString(countLabel)
			padding := separatorCharCount - 2 - sigLen - countLen
			if padding < 0 {
				padding = 0
			}
			fmt.Printf("  %s%s%s\n", sig, strings.Repeat(" ", padding), countLabel)
		}

		if len(methodSyms) > limit {
			fmt.Printf("  ... %d more (use --limit to show more)\n", len(methodSyms)-limit)
		}
	}

	// Fields section (properties only).
	var propSyms []*core.Symbol
	for _, f := range fieldSyms {
		if f.Kind == "property" {
			propSyms = append(propSyms, f)
		}
	}

	if len(propSyms) > 0 {
		fmt.Println()
		fmt.Println("fields:")
		for _, field := range propSyms {
			sig := field.Name
			if field.Signature != nil {
				sig = *field.Signature
			}
			fmt.Printf("  %s\n", sig)
		}
	}
}

// ---------------------------------------------------------------------------
// PrintMethodSketch prints a method/function sketch with calls and callers.
// ---------------------------------------------------------------------------

func PrintMethodSketch(
	symbol *core.Symbol,
	outgoingCalls []string,
	incomingCallers []core.CallerInfo,
) {
	printHeader(symbol)

	// Modifiers line.
	modifiers := extractModifiers(symbol.Metadata)
	if len(modifiers) > 0 {
		fmt.Println(strings.Join(modifiers, " "))
	}

	// Signature line.
	if symbol.Signature != nil {
		fmt.Printf("signature:  %s\n", *symbol.Signature)
	}

	// Calls line.
	if len(outgoingCalls) > 0 {
		fmt.Printf("calls:      %s\n", strings.Join(outgoingCalls, ", "))
	}

	// Called by line.
	if len(incomingCallers) > 0 {
		parts := make([]string, 0, len(incomingCallers))
		for _, c := range incomingCallers {
			if c.Count > 1 {
				parts = append(parts, fmt.Sprintf("%s [x%d]", c.Name, c.Count))
			} else {
				parts = append(parts, c.Name)
			}
		}
		fmt.Printf("called by:  %s\n", strings.Join(parts, ", "))
	}
}

// ---------------------------------------------------------------------------
// PrintInterfaceSketch prints an interface sketch with methods and implementors.
// ---------------------------------------------------------------------------

func PrintInterfaceSketch(
	symbol *core.Symbol,
	methods []core.Symbol,
	implementors []string,
	limit int,
) {
	printHeader(symbol)

	// Implemented by.
	if len(implementors) > 0 {
		fmt.Printf("implemented by:  %s\n", strings.Join(implementors, ", "))
	}

	// Methods section.
	if len(methods) > 0 {
		fmt.Println()
		fmt.Println("methods:")

		displayMethods := methods
		if len(displayMethods) > limit {
			displayMethods = displayMethods[:limit]
		}

		for i := range displayMethods {
			sig := methodDisplayLine(&displayMethods[i])
			fmt.Printf("  %s\n", sig)
		}

		if len(methods) > limit {
			fmt.Printf("  ... %d more (use --limit to show more)\n", len(methods)-limit)
		}
	}
}

// ---------------------------------------------------------------------------
// PrintGenericSketch prints a fallback sketch for enum, const, type, struct.
// ---------------------------------------------------------------------------

func PrintGenericSketch(symbol *core.Symbol) {
	printHeader(symbol)

	if symbol.Signature != nil {
		fmt.Printf("signature:  %s\n", *symbol.Signature)
	}
}

// ---------------------------------------------------------------------------
// PrintFileSketch prints all symbols in a file.
// ---------------------------------------------------------------------------

func PrintFileSketch(
	filePath string,
	symbols []core.Symbol,
	callerCounts map[string]int,
) {
	path := NormalizePath(filePath)
	fmt.Println(path)
	fmt.Println(Separator)

	for _, sym := range symbols {
		lineRange := FormatLineRange(sym.LineStart, sym.LineEnd)
		count := callerCounts[sym.ID]
		var countLabel string
		if count > 0 {
			suffix := "s"
			if count == 1 {
				suffix = ""
			}
			countLabel = fmt.Sprintf("[%d caller%s]", count, suffix)
		} else {
			countLabel = "[internal]"
		}
		fmt.Printf("  %-24s%-10s%-9s%s\n", sym.Name, sym.Kind, lineRange, countLabel)
	}
}

// ---------------------------------------------------------------------------
// PrintRefs prints a flat reference list.
// ---------------------------------------------------------------------------

func PrintRefs(symbolName string, refs []core.Reference, total int) {
	suffix := "s"
	if total == 1 {
		suffix = ""
	}
	fmt.Printf("%s \u2014 %d reference%s\n", symbolName, total, suffix)
	fmt.Println(Separator)

	for _, r := range refs {
		path := NormalizePath(r.FilePath)
		var location string
		if r.Line != nil {
			location = fmt.Sprintf("%s:%d", path, *r.Line)
		} else {
			location = path
		}
		displayText := r.Context
		if r.SnippetLine != nil {
			displayText = *r.SnippetLine
		}
		truncated := truncateStr(strings.TrimSpace(displayText), 80)
		fmt.Printf("%-40s%s\n", location, truncated)

		// Show multi-line context if available.
		if len(r.Snippet) > 0 {
			printSnippetContext(r.Snippet, r.Line)
		}
	}

	if len(refs) < total {
		fmt.Printf("... %d more (use --limit to show more)\n", total-len(refs))
	}
}

// ---------------------------------------------------------------------------
// PrintRefsGrouped prints references grouped by edge kind.
// ---------------------------------------------------------------------------

func PrintRefsGrouped(symbolName string, groups []ReferenceGroup, total int) {
	suffix := "s"
	if total == 1 {
		suffix = ""
	}
	fmt.Printf("%s \u2014 %d reference%s\n", symbolName, total, suffix)
	fmt.Println(Separator)

	shown := 0
	for _, g := range groups {
		kindLabel := humanizeEdgeKind(g.Kind)
		fmt.Printf("%s (%d):\n", kindLabel, len(g.Refs))
		for _, r := range g.Refs {
			path := NormalizePath(r.FilePath)
			var location string
			if r.Line != nil {
				location = fmt.Sprintf("%s:%d", path, *r.Line)
			} else {
				location = path
			}
			displayText := r.Context
			if r.SnippetLine != nil {
				displayText = *r.SnippetLine
			}
			truncated := truncateStr(strings.TrimSpace(displayText), 80)
			fmt.Printf("  %-38s%s\n", location, truncated)

			// Show multi-line context if available.
			if len(r.Snippet) > 0 {
				printSnippetContext(r.Snippet, r.Line)
			}
		}
		shown += len(g.Refs)
		fmt.Println()
	}

	if shown < total {
		fmt.Printf("... %d more (use --limit to show more)\n", total-shown)
	}
}

// ReferenceGroup holds references under a single edge kind for grouped output.
type ReferenceGroup struct {
	Kind string
	Refs []core.Reference
}

// ---------------------------------------------------------------------------
// PrintFileRefs prints file-level references.
// ---------------------------------------------------------------------------

func PrintFileRefs(filePath string, refs []core.Reference, total int) {
	path := NormalizePath(filePath)
	suffix := "s"
	if total == 1 {
		suffix = ""
	}
	fmt.Printf("%s \u2014 %d reference%s\n", path, total, suffix)
	fmt.Println(Separator)

	for _, r := range refs {
		rpath := NormalizePath(r.FilePath)
		var location string
		if r.Line != nil {
			location = fmt.Sprintf("%s:%d", rpath, *r.Line)
		} else {
			location = rpath
		}
		displayText := r.Context
		if r.SnippetLine != nil {
			displayText = *r.SnippetLine
		}
		truncated := truncateStr(strings.TrimSpace(displayText), 80)
		fmt.Printf("%-40s%s\n", location, truncated)

		// Show multi-line context if available.
		if len(r.Snippet) > 0 {
			printSnippetContext(r.Snippet, r.Line)
		}
	}

	if len(refs) < total {
		fmt.Printf("... %d more (use --limit to show more)\n", total-len(refs))
	}
}

// ---------------------------------------------------------------------------
// PrintDeps prints dependencies of a symbol, grouped by kind.
// ---------------------------------------------------------------------------

func PrintDeps(symbolName string, deps []core.Dependency, maxDepth int) {
	var depthLabel string
	if maxDepth <= 1 {
		depthLabel = "direct dependencies"
	} else {
		depthLabel = fmt.Sprintf("transitive dependencies (depth %d)", maxDepth)
	}

	fmt.Printf("%s \u2014 %s\n", symbolName, depthLabel)
	fmt.Println(Separator)

	if len(deps) == 0 {
		fmt.Println("(no dependencies found)")
		return
	}

	// Group by kind, preserving insertion order.
	type depGroup struct {
		kind string
		deps []*core.Dependency
	}
	var groups []depGroup
	for i := range deps {
		d := &deps[i]
		found := false
		for j := range groups {
			if groups[j].kind == d.Kind {
				groups[j].deps = append(groups[j].deps, d)
				found = true
				break
			}
		}
		if !found {
			groups = append(groups, depGroup{kind: d.Kind, deps: []*core.Dependency{d}})
		}
	}

	for _, g := range groups {
		allExternal := true
		for _, d := range g.deps {
			if !d.IsExternal {
				allExternal = false
				break
			}
		}
		if allExternal {
			fmt.Printf("%s (external):\n", g.kind)
		} else {
			fmt.Printf("%s:\n", g.kind)
		}

		for _, dep := range g.deps {
			if dep.IsExternal {
				fmt.Printf("  %-24s(external)\n", dep.Name)
			} else if dep.FilePath != nil {
				path := NormalizePath(*dep.FilePath)
				fmt.Printf("  %-24s%s\n", dep.Name, path)
			} else {
				fmt.Printf("  %s\n", dep.Name)
			}
		}

		fmt.Println()
	}
}

// ---------------------------------------------------------------------------
// PrintFileDeps prints file-level dependencies.
// ---------------------------------------------------------------------------

func PrintFileDeps(filePath string, deps []core.Dependency, maxDepth int) {
	path := NormalizePath(filePath)
	var depthLabel string
	if maxDepth <= 1 {
		depthLabel = "direct dependencies"
	} else {
		depthLabel = fmt.Sprintf("transitive dependencies (depth %d)", maxDepth)
	}

	fmt.Printf("%s \u2014 %s\n", path, depthLabel)
	fmt.Println(Separator)

	if len(deps) == 0 {
		fmt.Println("(no dependencies found)")
		return
	}

	// Group by kind, preserving insertion order.
	type depGroup struct {
		kind string
		deps []*core.Dependency
	}
	var groups []depGroup
	for i := range deps {
		d := &deps[i]
		found := false
		for j := range groups {
			if groups[j].kind == d.Kind {
				groups[j].deps = append(groups[j].deps, d)
				found = true
				break
			}
		}
		if !found {
			groups = append(groups, depGroup{kind: d.Kind, deps: []*core.Dependency{d}})
		}
	}

	for _, g := range groups {
		allExternal := true
		for _, d := range g.deps {
			if !d.IsExternal {
				allExternal = false
				break
			}
		}
		if allExternal {
			fmt.Printf("%s (external):\n", g.kind)
		} else {
			fmt.Printf("%s:\n", g.kind)
		}

		for _, dep := range g.deps {
			if dep.IsExternal {
				fmt.Printf("  %-24s(external)\n", dep.Name)
			} else if dep.FilePath != nil {
				fpath := NormalizePath(*dep.FilePath)
				fmt.Printf("  %-24s%s\n", dep.Name, fpath)
			} else {
				fmt.Printf("  %s\n", dep.Name)
			}
		}

		fmt.Println()
	}
}

// ---------------------------------------------------------------------------
// PrintImpact prints an impact analysis result (blast radius).
// ---------------------------------------------------------------------------

func PrintImpact(symbolName string, result *core.ImpactResult) {
	fmt.Printf("Impact analysis: %s\n", symbolName)
	fmt.Println(Separator)

	if len(result.NodesByDepth) == 0 && len(result.TestFiles) == 0 {
		fmt.Println("(no impact detected)")
		return
	}

	for _, dg := range result.NodesByDepth {
		depthLabel := impactDepthLabel(dg.Depth)
		fmt.Printf("%s (%d):\n", depthLabel, len(dg.Nodes))

		maxDisplay := 10
		displayNodes := dg.Nodes
		if len(displayNodes) > maxDisplay {
			displayNodes = displayNodes[:maxDisplay]
		}

		for _, node := range displayNodes {
			path := NormalizePath(node.FilePath)
			fmt.Printf("  %-40s%s\n", node.Name, path)
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
			path := NormalizePath(node.FilePath)
			fmt.Printf("  %s\n", path)
		}

		if len(result.TestFiles) > maxDisplay {
			fmt.Printf("  ... (%d more)\n", len(result.TestFiles)-maxDisplay)
		}
	}
}

// ---------------------------------------------------------------------------
// PrintTrace prints call paths from entry points to the target.
// ---------------------------------------------------------------------------

func PrintTrace(symbolName string, result *core.TraceResult, total int, truncated bool) {
	pathCount := len(result.Paths)
	pathWord := "paths"
	if pathCount == 1 {
		pathWord = "path"
	}

	displayCount := pathCount
	if truncated {
		displayCount = total
	}
	fmt.Printf("%s \u2014 %d entry %s\n", symbolName, displayCount, pathWord)
	fmt.Println(Separator)

	if pathCount == 0 {
		fmt.Println("(no entry paths found)")
		return
	}

	for i, callPath := range result.Paths {
		if len(callPath.Steps) == 0 {
			continue
		}

		// First step: the entry point (no arrow prefix).
		entry := callPath.Steps[0]
		fmt.Printf("Path %d: %s\n", i+1, entry.SymbolName)

		// Subsequent steps: indented with arrow.
		for stepIdx := 1; stepIdx < len(callPath.Steps); stepIdx++ {
			step := callPath.Steps[stepIdx]
			indent := strings.Repeat("  ", stepIdx)
			path := NormalizePath(step.FilePath)
			location := fmt.Sprintf("%s:%d", path, step.Line)
			fmt.Printf("%s\u2514\u2500\u2192 %-40s%s\n", indent, step.SymbolName, location)
		}

		// Blank line between paths (but not after the last one).
		if i < pathCount-1 {
			fmt.Println()
		}
	}

	if truncated {
		fmt.Printf("... %d more paths (use --limit to show more)\n", total-pathCount)
	}
}

// ---------------------------------------------------------------------------
// PrintEntrypoints prints entry points grouped by type.
// ---------------------------------------------------------------------------

func PrintEntrypoints(groups []EntrypointGroup, total int, fileCount int) {
	fileWord := "files"
	if fileCount == 1 {
		fileWord = "file"
	}
	fmt.Printf("Entrypoints \u2014 %d across %d %s\n", total, fileCount, fileWord)
	fmt.Println(Separator)

	if len(groups) == 0 {
		fmt.Println("(no entry points found)")
		return
	}

	for i, g := range groups {
		fmt.Printf("%s:\n", g.Name)

		// Calculate max name width for alignment within this group.
		maxNameLen := 0
		for _, e := range g.Entries {
			nameLen := utf8.RuneCountInString(e.Name)
			if nameLen > maxNameLen {
				maxNameLen = nameLen
			}
		}
		nameWidth := maxNameLen + 2
		if nameWidth < 22 {
			nameWidth = 22
		}

		for _, entry := range g.Entries {
			path := NormalizePath(entry.FilePath)
			var methodSuffix string
			if entry.MethodCount > 0 {
				ms := "s"
				if entry.MethodCount == 1 {
					ms = ""
				}
				methodSuffix = fmt.Sprintf("   \u2192 %d method%s", entry.MethodCount, ms)
			}
			fmt.Printf("  %-*s%s%s\n", nameWidth, entry.Name, path, methodSuffix)
		}

		// Blank line between groups (but not after the last one).
		if i < len(groups)-1 {
			fmt.Println()
		}
	}
}

// EntrypointGroup holds a named group of entry points for display.
type EntrypointGroup struct {
	Name    string
	Entries []EntrypointInfo
}

// ---------------------------------------------------------------------------
// PrintMap prints a structural map of the repository.
// ---------------------------------------------------------------------------

func PrintMap(
	projectName string,
	stats *MapStats,
	entrypoints []EntrypointGroup,
	coreSymbols []CoreSymbol,
	directories []DirStats,
) {
	// Header line.
	fmt.Printf("%s \u2014 %s files, %s symbols, %s edges\n",
		projectName,
		formatNumber(stats.FileCount),
		formatNumber(stats.SymbolCount),
		formatNumber(stats.EdgeCount),
	)
	fmt.Println(Separator)

	// Languages line.
	if len(stats.Languages) > 0 {
		fmt.Printf("Languages: %s\n", strings.Join(stats.Languages, ", "))
	}

	// Entry points section.
	var epCount int
	var epLines []string

	for _, g := range entrypoints {
		for _, entry := range g.Entries {
			path := NormalizePath(entry.FilePath)
			// Extract directory portion of the path.
			dir := ""
			if pos := strings.LastIndex(path, "/"); pos >= 0 {
				dir = path[:pos] + "/"
			}

			// Strip leading "src/" for brevity.
			displayDir := strings.TrimPrefix(dir, "src/")

			var methodSuffix string
			if entry.MethodCount > 0 {
				ms := "s"
				if entry.MethodCount == 1 {
					ms = ""
				}
				methodSuffix = fmt.Sprintf("   \u2192 %d method%s", entry.MethodCount, ms)
			}

			epLines = append(epLines, fmt.Sprintf("  %-32s%-32s%s", entry.Name, displayDir, methodSuffix))
			epCount++
		}
	}

	if len(epLines) > 0 {
		fmt.Println()
		fmt.Println("Entry points:")
		maxDisplay := 8
		for i, line := range epLines {
			if i >= maxDisplay {
				break
			}
			fmt.Println(line)
		}
		if epCount > maxDisplay {
			fmt.Printf("  ... %d more\n", epCount-maxDisplay)
		}
	}

	// Core symbols section.
	if len(coreSymbols) > 0 {
		fmt.Println()
		fmt.Println("Core symbols (by caller count):")
		for _, sym := range coreSymbols {
			path := NormalizePath(sym.FilePath)
			// Strip leading "src/" for brevity.
			displayPath := strings.TrimPrefix(path, "src/")

			cs := "s"
			if sym.CallerCount == 1 {
				cs = ""
			}
			callerLabel := fmt.Sprintf("%d caller%s", sym.CallerCount, cs)
			fmt.Printf("  %-32s%-14s%s\n", sym.Name, callerLabel, displayPath)
		}
	}

	// Architecture section.
	if len(directories) > 0 {
		fmt.Println()
		fmt.Println("Architecture:")
		for _, d := range directories {
			fs := "s"
			if d.FileCount == 1 {
				fs = ""
			}
			fileLabel := fmt.Sprintf("%d file%s", d.FileCount, fs)
			ss := "s"
			if d.SymbolCount == 1 {
				ss = ""
			}
			symLabel := fmt.Sprintf("%d symbol%s", d.SymbolCount, ss)
			fmt.Printf("  %-24s%-14s%s\n", d.Directory, fileLabel, symLabel)
		}
	}
}

// ---------------------------------------------------------------------------
// PrintSearchResults prints FTS5 results with scores.
// ---------------------------------------------------------------------------

func PrintSearchResults(results []core.SearchResult, query string) {
	fmt.Printf("Results for: \"%s\"\n", query)
	fmt.Println(Separator)

	if len(results) == 0 {
		fmt.Println("(no results found)")
		return
	}

	for _, r := range results {
		path := NormalizePath(r.FilePath)
		location := fmt.Sprintf("%s:%d", path, r.LineStart)
		fmt.Printf("%.2f  %-40s%-36s  %s\n", r.Score, r.Name, location, r.Kind)
	}
}

// ---------------------------------------------------------------------------
// PrintStatus prints index health information.
// ---------------------------------------------------------------------------

func PrintStatus(data *StatusData) {
	var statusLabel string
	if !data.IndexExists {
		statusLabel = "not indexed"
	} else if data.SymbolCount == 0 {
		statusLabel = "empty"
	} else {
		statusLabel = "up to date"
	}

	fmt.Printf("Index status: %s\n", statusLabel)
	fmt.Printf("  Symbols:    %s\n", formatNumber(data.SymbolCount))
	fmt.Printf("  Files:      %s\n", formatNumber(data.FileCount))
	fmt.Printf("  Edges:      %s\n", formatNumber(data.EdgeCount))
	if data.IndexExists {
		searchLabel := "available"
		if !data.SearchAvailable {
			searchLabel = "unavailable"
		}
		fmt.Printf("  Search:     %s\n", searchLabel)
	}
	if data.LastIndexedRelative != nil {
		fmt.Printf("  Last index: %s\n", *data.LastIndexedRelative)
	} else {
		fmt.Println("  Last index: never")
	}
}

// ---------------------------------------------------------------------------
// PrintIncrementalResult prints incremental index results.
// ---------------------------------------------------------------------------

func PrintIncrementalResult(stats *IncrementalStats) {
	total := len(stats.Modified) + len(stats.Added) + len(stats.Deleted)
	suffix := "s"
	if total == 1 {
		suffix = ""
	}
	fmt.Fprintf(os.Stderr, "%d file%s changed. Re-indexing...\n", total, suffix)

	for _, p := range stats.Modified {
		fmt.Fprintf(os.Stderr, "  Modified: %s\n", NormalizePath(p))
	}
	for _, p := range stats.Added {
		fmt.Fprintf(os.Stderr, "  Added:    %s\n", NormalizePath(p))
	}
	for _, p := range stats.Deleted {
		fmt.Fprintf(os.Stderr, "  Deleted:  %s\n", NormalizePath(p))
	}

	fmt.Fprintf(os.Stderr, "Updated in %.1fs.\n", stats.DurationSecs)
}

// ---------------------------------------------------------------------------
// Helper: methodDisplayLine builds the display string for a method.
// ---------------------------------------------------------------------------

func methodDisplayLine(method *core.Symbol) string {
	sig := method.Name
	if method.Signature != nil {
		sig = *method.Signature
	}

	modifiers := extractModifiers(method.Metadata)
	if len(modifiers) == 0 {
		return sig
	}

	// Only prepend modifiers not already present in the signature text.
	var missing []string
	for _, m := range modifiers {
		if !strings.Contains(sig, m) {
			missing = append(missing, m)
		}
	}

	if len(missing) == 0 {
		return sig
	}
	return strings.Join(missing, " ") + " " + sig
}

// ---------------------------------------------------------------------------
// Helper: extractModifiers parses JSON metadata for display-worthy modifiers.
// ---------------------------------------------------------------------------

func extractModifiers(metadataJSON string) []string {
	if metadataJSON == "" {
		return nil
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(metadataJSON), &parsed); err != nil {
		return nil
	}

	var mods []string

	// Access modifier (only show non-public).
	if access, ok := parsed["access"].(string); ok {
		switch access {
		case "private", "protected", "internal", "protected internal":
			mods = append(mods, access)
		}
	}

	if isAsync, ok := parsed["is_async"].(bool); ok && isAsync {
		mods = append(mods, "async")
	}

	if isStatic, ok := parsed["is_static"].(bool); ok && isStatic {
		mods = append(mods, "static")
	}

	if isAbstract, ok := parsed["is_abstract"].(bool); ok && isAbstract {
		mods = append(mods, "abstract")
	}

	if isVirtual, ok := parsed["is_virtual"].(bool); ok && isVirtual {
		mods = append(mods, "virtual")
	}

	if isOverride, ok := parsed["is_override"].(bool); ok && isOverride {
		mods = append(mods, "override")
	}

	return mods
}

// ---------------------------------------------------------------------------
// Helper: humanizeEdgeKind converts edge kind strings to past tense labels.
// ---------------------------------------------------------------------------

func humanizeEdgeKind(kind string) string {
	switch kind {
	case "instantiates":
		return "instantiated"
	case "extends":
		return "extended"
	case "implements":
		return "implemented"
	case "references_type":
		return "used as type"
	case "imports":
		return "imported"
	case "calls":
		return "called"
	case "references":
		return "referenced"
	default:
		return kind
	}
}

// ---------------------------------------------------------------------------
// Helper: formatNumber adds comma separators (e.g. 6764 -> "6,764").
// ---------------------------------------------------------------------------

func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result strings.Builder
	result.Grow(len(s) + len(s)/3)
	remainder := len(s) % 3
	if remainder == 0 {
		remainder = 3
	}
	result.WriteString(s[:remainder])
	for i := remainder; i < len(s); i += 3 {
		result.WriteByte(',')
		result.WriteString(s[i : i+3])
	}
	return result.String()
}

// ---------------------------------------------------------------------------
// Helper: truncateStr truncates a string to max chars, adding "..." if needed.
// ---------------------------------------------------------------------------

func truncateStr(s string, maxChars int) string {
	if utf8.RuneCountInString(s) <= maxChars {
		return s
	}
	runes := []rune(s)
	limit := maxChars - 3
	if limit < 0 {
		limit = 0
	}
	return string(runes[:limit]) + "..."
}

// ---------------------------------------------------------------------------
// Helper: printSnippetContext prints multi-line snippet context with line nums.
// ---------------------------------------------------------------------------

func printSnippetContext(snippet []string, refLine *int64) {
	if refLine == nil {
		return
	}

	lineNum := int(*refLine)
	refIdxInSnippet := len(snippet) / 2 // approximate center
	startLine := lineNum - refIdxInSnippet
	if startLine < 1 {
		startLine = 1
	}

	for i, code := range snippet {
		currentLine := startLine + i
		marker := " "
		if currentLine == lineNum {
			marker = ">"
		}
		fmt.Printf("  %s %4d | %s\n", marker, currentLine, code)
	}
}

// ---------------------------------------------------------------------------
// Helper: impactDepthLabel returns a human-readable label for impact depth.
// ---------------------------------------------------------------------------

func impactDepthLabel(depth int) string {
	switch depth {
	case 1:
		return "Direct callers"
	case 2:
		return "Second-degree"
	case 3:
		return "Third-degree"
	default:
		return "Further impact"
	}
}
