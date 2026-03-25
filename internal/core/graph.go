// Package core provides the SQLite-backed dependency graph storage.
//
// Stores symbols, edges, and file hashes. Provides query methods
// for refs, deps, rdeps, and impact analysis.
package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	inariSQL "github.com/KilimcininKorOglu/inari/sql"
	_ "modernc.org/sqlite"
)

// sharedPragmas are SQLite performance settings applied to both
// graph and searcher connections for consistent behavior.
var sharedPragmas = []string{
	"PRAGMA journal_mode=WAL",
	"PRAGMA busy_timeout=5000",  // 5 seconds for lock contention
	"PRAGMA synchronous=NORMAL",
	"PRAGMA cache_size=-64000",  // 64 MB page cache
	"PRAGMA temp_store=MEMORY",
}

// Graph is the dependency graph backed by SQLite.
type Graph struct {
	db *sql.DB
}

// Symbol is a code symbol extracted from source and stored in the graph.
type Symbol struct {
	// Unique identifier: "{file_path}::{name}::{kind}".
	ID string `json:"id"`
	// The symbol name (e.g. PaymentService, processPayment).
	Name string `json:"name"`
	// The kind of symbol (function, class, method, etc.).
	Kind string `json:"kind"`
	// File path relative to project root, always forward slashes.
	FilePath string `json:"file_path"`
	// First line of the symbol definition (1-based).
	LineStart uint32 `json:"line_start"`
	// Last line of the symbol definition (1-based).
	LineEnd uint32 `json:"line_end"`
	// Full type signature where available.
	Signature *string `json:"signature"`
	// Extracted doc comment.
	Docstring *string `json:"docstring"`
	// Parent symbol ID (e.g. class ID for a method).
	ParentID *string `json:"parent_id"`
	// Source language.
	Language string `json:"language"`
	// JSON blob with modifiers, parameters, return type, etc.
	Metadata string `json:"metadata"`
}

// Edge is a relationship between two symbols.
type Edge struct {
	// Source symbol ID.
	FromID string `json:"from_id"`
	// Target symbol ID (may reference external symbols not in the index).
	ToID string `json:"to_id"`
	// Edge kind: calls, imports, extends, implements, instantiates, references, references_type.
	Kind string `json:"kind"`
	// File where this edge was observed.
	FilePath string `json:"file_path"`
	// Line number where the edge was observed.
	Line *uint32 `json:"line"`
}

// ChangedFiles is the result of comparing current file hashes against the stored index.
type ChangedFiles struct {
	// Files that are new (not previously indexed).
	Added []string `json:"added"`
	// Files whose content hash has changed.
	Modified []string `json:"modified"`
	// Files that were previously indexed but no longer exist.
	Deleted []string `json:"deleted"`
}

// IsEmpty returns true if there are no changes.
func (cf *ChangedFiles) IsEmpty() bool {
	return len(cf.Added) == 0 && len(cf.Modified) == 0 && len(cf.Deleted) == 0
}

// Total returns the total number of changed files.
func (cf *ChangedFiles) Total() int {
	return len(cf.Added) + len(cf.Modified) + len(cf.Deleted)
}

// ClassRelationships contains the relationships of a class symbol:
// inheritance, interfaces, and dependencies.
type ClassRelationships struct {
	// Classes this class extends.
	Extends []string `json:"extends"`
	// Interfaces this class implements.
	Implements []string `json:"implements"`
	// Distinct symbol names from outgoing edges (imports, calls, etc.).
	Dependencies []string `json:"dependencies"`
}

// CallerInfo contains information about a caller of a symbol.
type CallerInfo struct {
	// Display name of the caller (e.g. OrderController.checkout).
	Name string `json:"name"`
	// Number of call sites from this caller.
	Count int `json:"count"`
}

// Reference is a reference to a symbol from elsewhere in the codebase.
type Reference struct {
	// The ID of the symbol making the reference.
	FromID string `json:"from_id"`
	// The human-readable name of the referencing symbol.
	FromName string `json:"from_name"`
	// The kind of reference (calls, imports, extends, etc.).
	Kind string `json:"kind"`
	// File path where the reference occurs.
	FilePath string `json:"file_path"`
	// Line number of the reference, if known.
	Line *int64 `json:"line"`
	// Context string (caller name or description).
	Context string `json:"context"`
	// The actual source line at the reference location (if available).
	SnippetLine *string `json:"snippet_line,omitempty"`
	// Multi-line context around the reference (if --context N was used).
	Snippet []string `json:"snippet,omitempty"`
}

// ImpactNode is a node in an impact analysis result.
type ImpactNode struct {
	// Symbol ID.
	ID string `json:"id"`
	// Symbol name.
	Name string `json:"name"`
	// File path where this symbol is defined.
	FilePath string `json:"file_path"`
	// Symbol kind (function, class, method, etc.).
	Kind string `json:"kind"`
	// Depth in the impact graph (1 = direct caller).
	Depth int `json:"depth"`
}

// ImpactResult is the result of an impact analysis query.
type ImpactResult struct {
	// Nodes grouped by depth level: (depth, nodes_at_that_depth).
	NodesByDepth []DepthGroup `json:"nodes_by_depth"`
	// Test files that are affected (separated from main results).
	TestFiles []ImpactNode `json:"test_files"`
	// Total number of distinct affected symbols (excluding test files).
	TotalAffected int `json:"total_affected"`
}

// DepthGroup holds impact nodes at a particular depth level.
type DepthGroup struct {
	Depth int          `json:"depth"`
	Nodes []ImpactNode `json:"nodes"`
}

// MarshalJSON implements custom JSON encoding for DepthGroup to match the
// Rust tuple serialization format: [depth, [nodes...]].
func (dg DepthGroup) MarshalJSON() ([]byte, error) {
	return json.Marshal([2]interface{}{dg.Depth, dg.Nodes})
}

// UnmarshalJSON implements custom JSON decoding for DepthGroup to match the
// Rust tuple serialization format: [depth, [nodes...]].
func (dg *DepthGroup) UnmarshalJSON(data []byte) error {
	var raw [2]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[0], &dg.Depth); err != nil {
		return err
	}
	return json.Unmarshal(raw[1], &dg.Nodes)
}

// Dependency is a dependency of a symbol.
type Dependency struct {
	// The name of the dependency.
	Name string `json:"name"`
	// File path of the dependency, if it exists in the index.
	FilePath *string `json:"file_path"`
	// Kind of dependency relationship (imports, calls, extends, etc.).
	Kind string `json:"kind"`
	// True if the dependency is not in the index (external package).
	IsExternal bool `json:"is_external"`
	// Depth in the dependency tree (1 = direct).
	Depth int `json:"depth"`
}

// CallPathStep is a single step in a call path from entry point to target.
type CallPathStep struct {
	// Display name of the symbol at this step.
	SymbolName string `json:"symbol_name"`
	// Full symbol ID.
	SymbolID string `json:"symbol_id"`
	// File path where this symbol is defined.
	FilePath string `json:"file_path"`
	// Line number of the symbol definition.
	Line uint32 `json:"line"`
	// Symbol kind (function, class, method, etc.).
	Kind string `json:"kind"`
}

// CallPath is a complete call path from an entry point to the target symbol.
type CallPath struct {
	// Ordered steps from entry point (first) to target (last).
	Steps []CallPathStep `json:"steps"`
}

// TraceResult is the result of a trace query -- all call paths reaching a target symbol.
type TraceResult struct {
	// The target symbol name.
	Target string `json:"target"`
	// All discovered call paths from entry points to the target.
	Paths []CallPath `json:"paths"`
}

