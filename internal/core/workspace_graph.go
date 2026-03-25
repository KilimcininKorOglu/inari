// Package core provides workspace-level query facade over multiple independent
// project graphs.
//
// WorkspaceGraph opens N Graph instances (one per workspace member) and fans
// out queries, merging results with project-name tags. This is a pure
// query-time aggregation layer -- no data is written to any shared database.
// Each member's .inari/graph.db remains fully independent.
//
// Symbol IDs are never modified on disk. Project prefixing happens at the
// output boundary only.
package core

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
)

// WorkspaceGraph is a workspace-level query facade over multiple independent
// project graphs.
type WorkspaceGraph struct {
	members        []WorkspaceMember
	skippedMembers []string
}

// WorkspaceMember is a single project within a workspace.
type WorkspaceMember struct {
	// Human-readable project name from the manifest.
	Name string
	// Absolute path to the project root.
	Root string
	// Open graph connection.
	Graph *Graph
}

// WorkspaceMemberInput describes a workspace member to open.
type WorkspaceMemberInput struct {
	// Human-readable project name from the manifest.
	Name string
	// Absolute path to the project root.
	Root string
}

// WorkspaceSymbol is a symbol result tagged with its source project.
type WorkspaceSymbol struct {
	// The workspace member name this symbol belongs to.
	Project string `json:"project"`
	// Symbol ID (relative to project root, not prefixed).
	ID string `json:"id"`
	// Symbol name.
	Name string `json:"name"`
	// Symbol kind (function, class, method, etc.).
	Kind string `json:"kind"`
	// File path relative to the project root.
	FilePath string `json:"file_path"`
	// First line of the symbol definition (1-based).
	LineStart uint32 `json:"line_start"`
	// Last line of the symbol definition (1-based).
	LineEnd uint32 `json:"line_end"`
}

// WorkspaceRef is a reference result tagged with its source project.
type WorkspaceRef struct {
	// The workspace member name this reference belongs to.
	Project   string    `json:"project"`
	Reference Reference `json:"reference"`
}

// WorkspaceEntrypointGroup groups entrypoints by their source project.
type WorkspaceEntrypointGroup struct {
	// The workspace member name.
	Project string `json:"project"`
	// Entrypoint symbols with their fan-out counts.
	Entries []EntrypointResult `json:"entries"`
}

// OpenWorkspaceGraph opens all member graphs.
//
// For each member input, attempts to open root/.inari/graph.db. Members with
// missing graph.db are warned about and skipped (partial workspace support).
// If there are more than 20 members, emits a performance warning.
func OpenWorkspaceGraph(members []WorkspaceMemberInput) (*WorkspaceGraph, error) {
	totalRequested := len(members)

	if totalRequested > 20 {
		log.Printf(
			"Workspace has %d members. Consider using --project <name> to target a specific member.",
			totalRequested,
		)
	}

	opened := make([]WorkspaceMember, 0, totalRequested)
	var skippedNames []string

	for _, m := range members {
		dbPath := filepath.Join(m.Root, ".inari", "graph.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			log.Printf(
				"Member '%s' has no graph.db at %s. Skipping (run 'inari index' in that project).",
				m.Name, dbPath,
			)
			skippedNames = append(skippedNames, m.Name)
			continue
		}

		graph, err := Open(dbPath)
		if err != nil {
			log.Printf("Failed to open graph for member '%s': %v. Skipping.", m.Name, err)
			skippedNames = append(skippedNames, m.Name)
			continue
		}

		opened = append(opened, WorkspaceMember{
			Name:  m.Name,
			Root:  m.Root,
			Graph: graph,
		})
	}

	if len(skippedNames) > 0 {
		fmt.Fprintf(os.Stderr,
			"Warning: %d of %d workspace members not indexed (run 'inari workspace index')\n",
			len(skippedNames), totalRequested,
		)
	}

	return &WorkspaceGraph{members: opened, skippedMembers: skippedNames}, nil
}

