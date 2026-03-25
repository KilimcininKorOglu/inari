// Indexer orchestrates full and incremental indexing of a codebase.
//
// Walks the file tree, parses source files via tree-sitter, and stores
// symbols and edges in the SQLite graph database. Optionally updates the
// FTS5 search index with relationship-enriched text.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/KilimcininKorOglu/inari/internal/config"
	"github.com/KilimcininKorOglu/inari/internal/languages"
)

// IndexStats holds statistics from a full indexing run.
type IndexStats struct {
	// Number of files indexed.
	FileCount int
	// Number of symbols extracted.
	SymbolCount int
	// Number of edges extracted.
	EdgeCount int
	// Duration of the indexing run.
	Duration time.Duration
	// Per-language breakdown.
	LanguageStats []LanguageStat
}

// IncrementalStats holds statistics from an incremental indexing run.
type IncrementalStats struct {
	// Files that were modified.
	Modified []string
	// Files that were added.
	Added []string
	// Files that were deleted.
	Deleted []string
	// Total symbols after update.
	SymbolCount int
	// Total edges after update.
	EdgeCount int
	// Duration of the indexing run.
	Duration time.Duration
	// True if nothing changed (index up to date).
	UpToDate bool
}

// LanguageStat holds per-language statistics from an indexing run.
type LanguageStat struct {
	// Language name.
	Language string
	// Number of files of this language.
	FileCount int
	// Number of symbols extracted from this language.
	SymbolCount int
}

// Indexer orchestrates parsing and storage of code symbols and edges.
type Indexer struct {
	parser *CodeParser
}

// NewIndexer creates a new indexer with all supported language plugins registered.
func NewIndexer() (*Indexer, error) {
	parser, err := NewCodeParser()
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	// Register all supported language plugins.
	plugins := []languages.LanguagePlugin{
		&languages.TypeScriptPlugin{},
		&languages.CSharpPlugin{},
		&languages.PythonPlugin{},
		&languages.RustPlugin{},
	}

	for _, plugin := range plugins {
		if err := parser.RegisterPlugin(plugin); err != nil {
			return nil, fmt.Errorf("failed to register %s plugin: %w", plugin.Language().String(), err)
		}
	}

	return &Indexer{parser: parser}, nil
}

// IndexFull performs a full index of the project.
//
// Clears all existing data and re-indexes every supported file.
// If searcher is non-nil, symbols are also indexed for full-text search
// with caller/callee enrichment done after all edges are in the graph.
func (idx *Indexer) IndexFull(
	projectRoot string,
	cfg *config.ProjectConfig,
	graph *Graph,
	searcher *Searcher,
) (*IndexStats, error) {
	start := time.Now()

	// Clear existing data.
	if err := graph.ClearAll(); err != nil {
		return nil, fmt.Errorf("failed to clear graph: %w", err)
	}
	if searcher != nil {
		if err := searcher.ClearAll(); err != nil {
			log.Printf("warning: failed to clear search index: %v", err)
		}
	}

	// Walk the file tree.
	files, err := idx.collectFiles(projectRoot, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to collect files: %w", err)
	}

	totalFiles := len(files)
	fmt.Fprintf(os.Stderr, "Indexing %d files...\n", totalFiles)

	var totalSymbols int
	var totalEdges int
	fileHashes := make(map[string]string)
	langStats := make(map[string][2]int) // language -> [fileCount, symbolCount]
	var allSymbols []Symbol

	for relPath, fileInfo := range files {
		source, err := os.ReadFile(fileInfo.absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", fileInfo.absPath, err)
		}

		// Compute file hash.
		hash := computeHash(source)
		fileHashes[relPath] = hash

		// Extract symbols and edges.
		symbols, err := idx.parser.ExtractSymbols(relPath, source, fileInfo.lang)
		if err != nil {
			return nil, fmt.Errorf("failed to extract symbols from %s: %w", relPath, err)
		}

		edges, err := idx.parser.ExtractEdges(relPath, source, fileInfo.lang)
		if err != nil {
			return nil, fmt.Errorf("failed to extract edges from %s: %w", relPath, err)
		}

		symCount := len(symbols)
		edgeCount := len(edges)

		// Store in graph.
		if err := graph.InsertFileData(relPath, symbols, edges); err != nil {
			return nil, fmt.Errorf("failed to insert data for %s: %w", relPath, err)
		}

		// Collect symbols for FTS indexing (done after all edges are in the graph).
		if searcher != nil {
			allSymbols = append(allSymbols, symbols...)
		}

		totalSymbols += symCount
		totalEdges += edgeCount

		langName := fileInfo.lang.AsStr()
		entry := langStats[langName]
		entry[0]++
		entry[1] += symCount
		langStats[langName] = entry
	}

	// Index symbols for full-text search with relationship context.
	// Done after all symbols and edges are in the graph so that
	// caller/callee relationships are available for cross-file enrichment.
	if searcher != nil {
		callers, err := graph.GetAllCallerNames()
		if err != nil {
			callers = make(map[string][]string)
		}
		callees, err := graph.GetAllCalleeNames()
		if err != nil {
			callees = make(map[string][]string)
		}
		importanceScores, err := graph.ComputeImportanceScores()
		if err != nil {
			return nil, fmt.Errorf("failed to compute importance scores: %w", err)
		}
		if err := searcher.IndexSymbols(allSymbols, callers, callees, importanceScores); err != nil {
			log.Printf("warning: failed to index symbols for search: %v", err)
		}
	}

	// Update file hashes.
	if err := graph.UpdateFileHashes(fileHashes); err != nil {
		return nil, fmt.Errorf("failed to update file hashes: %w", err)
	}

	duration := time.Since(start)

	languageStats := make([]LanguageStat, 0, len(langStats))
	for lang, counts := range langStats {
		languageStats = append(languageStats, LanguageStat{
			Language:    lang,
			FileCount:   counts[0],
			SymbolCount: counts[1],
		})
	}

	return &IndexStats{
		FileCount:     totalFiles,
		SymbolCount:   totalSymbols,
		EdgeCount:     totalEdges,
		Duration:      duration,
		LanguageStats: languageStats,
	}, nil
}