// callerCountMaps holds pre-computed caller count maps for O(1) symbol resolution.
type callerCountMaps struct {
	// Exact to_id -> count (for pattern 1: exact ID, pattern 2: bare name).
	byID map[string]int
	// Bare name (after last '.') -> count (for pattern 3: member-call suffix).
	bySuffix map[string]int
}

// Open opens or creates a graph database at the given path.
//
// Applies performance pragmas and ensures the schema is up to date.
func Open(path string) (*Graph, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open graph database at %s: %w", path, err)
	}

	// Performance pragmas -- safe for single-writer use.
	// Start with shared pragmas, then add graph-specific ones.
	pragmas := make([]string, len(sharedPragmas))
	copy(pragmas, sharedPragmas)
	pragmas = append(pragmas, "PRAGMA foreign_keys=ON", "PRAGMA case_sensitive_like=ON")
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma %q: %w", pragma, err)
		}
	}

	// Create schema tables and indexes
	if _, err := db.Exec(inariSQL.SchemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	return &Graph{db: db}, nil
}

// Close closes the underlying database connection.
func (g *Graph) Close() error {
	return g.db.Close()
}

// DB returns the underlying *sql.DB for advanced usage.
func (g *Graph) DB() *sql.DB {
	return g.db
}

// scanSymbol scans a Symbol from a row that selects all columns from the symbols table.
func scanSymbol(scanner interface {
	Scan(dest ...interface{}) error
}) (Symbol, error) {
	var s Symbol
	var signature, docstring, parentID sql.NullString
	err := scanner.Scan(
		&s.ID, &s.Name, &s.Kind, &s.FilePath,
		&s.LineStart, &s.LineEnd,
		&signature, &docstring, &parentID,
		&s.Language, &s.Metadata,
	)
	if err != nil {
		return Symbol{}, err
	}
	if signature.Valid {
		s.Signature = &signature.String
	}
	if docstring.Valid {
		s.Docstring = &docstring.String
	}
	if parentID.Valid {
		s.ParentID = &parentID.String
	}
	return s, nil
}

// FindSymbol finds a symbol by exact name match, or by qualified name (Class.method).
//
// Lookup order:
//  1. Exact match on symbols.name. If multiple matches, prefer the one
//     with no parent_id (top-level symbol).
//  2. If not found and name contains '.', split on '.' and try qualified
//     lookup: parent.name = class_part AND s.name = method_part.
//  3. Returns nil for unknown symbols.
func (g *Graph) FindSymbol(name string) (*Symbol, error) {
	row := g.db.QueryRow(
		`SELECT * FROM symbols WHERE name = ?
		 ORDER BY (CASE WHEN parent_id IS NULL THEN 0 ELSE 1 END)
		 LIMIT 1`,
		name,
	)
	sym, err := scanSymbol(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Try qualified name (ClassName.methodName)
			if idx := strings.Index(name, "."); idx >= 0 {
				class := name[:idx]
				method := name[idx+1:]
				row2 := g.db.QueryRow(
					`SELECT s.* FROM symbols s
					 JOIN symbols parent ON s.parent_id = parent.id
					 WHERE parent.name = ? AND s.name = ?`,
					class, method,
				)
				sym2, err2 := scanSymbol(row2)
				if err2 != nil {
					if errors.Is(err2, sql.ErrNoRows) {
						return nil, nil
					}
					return nil, err2
				}
				return &sym2, nil
			}
			return nil, nil
		}
		return nil, err
	}
	return &sym, nil
}

