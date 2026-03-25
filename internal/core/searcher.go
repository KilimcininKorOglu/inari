// Full-text search over symbol embeddings using SQLite FTS5.
//
// Provides BM25-ranked search for "inari find". Symbols are indexed with
// their name, signature, docstring, and parent context. Queries use
// FTS5 MATCH syntax with porter stemming and unicode tokenisation.
//
// This is the MVP search backend. A future iteration can swap in
// LanceDB + vector embeddings for true semantic search while keeping
// the same SearchResult interface.
package core

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"unicode"

	inariSQL "github.com/KilimcininKorOglu/inari/sql"
	_ "modernc.org/sqlite"
)

// BM25 column weights for FTS5 search ranking.
// Columns: symbol_id(0), name(5), kind(0), file_path(2), body(10).
// Name gets 5x weight, body gets 10x, file_path gets 2x for path-based queries.
const bm25Weights = "0.0, 5.0, 0.0, 2.0, 10.0"

const (
	// scoreEpsilon is the threshold for considering all ranks equal.
	scoreEpsilon = 1e-15
	// scoreFloor is the score when all results have identical rank.
	scoreFloor = 0.95
	// scoreBest is the score for the highest-ranked result.
	scoreBest = 1.0
	// scoreWorst is the score for the lowest-ranked result.
	scoreWorst = 0.5
)

// SearchResult is a single result from a search query.
type SearchResult struct {
	// Symbol ID (matches symbols.id in the graph).
	ID string `json:"id"`
	// Symbol display name.
	Name string `json:"name"`
	// File path relative to project root.
	FilePath string `json:"file_path"`
	// Symbol kind (function, class, method, etc.).
	Kind string `json:"kind"`
	// Relevance score: 0.0-1.0, higher = more relevant.
	Score float64 `json:"score"`
	// Start line of the symbol definition.
	LineStart uint32 `json:"line_start"`
	// End line of the symbol definition.
	LineEnd uint32 `json:"line_end"`
}

// Searcher is an FTS5-backed full-text search engine for symbols.
//
// Operates on the same SQLite database as the graph, using the
// symbols_fts virtual table created in schema.sql.
type Searcher struct {
	db *sql.DB
}

// rawFTSResult is a raw FTS5 result before score normalisation.
type rawFTSResult struct {
	symbolID string
	name     string
	kind     string
	filePath string
	rank     float64
}

// OpenSearcher opens a searcher on the graph database at the given path.
//
// The database schema (including the FTS5 table) is applied if not already present.
func OpenSearcher(dbPath string) (*Searcher, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open search index at %s: %w", dbPath, err)
	}

	// Apply shared performance pragmas (same as graph connection).
	for _, pragma := range sharedPragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma %q: %w", pragma, err)
		}
	}

	// Ensure FTS5 table exists via the shared schema.
	if _, err := db.Exec(inariSQL.SchemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	return &Searcher{db: db}, nil
}

// Close closes the database connection.
func (s *Searcher) Close() error {
	return s.db.Close()
}