// IndexIncremental performs an incremental index of the project.
//
// Compares file hashes to detect added, modified, and deleted files.
// Only re-parses changed files. Returns early if nothing changed.
// If searcher is non-nil, the search index is updated in sync.
func (idx *Indexer) IndexIncremental(
	projectRoot string,
	cfg *config.ProjectConfig,
	graph *Graph,
	searcher *Searcher,
) (*IncrementalStats, error) {
	start := time.Now()

	// Collect all current files and compute hashes.
	files, err := idx.collectFiles(projectRoot, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to collect files: %w", err)
	}

	currentHashes := make(map[string]string, len(files))
	for relPath, fileInfo := range files {
		source, err := os.ReadFile(fileInfo.absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", fileInfo.absPath, err)
		}
		hash := computeHash(source)
		currentHashes[relPath] = hash
	}

	// Compare against stored hashes.
	changed, err := graph.GetChangedFiles(currentHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	if changed.IsEmpty() {
		return &IncrementalStats{
			UpToDate: true,
			Duration: time.Since(start),
		}, nil
	}

	// Process deleted files.
	for _, filePath := range changed.Deleted {
		if err := graph.DeleteFileData(filePath); err != nil {
			return nil, fmt.Errorf("failed to delete data for %s: %w", filePath, err)
		}
		if searcher != nil {
			if err := searcher.DeleteFile(filePath); err != nil {
				log.Printf("warning: failed to remove search entries for %s: %v", filePath, err)
			}
		}
	}

	// Process modified and added files.
	filesToReindex := make([]string, 0, len(changed.Modified)+len(changed.Added))
	filesToReindex = append(filesToReindex, changed.Modified...)
	filesToReindex = append(filesToReindex, changed.Added...)

	updatedHashes := make(map[string]string)
	var allReindexedSymbols []Symbol

	for _, relPath := range filesToReindex {
		fileInfo, ok := files[relPath]
		if !ok {
			return nil, fmt.Errorf("file %s not found in file map", relPath)
		}

		source, err := os.ReadFile(fileInfo.absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", fileInfo.absPath, err)
		}

		symbols, err := idx.parser.ExtractSymbols(relPath, source, fileInfo.lang)
		if err != nil {
			return nil, fmt.Errorf("failed to extract symbols from %s: %w", relPath, err)
		}

		edges, err := idx.parser.ExtractEdges(relPath, source, fileInfo.lang)
		if err != nil {
			return nil, fmt.Errorf("failed to extract edges from %s: %w", relPath, err)
		}

		// Atomic per-file update: delete old data, insert new.
		if err := graph.InsertFileData(relPath, symbols, edges); err != nil {
			return nil, fmt.Errorf("failed to insert data for %s: %w", relPath, err)
		}

		// Delete old search entries for this file.
		if searcher != nil {
			if err := searcher.DeleteFile(relPath); err != nil {
				log.Printf("warning: failed to clear search entries for %s: %v", relPath, err)
			}
		}

		// Collect symbols for FTS re-indexing with relationship context.
		if searcher != nil {
			allReindexedSymbols = append(allReindexedSymbols, symbols...)
		}

		// Track the hash for this file.
		if hash, ok := currentHashes[relPath]; ok {
			updatedHashes[relPath] = hash
		}
	}

	// Re-index FTS for changed files with relationship context from graph.
	if searcher != nil {
		callers, err := graph.GetAllCallerNames()
		if err != nil {
			callers = make(map[string][]string)
		}
		callees, err := graph.GetAllCalleeNames()
		if err != nil {
			callees = make(map[string][]string)
		}
		importanceScores, err := graph.ComputeImportanceScores()
		if err != nil {
			return nil, fmt.Errorf("failed to compute importance scores: %w", err)
		}
		if err := searcher.IndexSymbols(allReindexedSymbols, callers, callees, importanceScores); err != nil {
			log.Printf("warning: failed to index symbols for search: %v", err)
		}
	}

	// Update file hashes for changed/added files.
	if err := graph.UpdateFileHashes(updatedHashes); err != nil {
		return nil, fmt.Errorf("failed to update file hashes: %w", err)
	}

	duration := time.Since(start)

	symCount, err := graph.SymbolCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get symbol count: %w", err)
	}

	edgeCount, err := graph.EdgeCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get edge count: %w", err)
	}

	return &IncrementalStats{
		Modified:    changed.Modified,
		Added:       changed.Added,
		Deleted:     changed.Deleted,
		SymbolCount: symCount,
		EdgeCount:   edgeCount,
		Duration:    duration,
		UpToDate:    false,
	}, nil
}