// GetMethods returns all child symbols (methods, properties) of a class/interface.
//
// Returns symbols where parent_id = classID, ordered by line_start.
func (g *Graph) GetMethods(classID string) ([]Symbol, error) {
	rows, err := g.db.Query(
		"SELECT * FROM symbols WHERE parent_id = ? ORDER BY line_start",
		classID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Symbol
	for rows.Next() {
		sym, err := scanSymbol(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, sym)
	}
	return result, rows.Err()
}

// GetCallerCount counts incoming call edges for a symbol (how many callers it has).
func (g *Graph) GetCallerCount(symbolID string) (int, error) {
	bareName := g.symbolNameFromID(symbolID)
	likeMember := "%." + bareName
	var count int64
	err := g.db.QueryRow(
		`SELECT COUNT(*) FROM edges
		 WHERE (to_id = ? OR to_id = ? OR to_id LIKE ?) AND kind = 'calls'`,
		symbolID, bareName, likeMember,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// GetCallerCounts is a batch version of GetCallerCount. Returns a map of symbol_id to caller count.
//
// Efficiently fetches caller counts for multiple symbols in a single pass.
// Matches by exact ID, bare name, and member-call patterns (e.g. svc.processPayment).
func (g *Graph) GetCallerCounts(symbolIDs []string) (map[string]int, error) {
	result := make(map[string]int)
	if len(symbolIDs) == 0 {
		return result, nil
	}

	for _, symID := range symbolIDs {
		count, err := g.GetCallerCount(symID)
		if err != nil {
			return nil, err
		}
		if count > 0 {
			result[symID] = count
		}
	}
	return result, nil
}

// GetClassRelationships returns class relationships: extends, implements, and dependencies.
func (g *Graph) GetClassRelationships(classID string) (*ClassRelationships, error) {
	rels := &ClassRelationships{
		Extends:      []string{},
		Implements:   []string{},
		Dependencies: []string{},
	}

	// Build the set of source IDs to check: the class itself and the
	// __module__::class synthetic ID.
	filePath := classID
	if idx := strings.Index(classID, "::"); idx >= 0 {
		filePath = classID[:idx]
	}
	moduleClassID := filePath + "::__module__::class"

	// Get 'extends' edges from this class
	extendsRows, err := g.db.Query(
		"SELECT to_id FROM edges WHERE from_id IN (?, ?) AND kind = 'extends'",
		classID, moduleClassID,
	)
	if err != nil {
		return nil, err
	}
	defer extendsRows.Close()
	for extendsRows.Next() {
		var toID string
		if err := extendsRows.Scan(&toID); err != nil {
			return nil, err
		}
		rels.Extends = append(rels.Extends, g.symbolNameFromID(toID))
	}
	if err := extendsRows.Err(); err != nil {
		return nil, err
	}

	// Get 'implements' edges from this class
	implRows, err := g.db.Query(
		"SELECT to_id FROM edges WHERE from_id IN (?, ?) AND kind = 'implements'",
		classID, moduleClassID,
	)
	if err != nil {
		return nil, err
	}
	defer implRows.Close()
	for implRows.Next() {
		var toID string
		if err := implRows.Scan(&toID); err != nil {
			return nil, err
		}
		rels.Implements = append(rels.Implements, g.symbolNameFromID(toID))
	}
	if err := implRows.Err(); err != nil {
		return nil, err
	}

	// Get dependencies: distinct symbol names from outgoing edges of the class
	// and its methods (excluding extends/implements, which are already captured)
	depRows, err := g.db.Query(
		`SELECT DISTINCT e.to_id FROM edges e
		 WHERE (e.from_id = ? OR e.from_id IN (
		     SELECT id FROM symbols WHERE parent_id = ?
		 ))
		 AND e.kind NOT IN ('extends', 'implements')
		 AND e.to_id != ?`,
		classID, classID, classID,
	)
	if err != nil {
		return nil, err
	}
	defer depRows.Close()
	for depRows.Next() {
		var toID string
		if err := depRows.Scan(&toID); err != nil {
			return nil, err
		}
		name := g.symbolNameFromID(toID)
		if !containsString(rels.Dependencies, name) {
			rels.Dependencies = append(rels.Dependencies, name)
		}
	}
	if err := depRows.Err(); err != nil {
		return nil, err
	}

	return rels, nil
}

// GetOutgoingCalls returns outgoing call edges from a symbol.
//
// Returns the names of symbols that this symbol calls.
func (g *Graph) GetOutgoingCalls(symbolID string) ([]string, error) {
	rows, err := g.db.Query(
		`SELECT DISTINCT e.to_id FROM edges e
		 WHERE e.from_id = ? AND e.kind = 'calls'`,
		symbolID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var toID string
		if err := rows.Scan(&toID); err != nil {
			return nil, err
		}
		result = append(result, g.symbolNameFromID(toID))
	}
	return result, rows.Err()
}

// GetIncomingCallers returns incoming callers for a symbol, grouped by caller with count.
//
// Returns CallerInfo pairs of (caller_display_name, call_count).
func (g *Graph) GetIncomingCallers(symbolID string) ([]CallerInfo, error) {
	rows, err := g.db.Query(
		`SELECT e.from_id, COUNT(*) as cnt FROM edges e
		 WHERE e.to_id = ? AND e.kind = 'calls'
		 GROUP BY e.from_id
		 ORDER BY cnt DESC`,
		symbolID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []CallerInfo
	for rows.Next() {
		var fromID string
		var count int64
		if err := rows.Scan(&fromID, &count); err != nil {
			return nil, err
		}
		name := g.callerDisplayName(fromID)
		result = append(result, CallerInfo{
			Name:  name,
			Count: int(count),
		})
	}
	return result, rows.Err()
}

// GetAllCallerNames returns all caller names grouped by target symbol ID.
//
// Returns a map: target_id -> vec of caller names. Used during indexing
// to enrich FTS text with relationship context. Caller lists are deduped
// and truncated to 10 entries per symbol to avoid bloating the FTS text.
func (g *Graph) GetAllCallerNames() (map[string][]string, error) {
	rows, err := g.db.Query(
		`SELECT e.to_id, s.name
		 FROM edges e
		 JOIN symbols s ON s.id = e.from_id
		 WHERE e.kind = 'calls'
		 ORDER BY e.to_id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var targetID, callerName string
		if err := rows.Scan(&targetID, &callerName); err != nil {
			return nil, err
		}
		entry := result[targetID]
		// Dedup: only add if not already present
		if !containsString(entry, callerName) {
			result[targetID] = append(entry, callerName)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Truncate long lists to avoid bloating FTS text
	for key, names := range result {
		if len(names) > 10 {
			result[key] = names[:10]
		}
	}

	return result, nil
}

// GetAllCalleeNames returns all callee names grouped by source symbol ID.
//
// Returns a map: source_id -> vec of callee names. Used during indexing
// to enrich FTS text with relationship context. Callee lists are deduped
// and truncated to 10 entries per symbol to avoid bloating the FTS text.
func (g *Graph) GetAllCalleeNames() (map[string][]string, error) {
	rows, err := g.db.Query(
		`SELECT e.from_id, s.name
		 FROM edges e
		 JOIN symbols s ON s.id = e.to_id
		 WHERE e.kind = 'calls'
		 ORDER BY e.from_id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var sourceID, calleeName string
		if err := rows.Scan(&sourceID, &calleeName); err != nil {
			return nil, err
		}
		entry := result[sourceID]
		// Dedup: only add if not already present
		if !containsString(entry, calleeName) {
			result[sourceID] = append(entry, calleeName)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Truncate long lists to avoid bloating FTS text
	for key, names := range result {
		if len(names) > 10 {
			result[key] = names[:10]
		}
	}

	return result, nil
}

// ComputeImportanceScores computes normalized importance scores for all symbols.
//
// Score = incoming_call_count / max_incoming_call_count (0.0-1.0).
// Symbols with no incoming calls get 0.0.
func (g *Graph) ComputeImportanceScores() (map[string]float64, error) {
	rows, err := g.db.Query(
		"SELECT to_id, COUNT(*) as cnt FROM edges WHERE kind = 'calls' GROUP BY to_id",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type idCount struct {
		id    string
		count int
	}
	var entries []idCount
	maxCount := 1
	for rows.Next() {
		var id string
		var cnt int
		if err := rows.Scan(&id, &cnt); err != nil {
			return nil, err
		}
		entries = append(entries, idCount{id, cnt})
		if cnt > maxCount {
			maxCount = cnt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	scores := make(map[string]float64, len(entries))
	for _, e := range entries {
		scores[e.id] = float64(e.count) / float64(maxCount)
	}
	return scores, nil
}

// GetImplementors returns symbols that implement a given interface.
func (g *Graph) GetImplementors(interfaceID string) ([]string, error) {
	rows, err := g.db.Query(
		`SELECT e.from_id FROM edges e
		 WHERE e.to_id = ? AND e.kind = 'implements'`,
		interfaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var fromID string
		if err := rows.Scan(&fromID); err != nil {
			return nil, err
		}
		result = append(result, g.symbolNameFromID(fromID))
	}
	return result, rows.Err()
}

// GetFileSymbols returns all symbols in a file, ordered by line_start.
func (g *Graph) GetFileSymbols(filePath string) ([]Symbol, error) {
	rows, err := g.db.Query(
		"SELECT * FROM symbols WHERE file_path = ? ORDER BY line_start",
		filePath,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Symbol
	for rows.Next() {
		sym, err := scanSymbol(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, sym)
	}
	return result, rows.Err()
}

// EntrypointResult pairs a Symbol with its outgoing call count (fan-out).
type EntrypointResult struct {
	Symbol  Symbol `json:"symbol"`
	FanOut  int    `json:"fan_out"`
}

// GetEntrypoints finds symbols that are entry points -- symbols with zero incoming call edges.
//
// Returns each symbol paired with its outgoing call count (fan-out).
// Only considers functions, methods, and classes. Filters out test files
// (paths containing test or spec).
func (g *Graph) GetEntrypoints() ([]EntrypointResult, error) {
	// Step 1: Get all candidate symbols (functions, methods, classes)
	symRows, err := g.db.Query(
		`SELECT * FROM symbols
		 WHERE kind IN ('function', 'method', 'class')
		 ORDER BY file_path, line_start`,
	)
	if err != nil {
		return nil, err
	}
	defer symRows.Close()

	var allSymbols []Symbol
	for symRows.Next() {
		sym, err := scanSymbol(symRows)
		if err != nil {
			return nil, err
		}
		allSymbols = append(allSymbols, sym)
	}
	if err := symRows.Err(); err != nil {
		return nil, err
	}

	// Step 2: Get all symbol IDs/names that ARE called (targets of call edges).
	calledRows, err := g.db.Query("SELECT DISTINCT to_id FROM edges WHERE kind = 'calls'")
	if err != nil {
		return nil, err
	}
	defer calledRows.Close()

	calledSet := make(map[string]struct{})
	bareCalledNames := make(map[string]struct{})
	for calledRows.Next() {
		var toID string
		if err := calledRows.Scan(&toID); err != nil {
			return nil, err
		}
		calledSet[toID] = struct{}{}
		// Build bare name set for suffix matching
		if idx := strings.LastIndex(toID, "."); idx >= 0 {
			bareCalledNames[toID[idx+1:]] = struct{}{}
		}
	}
	if err := calledRows.Err(); err != nil {
		return nil, err
	}

	// Step 3: Pre-compute outgoing call counts in a single aggregate query.
	outRows, err := g.db.Query(
		"SELECT from_id, COUNT(*) FROM edges WHERE kind = 'calls' GROUP BY from_id",
	)
	if err != nil {
		return nil, err
	}
	defer outRows.Close()

	outgoingCounts := make(map[string]int)
	for outRows.Next() {
		var fromID string
		var count int
		if err := outRows.Scan(&fromID, &count); err != nil {
			return nil, err
		}
		outgoingCounts[fromID] = count
	}
	if err := outRows.Err(); err != nil {
		return nil, err
	}

	// Step 4: Filter -- keep symbols not in the called set.
	var results []EntrypointResult
	for i := range allSymbols {
		sym := &allSymbols[i]
		// Skip test files
		if IsTestFile(sym.FilePath) {
			continue
		}

		// Check if this symbol is called by any edge (3 patterns, all O(1))
		_, byID := calledSet[sym.ID]
		_, byName := calledSet[sym.Name]
		_, byBare := bareCalledNames[sym.Name]
		isCalled := byID || byName || byBare

		if !isCalled {
			outgoing := outgoingCounts[sym.ID]
			results = append(results, EntrypointResult{
				Symbol: *sym,
				FanOut: outgoing,
			})
		}
	}

	return results, nil
}

// FindRefs finds all references to a symbol, with optional kind filtering and limit.
//
// Returns (references, total_count) where total_count is the untruncated
// count used for displaying "N more" in truncated output.
//
// Matches edges where to_id is either:
//   - The exact symbol ID (e.g. src/payments/service.ts::PaymentService::class)
//   - The bare symbol name (e.g. PaymentService)
//   - A relative-path qualified name ending with ::SymbolName
func (g *Graph) FindRefs(symbolName string, kinds []string, limit int) ([]Reference, int, error) {
	symbol, err := g.FindSymbol(symbolName)
	if err != nil {
		return nil, 0, err
	}
	if symbol == nil {
		return nil, 0, fmt.Errorf(
			"Symbol '%s' not found in index.\n"+
				"Tip: Check spelling, or use 'inari find \"%s\"' for semantic search.",
			symbolName, symbolName,
		)
	}

	// Collect all names to match against to_id
	matchNames := []string{symbol.Name, symbol.ID}

	// For classes, also include child method names
	if symbol.Kind == "class" || symbol.Kind == "struct" || symbol.Kind == "interface" {
		methods, err := g.GetMethods(symbol.ID)
		if err != nil {
			return nil, 0, err
		}
		for _, m := range methods {
			matchNames = append(matchNames, m.Name, m.ID)
		}
	}

	return g.findRefsWithNames(matchNames, kinds, limit)
}

// FindRefsGrouped finds references to a symbol, grouped by kind.
//
// Used for class symbols where refs should be displayed in groups
// (instantiated, extended, used as type, imported).
func (g *Graph) FindRefsGrouped(symbolName string, limit int) ([]RefsGroup, int, error) {
	refs, total, err := g.FindRefs(symbolName, nil, limit)
	if err != nil {
		return nil, 0, err
	}

	// Group by kind, preserving insertion order
	var groups []RefsGroup
	for _, r := range refs {
		found := false
		for i := range groups {
			if groups[i].Kind == r.Kind {
				groups[i].Refs = append(groups[i].Refs, r)
				found = true
				break
			}
		}
		if !found {
			groups = append(groups, RefsGroup{Kind: r.Kind, Refs: []Reference{r}})
		}
	}

	return groups, total, nil
}

// RefsGroup holds references grouped by kind.
type RefsGroup struct {
	Kind string      `json:"kind"`
	Refs []Reference `json:"refs"`
}

// FindFileRefs finds all references to symbols in a file.
//
// Aggregates refs to every symbol defined in the given file path.
func (g *Graph) FindFileRefs(filePath string, kinds []string, limit int) ([]Reference, int, error) {
	symbols, err := g.GetFileSymbols(filePath)
	if err != nil {
		return nil, 0, err
	}
	if len(symbols) == 0 {
		return nil, 0, fmt.Errorf(
			"No symbols found for file '%s'.\n"+
				"Tip: Check the path is relative to the project root. Run 'inari index' if the file is new.",
			filePath,
		)
	}

	// Collect all names and IDs to match against to_id
	var matchNames []string
	for _, sym := range symbols {
		matchNames = append(matchNames, sym.Name, sym.ID)
	}

	return g.findRefsWithNames(matchNames, kinds, limit)
}

// findRefsWithNames is the internal implementation shared by FindRefs and FindFileRefs.
// It builds dynamic SQL to match edges against a set of symbol names.
func (g *Graph) findRefsWithNames(matchNames []string, kinds []string, limit int) ([]Reference, int, error) {
	// Build the to_id matching clause and parameter list
	matchConditions, params := buildToIDMatchClause(matchNames)

	// Build kind clause
	kindClause := ""
	if len(kinds) > 0 {
		placeholders := make([]string, len(kinds))
		for i, k := range kinds {
			placeholders[i] = "?"
			params = append(params, k)
		}
		kindClause = " AND e.kind IN (" + strings.Join(placeholders, ", ") + ")"
	}

	// Count total
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM edges e WHERE (%s)%s", matchConditions, kindClause)
	var total int64
	err := g.db.QueryRow(countSQL, params...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Fetch refs with limit
	fetchSQL := fmt.Sprintf(
		`SELECT e.from_id, e.kind, e.file_path, e.line
		 FROM edges e
		 WHERE (%s)%s
		 ORDER BY e.kind, e.file_path, e.line
		 LIMIT ?`,
		matchConditions, kindClause,
	)
	fetchParams := make([]interface{}, len(params))
	copy(fetchParams, params)
	fetchParams = append(fetchParams, limit)

	rows, err := g.db.Query(fetchSQL, fetchParams...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var refs []Reference
	for rows.Next() {
		var fromID, kind, filePath string
		var line sql.NullInt64
		if err := rows.Scan(&fromID, &kind, &filePath, &line); err != nil {
			return nil, 0, err
		}
		context := g.callerDisplayName(fromID)
		fromName := g.symbolNameFromID(fromID)
		ref := Reference{
			FromID:   fromID,
			FromName: fromName,
			Kind:     kind,
			FilePath: filePath,
			Context:  context,
		}
		if line.Valid {
			l := line.Int64
			ref.Line = &l
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return refs, int(total), nil
}

// buildToIDMatchClause builds a SQL WHERE clause matching edges by to_id.
//
// Matches exact name, fully-qualified ID suffix (%::Name), and
// dot-separated member calls (%.Name) so that svc.processPayment
// matches when searching for processPayment.
//
// Returns the clause string and the corresponding parameter slice.
func buildToIDMatchClause(names []string) (string, []interface{}) {
	var conditions []string
	var params []interface{}
	for _, name := range names {
		conditions = append(conditions,
			"e.to_id = ? OR e.to_id LIKE ? OR e.to_id LIKE ?",
		)
		params = append(params, name, "%::"+name, "%."+name)
	}
	return strings.Join(conditions, " OR "), params
}

// FindDeps finds dependencies of a symbol (outgoing edges).
//
// For depth 1: returns direct dependencies.
// For depth > 1: uses a recursive CTE to traverse transitive dependencies.
// For classes: includes dependencies from all child methods.
//
// Also includes edges from the __module__ synthetic node for the symbol's
// file, since tree-sitter extractors often attribute edges to the module level.
func (g *Graph) FindDeps(symbolName string, maxDepth int) ([]Dependency, error) {
	symbol, err := g.FindSymbol(symbolName)
	if err != nil {
		return nil, err
	}
	if symbol == nil {
		return nil, fmt.Errorf(
			"Symbol '%s' not found in index.\n"+
				"Tip: Check spelling, or use 'inari find \"%s\"' for semantic search.",
			symbolName, symbolName,
		)
	}

	// Collect source IDs: symbol itself, child methods, and __module__ synthetic IDs
	sourceIDs := []string{symbol.ID}
	if symbol.Kind == "class" || symbol.Kind == "struct" || symbol.Kind == "interface" {
		methods, err := g.GetMethods(symbol.ID)
		if err != nil {
			return nil, err
		}
		for _, m := range methods {
			sourceIDs = append(sourceIDs, m.ID)
		}
	}

	// Also check __module__ synthetic ID for backward compatibility
	moduleID := symbol.FilePath + "::__module__::function"
	if !containsString(sourceIDs, moduleID) {
		sourceIDs = append(sourceIDs, moduleID)
	}

	if maxDepth <= 1 {
		return g.findDirectDeps(sourceIDs)
	}
	return g.findTransitiveDeps(sourceIDs, maxDepth)
}

// FindFileDeps finds dependencies of all symbols in a file.
func (g *Graph) FindFileDeps(filePath string, maxDepth int) ([]Dependency, error) {
	symbols, err := g.GetFileSymbols(filePath)
	if err != nil {
		return nil, err
	}
	if len(symbols) == 0 {
		return nil, fmt.Errorf(
			"No symbols found for file '%s'.\n"+
				"Tip: Check the path is relative to the project root. Run 'inari index' if the file is new.",
			filePath,
		)
	}

	sourceIDs := make([]string, 0, len(symbols)+1)
	for _, s := range symbols {
		sourceIDs = append(sourceIDs, s.ID)
	}

	// Also check __module__ synthetic ID for backward compatibility
	moduleID := filePath + "::__module__::function"
	if !containsString(sourceIDs, moduleID) {
		sourceIDs = append(sourceIDs, moduleID)
	}

	if maxDepth <= 1 {
		return g.findDirectDeps(sourceIDs)
	}
	return g.findTransitiveDeps(sourceIDs, maxDepth)
}

// findDirectDeps gets direct (depth-1) dependencies from a set of source symbol IDs.
func (g *Graph) findDirectDeps(sourceIDs []string) ([]Dependency, error) {
	placeholders := make([]string, len(sourceIDs))
	params := make([]interface{}, len(sourceIDs))
	for i, id := range sourceIDs {
		placeholders[i] = "?"
		params[i] = id
	}
	idClause := strings.Join(placeholders, ", ")

	query := fmt.Sprintf(
		`SELECT DISTINCT e.to_id, e.kind
		 FROM edges e
		 WHERE e.from_id IN (%s)
		 ORDER BY e.kind, e.to_id`,
		idClause,
	)

	rows, err := g.db.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []Dependency
	seen := make(map[string]struct{})

	for rows.Next() {
		var toID, kind string
		if err := rows.Scan(&toID, &kind); err != nil {
			return nil, err
		}

		// Skip self-references
		if containsString(sourceIDs, toID) {
			continue
		}

		// Dedup by (name, kind)
		name := g.symbolNameFromID(toID)
		key := name + "::" + kind
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		// Check if the dep exists in the index
		symInfo, err := g.resolveDepSymbol(toID, name)
		if err != nil {
			return nil, err
		}

		var depName string
		var fp *string
		var isExternal bool
		if symInfo != nil {
			depName = symInfo.name
			fp = &symInfo.filePath
			isExternal = false
		} else {
			depName = name
			isExternal = true
		}

		deps = append(deps, Dependency{
			Name:       depName,
			FilePath:   fp,
			Kind:       kind,
			IsExternal: isExternal,
			Depth:      1,
		})
	}
	return deps, rows.Err()
}

// depSymbolInfo holds resolved symbol information for a dependency.
type depSymbolInfo struct {
	name     string
	filePath string
}

// resolveDepSymbol resolves a dependency target to a symbol in the index.
//
// Tries: exact ID match, then name match (for relative-path style to_ids).
func (g *Graph) resolveDepSymbol(toID, extractedName string) (*depSymbolInfo, error) {
	// Try exact ID match
	var name, filePath string
	err := g.db.QueryRow(
		"SELECT name, file_path FROM symbols WHERE id = ?",
		toID,
	).Scan(&name, &filePath)
	if err == nil {
		return &depSymbolInfo{name: name, filePath: filePath}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Try by name -- prefer top-level symbols (no parent)
	err = g.db.QueryRow(
		`SELECT name, file_path FROM symbols WHERE name = ?
		 ORDER BY (CASE WHEN parent_id IS NULL THEN 0 ELSE 1 END)
		 LIMIT 1`,
		extractedName,
	).Scan(&name, &filePath)
	if err == nil {
		return &depSymbolInfo{name: name, filePath: filePath}, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return nil, err
}

// findTransitiveDeps gets transitive dependencies using a recursive CTE.
func (g *Graph) findTransitiveDeps(sourceIDs []string, maxDepth int) ([]Dependency, error) {
	// Build the seed UNION for all source IDs
	seedConditions := make([]string, len(sourceIDs))
	params := make([]interface{}, 0, len(sourceIDs)+1)
	for i, id := range sourceIDs {
		seedConditions[i] = "SELECT e.to_id, e.kind, 1 FROM edges e WHERE e.from_id = ?"
		params = append(params, id)
	}
	seedUnion := strings.Join(seedConditions, " UNION ALL ")

	params = append(params, maxDepth)

	query := fmt.Sprintf(
		`WITH RECURSIVE deps(id, kind, depth) AS (
			%s
			UNION
			SELECT e.to_id, e.kind, d.depth + 1
			FROM edges e
			JOIN deps d ON e.from_id = d.id
			WHERE d.depth < ?
		)
		SELECT DISTINCT d.id, d.kind, MIN(d.depth) as min_depth
		FROM deps d
		GROUP BY d.id, d.kind
		ORDER BY min_depth, d.kind, d.id`,
		seedUnion,
	)

	rows, err := g.db.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []Dependency
	seen := make(map[string]struct{})

	for rows.Next() {
		var toID, kind string
		var depth int64
		if err := rows.Scan(&toID, &kind, &depth); err != nil {
			return nil, err
		}

		// Skip self-references
		if containsString(sourceIDs, toID) {
			continue
		}

		name := g.symbolNameFromID(toID)
		key := name + "::" + kind
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		symInfo, err := g.resolveDepSymbol(toID, name)
		if err != nil {
			return nil, err
		}

		var depName string
		var fp *string
		var isExternal bool
		if symInfo != nil {
			depName = symInfo.name
			fp = &symInfo.filePath
			isExternal = false
		} else {
			depName = name
			isExternal = true
		}

		deps = append(deps, Dependency{
			Name:       depName,
			FilePath:   fp,
			Kind:       kind,
			IsExternal: isExternal,
			Depth:      int(depth),
		})
	}
	return deps, rows.Err()
}

// IsClassLike checks if a symbol is a class (or struct/interface -- types that get grouped refs).
func (g *Graph) IsClassLike(symbolName string) (bool, error) {
	symbol, err := g.FindSymbol(symbolName)
	if err != nil {
		return false, err
	}
	if symbol == nil {
		return false, nil
	}
	return symbol.Kind == "class" || symbol.Kind == "struct" || symbol.Kind == "interface", nil
}

// FindImpact finds the transitive impact (blast radius) of changing a symbol.
//
// Performs a recursive reverse dependency traversal: finds all symbols
// that directly or transitively depend on the given symbol. Results are
// grouped by depth and test files are separated.
func (g *Graph) FindImpact(symbolName string, maxDepth int) (*ImpactResult, error) {
	symbol, err := g.FindSymbol(symbolName)
	if err != nil {
		return nil, err
	}
	if symbol == nil {
		return nil, fmt.Errorf(
			"Symbol '%s' not found in index.\n"+
				"Tip: Check spelling, or use 'inari find \"%s\"' for semantic search.",
			symbolName, symbolName,
		)
	}

	// Collect all IDs to seed the impact traversal
	seedIDs := []string{symbol.ID}

	// For classes, also include child methods as seeds
	if symbol.Kind == "class" || symbol.Kind == "struct" || symbol.Kind == "interface" {
		methods, err := g.GetMethods(symbol.ID)
		if err != nil {
			return nil, err
		}
		for _, m := range methods {
			seedIDs = append(seedIDs, m.ID)
		}
	}

	return g.runImpactQuery(seedIDs, maxDepth)
}

// FindFileImpact finds the impact of changing any symbol in a file.
//
// Collects all symbols in the file and runs impact analysis for each,
// deduplicating results.
func (g *Graph) FindFileImpact(filePath string, maxDepth int) (*ImpactResult, error) {
	symbols, err := g.GetFileSymbols(filePath)
	if err != nil {
		return nil, err
	}
	if len(symbols) == 0 {
		return nil, fmt.Errorf(
			"No symbols found for file '%s'.\n"+
				"Tip: Check the path is relative to the project root. Run 'inari index' if the file is new.",
			filePath,
		)
	}

	seedIDs := make([]string, len(symbols))
	for i, s := range symbols {
		seedIDs[i] = s.ID
	}
	return g.runImpactQuery(seedIDs, maxDepth)
}

// runImpactQuery executes the recursive CTE impact query for a set of seed symbol IDs.
func (g *Graph) runImpactQuery(seedIDs []string, maxDepth int) (*ImpactResult, error) {
	// Build seed conditions: for each seed ID, match edges where
	// to_id equals the ID exactly, matches the name, or ends with ::Name / .Name
	var seedUnions []string
	var params []interface{}

	for _, seedID := range seedIDs {
		bareName := g.symbolNameFromID(seedID)
		likeQualified := "%::" + bareName
		likeMember := "%." + bareName

		seedUnions = append(seedUnions, fmt.Sprintf(
			`SELECT e.from_id, 1, CAST(e.from_id AS TEXT)
			 FROM edges e WHERE (e.to_id = ? OR e.to_id = ? OR e.to_id LIKE ? OR e.to_id LIKE ?)`,
		))
		params = append(params, seedID, bareName, likeQualified, likeMember)
	}

	seedSQL := strings.Join(seedUnions, " UNION ALL ")
	params = append(params, maxDepth)

	query := fmt.Sprintf(
		`WITH RECURSIVE impact(id, depth, path) AS (
			%s
			UNION ALL
			SELECT e.from_id, i.depth + 1, i.path || ',' || e.from_id
			FROM edges e
			JOIN impact i ON e.to_id = i.id
			WHERE i.depth < ?
			  AND (',' || i.path || ',') NOT LIKE '%%,' || e.from_id || ',%%'
		)
		SELECT DISTINCT i.id, MIN(i.depth) as min_depth, s.name, s.file_path, s.kind
		FROM impact i
		JOIN symbols s ON s.id = i.id
		GROUP BY i.id
		ORDER BY min_depth, s.file_path`,
		seedSQL,
	)

	rows, err := g.db.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Build seed ID set for fast lookup
	seedSet := make(map[string]struct{}, len(seedIDs))
	for _, id := range seedIDs {
		seedSet[id] = struct{}{}
	}

	var testFiles []ImpactNode
	var nonTestNodes []ImpactNode

	for rows.Next() {
		var node ImpactNode
		var minDepth int64
		if err := rows.Scan(&node.ID, &minDepth, &node.Name, &node.FilePath, &node.Kind); err != nil {
			return nil, err
		}
		node.Depth = int(minDepth)

		// Skip seed IDs from appearing in the results
		if _, isSeed := seedSet[node.ID]; isSeed {
			continue
		}

		if IsTestFile(node.FilePath) {
			testFiles = append(testFiles, node)
		} else {
			nonTestNodes = append(nonTestNodes, node)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	totalAffected := len(nonTestNodes)

	// Group non-test nodes by depth (using a sorted map approach)
	depthMap := make(map[int][]ImpactNode)
	for _, node := range nonTestNodes {
		depthMap[node.Depth] = append(depthMap[node.Depth], node)
	}

	// Sort depth keys
	depthKeys := make([]int, 0, len(depthMap))
	for k := range depthMap {
		depthKeys = append(depthKeys, k)
	}
	sort.Ints(depthKeys)

	nodesByDepth := make([]DepthGroup, 0, len(depthKeys))
	for _, depth := range depthKeys {
		nodesByDepth = append(nodesByDepth, DepthGroup{
			Depth: depth,
			Nodes: depthMap[depth],
		})
	}

	return &ImpactResult{
		NodesByDepth:  nodesByDepth,
		TestFiles:     testFiles,
		TotalAffected: totalAffected,
	}, nil
}

// FindCallPaths finds all call paths from entry points to a target symbol.
//
// Walks the call graph backward from the target to discover entry points
// (symbols with no incoming calls edges). Returns all distinct paths
// from each entry point through intermediate callers to the target.
//
// Uses a recursive CTE that tracks the full path as a >-separated
// string to detect cycles and reconstruct paths afterward.
func (g *Graph) FindCallPaths(targetID, targetName string, maxDepth int) (*TraceResult, error) {
	bareName := g.symbolNameFromID(targetID)
	likeQualified := "%::" + bareName
	likeMember := "%." + bareName

	// Use ASCII Unit Separator (0x1F) as path delimiter — cannot appear in symbol names.
	const pathSep = "\x1f"

	query := `
		WITH RECURSIVE trace(id, depth, path) AS (
			-- Seed: direct callers of the target
			SELECT e.from_id, 1, e.from_id || char(31) || ?
			FROM edges e
			WHERE (e.to_id = ? OR e.to_id = ? OR e.to_id LIKE ? OR e.to_id LIKE ?)
			  AND e.kind = 'calls'

			UNION ALL

			-- Walk backward: find who calls the current head of the path
			SELECT e.from_id, t.depth + 1, e.from_id || char(31) || t.path
			FROM edges e
			JOIN trace t ON e.to_id = t.id
			WHERE e.kind = 'calls'
			  AND t.depth < ?
			  AND (char(31) || t.path || char(31)) NOT LIKE '%' || char(31) || e.from_id || char(31) || '%'
		)
		-- Return paths that terminate at entry points (no incoming calls)
		SELECT t.path, t.depth
		FROM trace t
		WHERE NOT EXISTS (
			SELECT 1 FROM edges e2
			WHERE e2.to_id = t.id AND e2.kind = 'calls'
		)
		ORDER BY t.depth, t.path
	`
	_ = pathSep // used below in Split

	rows, err := g.db.Query(query,
		targetID, targetID, bareName, likeQualified, likeMember, maxDepth,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rawPaths []string
	seen := make(map[string]struct{})
	for rows.Next() {
		var path string
		var depth int
		if err := rows.Scan(&path, &depth); err != nil {
			return nil, err
		}
		// Deduplicate paths
		if _, exists := seen[path]; !exists {
			seen[path] = struct{}{}
			rawPaths = append(rawPaths, path)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Resolve each path: split on Unit Separator and look up symbol info for each step
	var paths []CallPath
	for _, rawPath := range rawPaths {
		ids := strings.Split(rawPath, pathSep)
		var steps []CallPathStep
		for _, id := range ids {
			step, err := g.resolveCallPathStep(id)
			if err != nil {
				return nil, err
			}
			steps = append(steps, step)
		}
		if len(steps) > 0 {
			paths = append(paths, CallPath{Steps: steps})
		}
	}

	// Sort: shortest paths first, then alphabetically by first step name
	sort.Slice(paths, func(i, j int) bool {
		if len(paths[i].Steps) != len(paths[j].Steps) {
			return len(paths[i].Steps) < len(paths[j].Steps)
		}
		aName := ""
		if len(paths[i].Steps) > 0 {
			aName = paths[i].Steps[0].SymbolName
		}
		bName := ""
		if len(paths[j].Steps) > 0 {
			bName = paths[j].Steps[0].SymbolName
		}
		return aName < bName
	})

	return &TraceResult{
		Target: targetName,
		Paths:  paths,
	}, nil
}

// resolveCallPathStep resolves a symbol ID to a CallPathStep, falling back to parsing
// the ID format if the symbol is not in the index.
func (g *Graph) resolveCallPathStep(id string) (CallPathStep, error) {
	var step CallPathStep
	err := g.db.QueryRow(
		"SELECT id, name, kind, file_path, line_start FROM symbols WHERE id = ?",
		id,
	).Scan(&step.SymbolID, &step.SymbolName, &step.Kind, &step.FilePath, &step.Line)
	if err == nil {
		return step, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return CallPathStep{}, err
	}

	// Fallback: parse the ID format "file::name::kind"
	parts := strings.Split(id, "::")
	name := id
	if len(parts) >= 2 {
		name = parts[1]
	}

	fp := "unknown"
	if len(parts) >= 1 {
		fp = parts[0]
	}

	kind := "unknown"
	if len(parts) >= 3 {
		kind = parts[2]
	}

	return CallPathStep{
		SymbolID:   id,
		SymbolName: name,
		FilePath:   fp,
		Line:       0,
		Kind:       kind,
	}, nil
}

// InsertFileData inserts a batch of symbols and edges within a single transaction.
//
// Used during indexing to efficiently store all extracted data for a file.
func (g *Graph) InsertFileData(filePath string, symbols []Symbol, edges []Edge) error {
	tx, err := g.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing data for this file
	if _, err := tx.Exec("DELETE FROM edges WHERE file_path = ?", filePath); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM symbols WHERE file_path = ?", filePath); err != nil {
		return err
	}

	// Insert symbols
	symStmt, err := tx.Prepare(
		`INSERT INTO symbols
		 (id, name, kind, file_path, line_start, line_end, signature, docstring, parent_id, language, metadata)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer symStmt.Close()

	for _, sym := range symbols {
		_, err := symStmt.Exec(
			sym.ID, sym.Name, sym.Kind, sym.FilePath,
			sym.LineStart, sym.LineEnd,
			sym.Signature, sym.Docstring, sym.ParentID,
			sym.Language, sym.Metadata,
		)
		if err != nil {
			return err
		}
	}

	// Insert edges
	edgeStmt, err := tx.Prepare(
		`INSERT OR IGNORE INTO edges (from_id, to_id, kind, file_path, line)
		 VALUES (?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer edgeStmt.Close()

	for _, edge := range edges {
		_, err := edgeStmt.Exec(
			edge.FromID, edge.ToID, edge.Kind, edge.FilePath, edge.Line,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteFileData deletes all symbols, edges, and file hash data for a given file path.
func (g *Graph) DeleteFileData(filePath string) error {
	tx, err := g.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM edges WHERE file_path = ?", filePath); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM symbols WHERE file_path = ?", filePath); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM file_hashes WHERE file_path = ?", filePath); err != nil {
		return err
	}

	return tx.Commit()
}

// ClearAll clears all data from the graph (used before a full re-index).
func (g *Graph) ClearAll() error {
	tx, err := g.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM edges"); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM symbols"); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM file_hashes"); err != nil {
		return err
	}

	return tx.Commit()
}

// SymbolCount returns the total number of symbols in the index.
func (g *Graph) SymbolCount() (int, error) {
	var count int64
	err := g.db.QueryRow("SELECT COUNT(*) FROM symbols").Scan(&count)
	return int(count), err
}

// FileCount returns the total number of indexed files.
func (g *Graph) FileCount() (int, error) {
	var count int64
	err := g.db.QueryRow("SELECT COUNT(DISTINCT file_path) FROM symbols").Scan(&count)
	return int(count), err
}

// EdgeCount returns the total number of edges in the index.
func (g *Graph) EdgeCount() (int, error) {
	var count int64
	err := g.db.QueryRow("SELECT COUNT(*) FROM edges").Scan(&count)
	return int(count), err
}

// SymbolImportance pairs a Symbol with its caller count.
type SymbolImportance struct {
	Symbol      Symbol `json:"symbol"`
	CallerCount int    `json:"caller_count"`
}

// GetSymbolsByImportance returns top N symbols by incoming call count (importance).
//
// Returns (Symbol, caller_count) pairs sorted by caller count descending.
// Only considers functions and methods with at least one incoming call edge.
func (g *Graph) GetSymbolsByImportance(limit int) ([]SymbolImportance, error) {
	// Pre-compute all caller counts in a single aggregate query
	callerCounts, err := g.getAllCallerCounts()
	if err != nil {
		return nil, err
	}

	// Fetch all function/method symbols
	rows, err := g.db.Query(
		"SELECT * FROM symbols WHERE kind IN ('function', 'method')",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scored []SymbolImportance
	for rows.Next() {
		sym, err := scanSymbol(rows)
		if err != nil {
			return nil, err
		}
		count := resolveCallerCount(callerCounts, sym.ID, sym.Name)
		if count > 0 {
			scored = append(scored, SymbolImportance{Symbol: sym, CallerCount: count})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort by caller count descending, then by name for deterministic output
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].CallerCount != scored[j].CallerCount {
			return scored[i].CallerCount > scored[j].CallerCount
		}
		return scored[i].Symbol.Name < scored[j].Symbol.Name
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

// getAllCallerCounts fetches all call-edge target counts in a single aggregate query.
//
// Returns two maps for O(1) lookup by all three matching patterns:
//   - byID: exact to_id -> count (covers patterns 1 and 2: exact ID and bare name)
//   - bySuffix: bare name (part after last '.') -> count (covers pattern 3: member-call)
func (g *Graph) getAllCallerCounts() (*callerCountMaps, error) {
	rows, err := g.db.Query(
		"SELECT to_id, COUNT(*) FROM edges WHERE kind = 'calls' GROUP BY to_id",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := make(map[string]int)
	bySuffix := make(map[string]int)

	for rows.Next() {
		var toID string
		var count int64
		if err := rows.Scan(&toID, &count); err != nil {
			return nil, err
		}
		c := int(count)
		// Build suffix map: extract bare name after last '.'
		if idx := strings.LastIndex(toID, "."); idx >= 0 {
			bare := toID[idx+1:]
			if bare != toID {
				bySuffix[bare] += c
			}
		}
		byID[toID] = c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &callerCountMaps{byID: byID, bySuffix: bySuffix}, nil
}

// DirectoryStats holds directory-level statistics.
type DirectoryStats struct {
	Directory   string `json:"directory"`
	FileCount   int    `json:"file_count"`
	SymbolCount int    `json:"symbol_count"`
}

// GetDirectoryStats returns directory-level statistics.
//
// Returns (directory_path, file_count, symbol_count) tuples grouped by
// the top-level directory component (after stripping a leading src/).
func (g *Graph) GetDirectoryStats() ([]DirectoryStats, error) {
	rows, err := g.db.Query("SELECT file_path FROM symbols")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dirFiles := make(map[string]map[string]struct{})
	dirSymbols := make(map[string]int)

	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}

		normalized := strings.ReplaceAll(path, "\\", "/")
		// Strip leading "src/" if present
		stripped := normalized
		if strings.HasPrefix(normalized, "src/") {
			stripped = normalized[4:]
		}

		// Extract top-level directory component
		dir := "(root)"
		if slashPos := strings.Index(stripped, "/"); slashPos >= 0 {
			dir = stripped[:slashPos]
		}

		dirKey := dir + "/"
		if _, exists := dirFiles[dirKey]; !exists {
			dirFiles[dirKey] = make(map[string]struct{})
		}
		dirFiles[dirKey][normalized] = struct{}{}
		dirSymbols[dirKey]++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var results []DirectoryStats
	for dir, files := range dirFiles {
		symCount := dirSymbols[dir]
		results = append(results, DirectoryStats{
			Directory:   dir,
			FileCount:   len(files),
			SymbolCount: symCount,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Directory < results[j].Directory
	})
	return results, nil
}

// GetLanguages returns distinct languages present in the index.
func (g *Graph) GetLanguages() ([]string, error) {
	rows, err := g.db.Query("SELECT DISTINCT language FROM symbols ORDER BY language")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var langs []string
	for rows.Next() {
		var lang string
		if err := rows.Scan(&lang); err != nil {
			return nil, err
		}
		langs = append(langs, lang)
	}
	return langs, rows.Err()
}

// -- File hash operations --

// GetChangedFiles compares current file hashes against the stored index to find changes.
func (g *Graph) GetChangedFiles(currentHashes map[string]string) (*ChangedFiles, error) {
	changed := &ChangedFiles{
		Added:    []string{},
		Modified: []string{},
		Deleted:  []string{},
	}

	// Load stored hashes
	rows, err := g.db.Query("SELECT file_path, hash FROM file_hashes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stored := make(map[string]string)
	for rows.Next() {
		var fp, hash string
		if err := rows.Scan(&fp, &hash); err != nil {
			return nil, err
		}
		stored[fp] = hash
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for path, hash := range currentHashes {
		oldHash, exists := stored[path]
		if !exists {
			changed.Added = append(changed.Added, path)
		} else if oldHash != hash {
			changed.Modified = append(changed.Modified, path)
		}
		// unchanged: do nothing
	}

	for path := range stored {
		if _, exists := currentHashes[path]; !exists {
			changed.Deleted = append(changed.Deleted, path)
		}
	}

	return changed, nil
}

// UpdateFileHashes updates the stored file hashes after indexing.
func (g *Graph) UpdateFileHashes(hashes map[string]string) error {
	now := time.Now().Unix()

	tx, err := g.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT OR REPLACE INTO file_hashes (file_path, hash, indexed_at)
		 VALUES (?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for path, hash := range hashes {
		if _, err := stmt.Exec(path, hash, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// LastIndexedAt returns the most recent indexed_at timestamp from file_hashes.
//
// Returns nil if no files have been indexed yet.
func (g *Graph) LastIndexedAt() (*int64, error) {
	var ts sql.NullInt64
	err := g.db.QueryRow("SELECT MAX(indexed_at) FROM file_hashes").Scan(&ts)
	if err != nil {
		return nil, err
	}
	if !ts.Valid {
		return nil, nil
	}
	return &ts.Int64, nil
}

// IndexedFileCount returns the number of tracked files in the file_hashes table.
func (g *Graph) IndexedFileCount() (int, error) {
	var count int64
	err := g.db.QueryRow("SELECT COUNT(*) FROM file_hashes").Scan(&count)
	return int(count), err
}

// DeleteFileHash deletes a file hash entry.
func (g *Graph) DeleteFileHash(filePath string) error {
	_, err := g.db.Exec("DELETE FROM file_hashes WHERE file_path = ?", filePath)
	return err
}

// -- Internal helpers --

// symbolNameFromID extracts a human-readable name from a symbol ID.
//
// If the ID corresponds to a symbol in the index, returns its name.
// Otherwise, extracts the name portion from the ID format "file::name::kind".
func (g *Graph) symbolNameFromID(id string) string {
	// Try to look up the symbol
	var name string
	err := g.db.QueryRow("SELECT name FROM symbols WHERE id = ?", id).Scan(&name)
	if err == nil {
		return name
	}

	// Fallback: parse the ID format "file::name::kind"
	parts := strings.SplitN(id, "::", 3)
	if len(parts) >= 2 {
		return parts[1]
	}

	return id
}

// callerDisplayName builds a display name for a caller, including parent class if available.
//
// For __module__ synthetic IDs, extracts the file stem.
func (g *Graph) callerDisplayName(fromID string) string {
	// Check if this is a real symbol
	var symName string
	var parentName sql.NullString
	err := g.db.QueryRow(
		`SELECT s.name, p.name FROM symbols s
		 LEFT JOIN symbols p ON s.parent_id = p.id
		 WHERE s.id = ?`,
		fromID,
	).Scan(&symName, &parentName)

	if err == nil {
		if parentName.Valid {
			return parentName.String + "." + symName
		}
		return symName
	}

	// Synthetic ID -- extract something meaningful
	if strings.Contains(fromID, "__module__") {
		// Format: "file_path::__module__::module"
		parts := strings.SplitN(fromID, "::", 2)
		if len(parts) >= 1 {
			filePart := parts[0]
			if idx := strings.LastIndex(filePart, "/"); idx >= 0 {
				filename := filePart[idx+1:]
				if dotIdx := strings.LastIndex(filename, "."); dotIdx >= 0 {
					return filename[:dotIdx]
				}
				return filename
			}
		}
	}

	return g.symbolNameFromID(fromID)
}

// -- Free functions --

// IsTestFile checks if a file path belongs to a test file.
//
// Heuristic: returns true if the lowercase path contains common test path
// segments or test file naming patterns.
func IsTestFile(filePath string) bool {
	lower := strings.ToLower(strings.ReplaceAll(filePath, "\\", "/"))
	return strings.Contains(lower, "/test/") ||
		strings.Contains(lower, "/tests/") ||
		strings.Contains(lower, ".test.") ||
		strings.Contains(lower, ".spec.") ||
		strings.Contains(lower, "_test.") ||
		strings.Contains(lower, "_spec.") ||
		strings.HasPrefix(lower, "test/") ||
		strings.HasPrefix(lower, "tests/")
}

// resolveCallerCount resolves a symbol's caller count from pre-computed maps in O(1).
//
// Matches using the same three patterns as GetCallerCount:
//  1. Exact ID match (O(1) HashMap lookup)
//  2. Bare name match (O(1) HashMap lookup)
//  3. Member-call suffix match via pre-computed suffix map (O(1) lookup)
func resolveCallerCount(maps *callerCountMaps, symbolID, symbolName string) int {
	total := 0
	// Pattern 1: exact ID match
	if c, ok := maps.byID[symbolID]; ok {
		total += c
	}
	// Pattern 2: bare name match (only if different from ID)
	if symbolName != symbolID {
		if c, ok := maps.byID[symbolName]; ok {
			total += c
		}
	}
	// Pattern 3: member-call suffix -- use pre-computed suffix map
	if c, ok := maps.bySuffix[symbolName]; ok {
		total += c
	}
	return total
}

// containsString checks if a string slice contains a specific string.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