// IndexSymbols indexes a batch of symbols into the FTS5 table.
//
// Builds the embedding text for each symbol and inserts it.
// Caller and callee maps provide relationship context for richer search.
// Existing FTS data for the symbols' files is cleared before inserting.
func (s *Searcher) IndexSymbols(
	symbols []Symbol,
	callers map[string][]string,
	callees map[string][]string,
	importance map[string]float64,
) error {
	// Collect distinct file paths to clear existing FTS data.
	filePaths := make(map[string]struct{})
	for i := range symbols {
		filePaths[symbols[i].FilePath] = struct{}{}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing FTS entries for these files.
	delStmt, err := tx.Prepare("DELETE FROM symbols_fts WHERE file_path = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare delete statement: %w", err)
	}
	defer delStmt.Close()

	for fp := range filePaths {
		if _, err := delStmt.Exec(fp); err != nil {
			return fmt.Errorf("failed to delete FTS entries for %s: %w", fp, err)
		}
	}

	// Insert embedding text for each symbol.
	insStmt, err := tx.Prepare(
		"INSERT OR REPLACE INTO symbols_fts (symbol_id, name, kind, file_path, body) VALUES (?, ?, ?, ?, ?)",
	)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer insStmt.Close()

	for i := range symbols {
		sym := &symbols[i]
		symCallers := callers[sym.ID]
		symCallees := callees[sym.ID]
		imp := importance[sym.ID]
		body := BuildEmbeddingText(sym, symCallers, symCallees, imp)

		if _, err := insStmt.Exec(sym.ID, sym.Name, sym.Kind, sym.FilePath, body); err != nil {
			return fmt.Errorf("failed to insert FTS entry for %s: %w", sym.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit FTS index: %w", err)
	}

	return nil
}

// DeleteFile deletes all FTS entries for a given file path.
//
// Used during incremental indexing when a file is removed or re-indexed.
func (s *Searcher) DeleteFile(filePath string) error {
	_, err := s.db.Exec("DELETE FROM symbols_fts WHERE file_path = ?", filePath)
	if err != nil {
		return fmt.Errorf("failed to delete FTS entries for %s: %w", filePath, err)
	}
	return nil
}

// ClearAll deletes all FTS data (used before a full re-index).
func (s *Searcher) ClearAll() error {
	_, err := s.db.Exec("DELETE FROM symbols_fts")
	if err != nil {
		return fmt.Errorf("failed to clear FTS data: %w", err)
	}
	return nil
}

// Search searches for symbols matching a natural-language query.
//
// Uses FTS5 MATCH with BM25 ranking. The query is automatically
// converted to an OR query so that partial matches still surface.
// Results are ranked by relevance and optionally filtered by kind.
// Pass an empty string for kindFilter to skip kind filtering.
func (s *Searcher) Search(query string, limit int, kindFilter string) ([]SearchResult, error) {
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return nil, nil
	}

	// Build SQL with optional kind filter.
	var sqlStr string
	var rows *sql.Rows
	var err error

	if kindFilter != "" {
		sqlStr = `SELECT symbol_id, name, kind, file_path,
		                 bm25(symbols_fts, ` + bm25Weights + `) AS rank
		          FROM symbols_fts
		          WHERE symbols_fts MATCH ? AND kind = ?
		          ORDER BY rank
		          LIMIT ?`
		rows, err = s.db.Query(sqlStr, ftsQuery, kindFilter, limit)
	} else {
		sqlStr = `SELECT symbol_id, name, kind, file_path,
		                 bm25(symbols_fts, ` + bm25Weights + `) AS rank
		          FROM symbols_fts
		          WHERE symbols_fts MATCH ?
		          ORDER BY rank
		          LIMIT ?`
		rows, err = s.db.Query(sqlStr, ftsQuery, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("FTS5 search failed: %w", err)
	}
	defer rows.Close()

	var rawResults []rawFTSResult
	for rows.Next() {
		var r rawFTSResult
		if err := rows.Scan(&r.symbolID, &r.name, &r.kind, &r.filePath, &r.rank); err != nil {
			return nil, fmt.Errorf("failed to scan FTS result: %w", err)
		}
		rawResults = append(rawResults, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("FTS result iteration error: %w", err)
	}

	// Convert BM25 ranks to 0.0-1.0 scores (BM25 returns negative values; lower = better).
	results := normalizeScores(rawResults)

	// Enrich with line numbers from the symbols table.
	enriched, err := s.enrichWithLines(results)
	if err != nil {
		return nil, err
	}

	return enriched, nil
}

// enrichWithLines looks up line_start and line_end for each search result from the symbols table.
func (s *Searcher) enrichWithLines(results []SearchResult) ([]SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	stmt, err := s.db.Prepare("SELECT line_start, line_end FROM symbols WHERE id = ?")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare line lookup: %w", err)
	}
	defer stmt.Close()

	enriched := make([]SearchResult, 0, len(results))
	for _, r := range results {
		var lineStart, lineEnd uint32
		err := stmt.QueryRow(r.ID).Scan(&lineStart, &lineEnd)
		if err != nil {
			// If symbol not found in the symbols table, keep existing values.
			lineStart = r.LineStart
			lineEnd = r.LineEnd
		}

		enriched = append(enriched, SearchResult{
			ID:        r.ID,
			Name:      r.Name,
			FilePath:  r.FilePath,
			Kind:      r.Kind,
			Score:     r.Score,
			LineStart: lineStart,
			LineEnd:   lineEnd,
		})
	}

	return enriched, nil
}

// normalizeScores converts BM25 rank values to 0.0-1.0 similarity scores.
//
// BM25 returns negative values where lower (more negative) = better match.
// We invert and normalise so that higher = more relevant.
func normalizeScores(raw []rawFTSResult) []SearchResult {
	if len(raw) == 0 {
		return nil
	}

	// Find the range of ranks for normalisation.
	minRank := math.Inf(1)
	maxRank := math.Inf(-1)
	for _, r := range raw {
		if r.rank < minRank {
			minRank = r.rank
		}
		if r.rank > maxRank {
			maxRank = r.rank
		}
	}

	rankRange := maxRank - minRank

	results := make([]SearchResult, 0, len(raw))
	for _, r := range raw {
		var score float64
		if math.Abs(rankRange) < scoreEpsilon {
			score = scoreFloor
		} else {
			// Invert: best (most negative) rank -> highest score.
			normalised := (r.rank - minRank) / rankRange
			score = scoreBest - (normalised * (scoreBest - scoreWorst))
		}

		results = append(results, SearchResult{
			ID:       r.symbolID,
			Name:     r.name,
			FilePath: r.filePath,
			Kind:     r.kind,
			Score:    score,
		})
	}

	return results
}

// buildFTSQuery builds an FTS5 query from a natural-language search string.
//
// Splits the query into tokens and joins them with OR for partial matching.
// Each token gets a "*" suffix for prefix matching, and camelCase tokens
// are additionally split into component words for broader recall.
//
// Examples:
//   - "TransactionController" -> "TransactionController* OR transaction* OR controller*"
//   - "payment" -> "payment*"
//   - "authentication errors" -> "authentication* OR errors*"
func buildFTSQuery(query string) string {
	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		return ""
	}

	var ftsTokens []string

	for _, token := range tokens {
		// Filter non-alphanumeric characters (keep underscores).
		var cleaned strings.Builder
		for _, ch := range token {
			if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
				cleaned.WriteRune(ch)
			}
		}
		cleanedStr := cleaned.String()
		if cleanedStr == "" {
			continue
		}

		// Add the full token with prefix matching.
		ftsTokens = append(ftsTokens, cleanedStr+"*")

		// Also split camelCase and add component words (min 3 chars).
		split := SplitCamelCase(cleanedStr)
		for _, word := range strings.Fields(split) {
			lower := strings.ToLower(word)
			if lower != strings.ToLower(cleanedStr) && len(lower) >= 3 {
				ftsTokens = append(ftsTokens, lower+"*")
			}
		}

		// Also split snake_case and add component words.
		if strings.Contains(cleanedStr, "_") {
			for _, word := range strings.Fields(SplitSnakeCase(cleanedStr)) {
				lower := strings.ToLower(word)
				if len(lower) >= 3 {
					ftsTokens = append(ftsTokens, lower+"*")
				}
			}
		}
	}

	// Dedup while preserving order.
	ftsTokens = dedup(ftsTokens)

	return strings.Join(ftsTokens, " OR ")
}

// dedup removes duplicate strings from a slice, preserving order of first occurrence.
func dedup(items []string) []string {
	if len(items) == 0 {
		return items
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}