// fileInfo holds resolved information about a source file.
type fileInfo struct {
	absPath string
	lang    languages.SupportedLanguage
}

// collectFiles walks the project file tree and returns a map of relative path
// to file info (absolute path and detected language). Respects .gitignore
// patterns, config ignore patterns, and skips nested projects.
func (idx *Indexer) collectFiles(
	projectRoot string,
	cfg *config.ProjectConfig,
) (map[string]fileInfo, error) {
	result := make(map[string]fileInfo)

	// Parse .gitignore patterns.
	gitignorePatterns := parseGitignore(projectRoot)

	// Pre-scan for nested projects (subdirectories with their own .inari/config.toml).
	nestedRoots := findNestedProjects(projectRoot)

	// Build a set of configured languages for fast lookup.
	configuredLangs := make(map[string]bool, len(cfg.Project.Languages))
	for _, lang := range cfg.Project.Languages {
		configuredLangs[strings.ToLower(lang)] = true
	}

	err := filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible entries.
		}

		// Compute relative path with forward slashes.
		relPath, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return nil
		}
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		// Skip hidden directories at the walk level.
		if d.IsDir() {
			baseName := d.Name()

			// Skip hidden directories.
			if strings.HasPrefix(baseName, ".") {
				return filepath.SkipDir
			}

			// Skip .inari directory.
			if baseName == ".inari" {
				return filepath.SkipDir
			}

			// Skip directories matching config ignore or .gitignore patterns.
			if shouldIgnoreByPatterns(cfg.Index.Ignore, relPath, baseName) {
				return filepath.SkipDir
			}
			if shouldIgnoreByPatterns(gitignorePatterns, relPath, baseName) {
				return filepath.SkipDir
			}

			// Skip nested projects (subdirs with their own .inari/config.toml).
			for _, nested := range nestedRoots {
				if path == nested {
					return filepath.SkipDir
				}
			}

			return nil
		}

		// Only process regular files.
		if !d.Type().IsRegular() {
			return nil
		}

		// Skip files matching .gitignore or config ignore patterns.
		fileBaseName := filepath.Base(path)
		if shouldIgnoreByPatterns(gitignorePatterns, relPath, fileBaseName) {
			return nil
		}
		if shouldIgnoreByPatterns(cfg.Index.Ignore, relPath, fileBaseName) {
			return nil
		}

		// Check if the file is a supported language.
		if !idx.parser.IsSupported(path) {
			return nil
		}

		// Detect language and check if it is configured.
		lang, err := idx.parser.DetectLanguage(path)
		if err != nil {
			return nil
		}

		if !configuredLangs[lang.AsStr()] {
			return nil
		}

		result[relPath] = fileInfo{
			absPath: path,
			lang:    lang,
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk project tree: %w", err)
	}

	return result, nil
}

