// Workspace-specific output formatting functions.
//
// These print functions handle workspace-level commands where results
// are tagged with their source project name.
package output

import (
	"fmt"
	"strings"
)

// PrintWorkspaceList prints a workspace member table.
func PrintWorkspaceList(workspaceName string, members []MemberListEntry) {
	fmt.Printf("Workspace: %s\n", workspaceName)
	fmt.Println(Separator)

	if len(members) == 0 {
		fmt.Println("  (no members)")
		return
	}

	// Find column widths.
	maxName := 4
	maxPath := 4
	for _, m := range members {
		if len(m.Name) > maxName {
			maxName = len(m.Name)
		}
		if len(m.Path) > maxPath {
			maxPath = len(m.Path)
		}
	}

	// Header.
	fmt.Printf("  %-*s  %-*s  %-15s  %5s  %7s\n",
		maxName, "Name",
		maxPath, "Path",
		"Status",
		"Files",
		"Symbols",
	)

	for _, m := range members {
		fileStr := "\u2500"
		if m.FileCount > 0 {
			fileStr = formatNumber(m.FileCount)
		}
		symStr := "\u2500"
		if m.SymbolCount > 0 {
			symStr = formatNumber(m.SymbolCount)
		}
		fmt.Printf("  %-*s  %-*s  %-15s  %5s  %7s\n",
			maxName, m.Name,
			maxPath, NormalizePath(m.Path),
			m.Status,
			fileStr,
			symStr,
		)
	}
}

// PrintWorkspaceStatus prints workspace status with per-member details.
func PrintWorkspaceStatus(data *WorkspaceStatusData) {
	fmt.Printf("Workspace: %s\n", data.WorkspaceName)
	fmt.Println(Separator)

	for _, m := range data.Members {
		var statusLabel string
		if !m.Status.IndexExists {
			statusLabel = "not indexed"
		} else if m.Status.SymbolCount == 0 {
			statusLabel = "empty"
		} else {
			statusLabel = "indexed"
		}
		last := "never"
		if m.Status.LastIndexedRelative != nil {
			last = *m.Status.LastIndexedRelative
		}
		fmt.Printf("  %-16s%-14s%6s files  %7s symbols  %7s edges  %s\n",
			m.Name,
			statusLabel,
			formatNumber(m.Status.FileCount),
			formatNumber(m.Status.SymbolCount),
			formatNumber(m.Status.EdgeCount),
			last,
		)
	}

	fmt.Println(Separator)
	fmt.Printf("  %-16s%-14s%6s files  %7s symbols  %7s edges\n",
		"Total",
		"",
		formatNumber(data.Totals.FileCount),
		formatNumber(data.Totals.SymbolCount),
		formatNumber(data.Totals.EdgeCount),
	)
}

// PrintWorkspaceRefs prints references tagged with project names.
func PrintWorkspaceRefs(symbolName string, refs []WorkspaceRef, total int) {
	suffix := "s"
	if total == 1 {
		suffix = ""
	}
	fmt.Printf("%s \u2014 %d reference%s (workspace)\n", symbolName, total, suffix)
	fmt.Println(Separator)

	for _, wr := range refs {
		r := &wr.Reference
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
		truncated := truncateStr(strings.TrimSpace(displayText), 70)
		fmt.Printf("[%-12s] %-36s%s\n", wr.Project, location, truncated)
	}
}

// PrintWorkspaceMap prints a workspace-level map (delegates to PrintMap).
func PrintWorkspaceMap(
	workspaceName string,
	stats *MapStats,
	entrypoints []EntrypointGroup,
	coreSymbols []CoreSymbol,
	directories []DirStats,
) {
	PrintMap(workspaceName, stats, entrypoints, coreSymbols, directories)
}

// PrintWorkspaceSearchResults prints workspace find results with project tags.
func PrintWorkspaceSearchResults(results []WorkspaceSearchResult, query string) {
	suffix := "s"
	if len(results) == 1 {
		suffix = ""
	}
	fmt.Printf("find \"%s\" \u2014 %d result%s\n", query, len(results), suffix)
	fmt.Println(Separator)

	for _, r := range results {
		path := NormalizePath(r.Result.FilePath)
		lineRange := FormatLineRange(r.Result.LineStart, r.Result.LineEnd)
		fmt.Printf("[%-12s] %-32s%-8s  %s:%s  (%.2f)\n",
			r.Project,
			r.Result.Name,
			r.Result.Kind,
			path,
			lineRange,
			r.Result.Score,
		)
	}
}

// PrintWorkspaceEntrypoints prints workspace entrypoints (delegates to PrintEntrypoints).
func PrintWorkspaceEntrypoints(groups []EntrypointGroup, total int, fileCount int) {
	PrintEntrypoints(groups, total, fileCount)
}