// Close closes all member graph connections.
func (wg *WorkspaceGraph) Close() error {
	var firstErr error
	for _, m := range wg.members {
		if err := m.Graph.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// MemberCount returns the number of successfully opened members.
func (wg *WorkspaceGraph) MemberCount() int {
	return len(wg.members)
}

// MemberNames returns the names of all opened members.
func (wg *WorkspaceGraph) MemberNames() []string {
	names := make([]string, len(wg.members))
	for i, m := range wg.members {
		names[i] = m.Name
	}
	return names
}

// Members returns the list of opened workspace members.
func (wg *WorkspaceGraph) Members() []WorkspaceMember {
	return wg.members
}

// SkippedMembers returns the names of workspace members that could not be
// opened (missing index, corrupt database, etc.). Callers can include this
// in output so users know which members were excluded from query results.
func (wg *WorkspaceGraph) SkippedMembers() []string {
	return wg.skippedMembers
}

// GetLanguages returns a deduplicated, sorted union of languages across all
// workspace members.
func (wg *WorkspaceGraph) GetLanguages() []string {
	seen := make(map[string]struct{})
	var langs []string

	for _, m := range wg.members {
		memberLangs, err := m.Graph.GetLanguages()
		if err != nil {
			log.Printf("Failed to get languages for member '%s': %v", m.Name, err)
			continue
		}
		for _, lang := range memberLangs {
			if _, exists := seen[lang]; !exists {
				seen[lang] = struct{}{}
				langs = append(langs, lang)
			}
		}
	}

	sort.Strings(langs)
	return langs
}

// FindSymbol finds a symbol by name across all workspace members.
//
// Returns all matches tagged with their project name. The caller decides
// how to disambiguate (e.g. prompt user or use --project).
func (wg *WorkspaceGraph) FindSymbol(name string) []WorkspaceSymbol {
	var results []WorkspaceSymbol

	for _, m := range wg.members {
		sym, err := m.Graph.FindSymbol(name)
		if err != nil {
			log.Printf("Error querying member '%s' for symbol '%s': %v", m.Name, name, err)
			continue
		}
		if sym == nil {
			continue
		}
		results = append(results, WorkspaceSymbol{
			Project:   m.Name,
			ID:        sym.ID,
			Name:      sym.Name,
			Kind:      sym.Kind,
			FilePath:  sym.FilePath,
			LineStart: sym.LineStart,
			LineEnd:   sym.LineEnd,
		})
	}

	return results
}

// FindRefs finds references to a symbol across all workspace members.
//
// Fans out to each member with limit*2 to allow fair merging, then merges
// results sorted by (kind, from_name) and applies the global limit.
func (wg *WorkspaceGraph) FindRefs(symbolName string, kinds []string, limit int) []WorkspaceRef {
	perMemberLimit := limit * 2
	if perMemberLimit < limit {
		// Overflow guard.
		perMemberLimit = limit
	}

	var allRefs []WorkspaceRef

	for _, m := range wg.members {
		refs, _, err := m.Graph.FindRefs(symbolName, kinds, perMemberLimit)
		if err != nil {
			log.Printf("Error querying refs in member '%s': %v. Results may be incomplete.", m.Name, err)
			continue
		}
		for _, r := range refs {
			allRefs = append(allRefs, WorkspaceRef{
				Project:   m.Name,
				Reference: r,
			})
		}
	}

	// Sort by (kind, from_name) for deterministic output.
	sort.Slice(allRefs, func(i, j int) bool {
		if allRefs[i].Reference.Kind != allRefs[j].Reference.Kind {
			return allRefs[i].Reference.Kind < allRefs[j].Reference.Kind
		}
		return allRefs[i].Reference.FromName < allRefs[j].Reference.FromName
	})

	if len(allRefs) > limit {
		allRefs = allRefs[:limit]
	}

	return allRefs
}

// GetEntrypoints returns entry points from all workspace members, grouped by
// project.
func (wg *WorkspaceGraph) GetEntrypoints() []WorkspaceEntrypointGroup {
	var results []WorkspaceEntrypointGroup

	for _, m := range wg.members {
		entries, err := m.Graph.GetEntrypoints()
		if err != nil {
			log.Printf("Error getting entrypoints from member '%s': %v", m.Name, err)
			continue
		}
		if len(entries) == 0 {
			continue
		}
		results = append(results, WorkspaceEntrypointGroup{
			Project: m.Name,
			Entries: entries,
		})
	}

	return results
}

// SymbolCount returns the total symbol count across all workspace members.
func (wg *WorkspaceGraph) SymbolCount() int {
	total := 0
	for _, m := range wg.members {
		count, err := m.Graph.SymbolCount()
		if err != nil {
			continue
		}
		total += count
	}
	return total
}

// EdgeCount returns the total edge count across all workspace members.
func (wg *WorkspaceGraph) EdgeCount() int {
	total := 0
	for _, m := range wg.members {
		count, err := m.Graph.EdgeCount()
		if err != nil {
			continue
		}
		total += count
	}
	return total
}

// FileCount returns the total file count across all workspace members.
func (wg *WorkspaceGraph) FileCount() int {
	total := 0
	for _, m := range wg.members {
		count, err := m.Graph.FileCount()
		if err != nil {
			continue
		}
		total += count
	}
	return total
}

// WorkspaceDisplayID prefixes a symbol ID with the project name for workspace
// display. Never stored -- used only at the output boundary.
func WorkspaceDisplayID(project, id string) string {
	return project + "::" + id
}