// parseGitignore reads the .gitignore file in the project root and returns
// the list of ignore patterns. Returns an empty slice if .gitignore does
// not exist or cannot be read.
func parseGitignore(projectRoot string) []string {
	gitignorePath := filepath.Join(projectRoot, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		return nil
	}

	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// matchGitignorePattern checks if a relative path matches a gitignore-style pattern.
// Supports simple basename matching, directory patterns (trailing /), and
// glob patterns with *, ?, and [].
func matchGitignorePattern(pattern, relPath string) bool {
	// Normalize pattern: remove trailing slashes for matching.
	cleanPattern := strings.TrimRight(pattern, "/")

	// Negation patterns are handled at a higher level (shouldIgnoreByPatterns).
	if strings.HasPrefix(cleanPattern, "!") {
		cleanPattern = strings.TrimPrefix(cleanPattern, "!")
	}

	// If pattern contains a slash, match against the full relative path.
	if strings.Contains(cleanPattern, "/") {
		matched, _ := filepath.Match(cleanPattern, relPath)
		if matched {
			return true
		}
		// Also try matching with ** prefix removed.
		if strings.HasPrefix(cleanPattern, "**/") {
			subPattern := cleanPattern[3:]
			matched, _ = filepath.Match(subPattern, relPath)
			if matched {
				return true
			}
			// Check if any suffix of the path matches.
			parts := strings.Split(relPath, "/")
			for i := range parts {
				suffix := strings.Join(parts[i:], "/")
				matched, _ = filepath.Match(subPattern, suffix)
				if matched {
					return true
				}
			}
		}
		return false
	}

	// Pattern without a slash: match against any path component.
	parts := strings.Split(relPath, "/")
	for _, part := range parts {
		if part == cleanPattern {
			return true
		}
		matched, _ := filepath.Match(cleanPattern, part)
		if matched {
			return true
		}
	}

	return false
}

// shouldIgnoreByPatterns checks if a path should be ignored according to a list
// of gitignore-style patterns. Patterns are evaluated in order: later patterns
// override earlier ones. Negation patterns (prefixed with !) un-ignore a
// previously matched file, matching real gitignore semantics.
func shouldIgnoreByPatterns(patterns []string, relPath, baseName string) bool {
	ignored := false
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "!") {
			// Negation: if it matches, un-ignore.
			negPattern := strings.TrimPrefix(pattern, "!")
			negPattern = strings.TrimRight(negPattern, "/")
			if baseName == negPattern || matchGitignorePattern(negPattern, relPath) {
				ignored = false
			}
		} else {
			if baseName == pattern || matchGitignorePattern(pattern, relPath) {
				ignored = true
			}
		}
	}
	return ignored
}

// findNestedProjects finds subdirectories that contain their own .inari/config.toml.
// These are nested projects that should not be indexed by the parent.
// Only searches 3 levels deep to keep it fast.
func findNestedProjects(projectRoot string) []string {
	var nested []string
	scanForNested(projectRoot, projectRoot, 0, 3, &nested)
	return nested
}

// scanForNested recursively scans for nested .inari/config.toml directories.
func scanForNested(projectRoot, current string, depth, maxDepth int, results *[]string) {
	if depth > maxDepth {
		return
	}

	entries, err := os.ReadDir(current)
	if err != nil {
		log.Printf("warning: cannot read directory %s: %v. Skipping nested scan.", current, err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()

		// Skip hidden dirs and common non-project dirs.
		if strings.HasPrefix(dirName, ".") ||
			dirName == "node_modules" ||
			dirName == "target" ||
			dirName == "dist" ||
			dirName == "build" {
			continue
		}

		path := filepath.Join(current, dirName)

		// Check if this subdir is a nested project.
		inariConfig := filepath.Join(path, ".inari", "config.toml")
		if path != projectRoot {
			if _, err := os.Stat(inariConfig); err == nil {
				*results = append(*results, path)
				// Do not recurse into nested projects.
				continue
			}
		}

		scanForNested(projectRoot, path, depth+1, maxDepth, results)
	}
}

// computeHash returns the SHA-256 hex digest of the given content.
func computeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
