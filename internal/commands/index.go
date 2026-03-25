// `inari index` -- build or refresh the code index.
//
// Walks the project's source files, parses them with tree-sitter,
// and stores symbols and edges in the SQLite graph database.
//
// By default, runs incrementally: only re-indexes changed files.
// Use --full to force a complete rebuild.
// Use --watch to keep running and re-index automatically on file changes.
package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/KilimcininKorOglu/inari/internal/config"
	"github.com/KilimcininKorOglu/inari/internal/core"
	"github.com/KilimcininKorOglu/inari/internal/output"
)

// newIndexCmd creates the cobra command for `inari index`.
func newIndexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Build or refresh the code index",
		Long: "Walks all source files, parses them with tree-sitter, and stores\n" +
			"symbols and edges in the local SQLite graph database. First run\n" +
			"is always a full index. Subsequent runs can be incremental.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSingleProjectCommand(cmd, func(root string) error {
				fullFlag, _ := cmd.Flags().GetBool("full")
				watchFlag, _ := cmd.Flags().GetBool("watch")
				jsonFlag, _ := cmd.Flags().GetBool("json")
				return runIndex(root, fullFlag, watchFlag, jsonFlag)
			})
		},
	}
	cmd.Flags().Bool("full", false, "Force a full rebuild of the index (ignore incremental cache)")
	cmd.Flags().BoolP("json", "j", false, "Output as JSON instead of human-readable format")
	cmd.Flags().Bool("watch", false,
		"Watch for file changes and re-index automatically.\n"+
			"With --json, emits NDJSON events to stdout.")
	return cmd
}

// runIndex performs the index logic: full, incremental, or watch mode.
func runIndex(projectRoot string, fullFlag, watchFlag, jsonFlag bool) error {
	inariDir := filepath.Join(projectRoot, ".inari")

	if _, err := os.Stat(inariDir); os.IsNotExist(err) {
		return fmt.Errorf("no .inari/ directory found. Run 'inari init' first")
	}

	// Load config.
	cfg, err := config.LoadProjectConfig(inariDir)
	if err != nil {
		return err
	}

	// Open graph database.
	dbPath := filepath.Join(inariDir, "graph.db")
	graph, err := core.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open graph database: %w", err)
	}
	defer graph.Close()

	// Create indexer.
	indexer, err := core.NewIndexer()
	if err != nil {
		return fmt.Errorf("failed to create indexer: %w", err)
	}

	// Open search index (FTS5) -- optional, skip with warning if it fails.
	searcher, err := core.OpenSearcher(dbPath)
	if err != nil {
		log.Printf("Search index unavailable: %v", err)
		searcher = nil
	}
	if searcher != nil {
		defer searcher.Close()
	}

	if watchFlag {
		return runWatchMode(projectRoot, cfg, graph, indexer, searcher, inariDir, dbPath, fullFlag, jsonFlag)
	}
	if fullFlag {
		return runFullIndex(projectRoot, cfg, graph, indexer, searcher, jsonFlag)
	}
	return runIncrementalIndex(projectRoot, cfg, graph, indexer, searcher, jsonFlag)
}

// runFullIndex performs a full index rebuild.
func runFullIndex(
	projectRoot string,
	cfg *config.ProjectConfig,
	graph *core.Graph,
	indexer *core.Indexer,
	searcher *core.Searcher,
	jsonFlag bool,
) error {
	stats, err := indexer.IndexFull(projectRoot, cfg, graph, searcher)
	if err != nil {
		return err
	}

	if jsonFlag {
		langStats := make([]map[string]interface{}, len(stats.LanguageStats))
		for i, ls := range stats.LanguageStats {
			langStats[i] = map[string]interface{}{
				"language":     ls.Language,
				"file_count":   ls.FileCount,
				"symbol_count": ls.SymbolCount,
			}
		}
		result := map[string]interface{}{
			"command":      "index",
			"mode":         "full",
			"file_count":   stats.FileCount,
			"symbol_count": stats.SymbolCount,
			"edge_count":   stats.EdgeCount,
			"duration_secs": stats.Duration.Seconds(),
			"languages":    langStats,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", data)
		return nil
	}

	// Human-readable output.
	for _, ls := range stats.LanguageStats {
		fmt.Fprintf(os.Stderr, "  %-12s %d files  %d symbols\n",
			ls.Language, ls.FileCount, ls.SymbolCount)
	}
	fmt.Fprintf(os.Stderr, "Built in %.1fs. %d symbols, %d edges.\n",
		stats.Duration.Seconds(), stats.SymbolCount, stats.EdgeCount)

	return nil
}

// runIncrementalIndex performs an incremental index (default mode).
func runIncrementalIndex(
	projectRoot string,
	cfg *config.ProjectConfig,
	graph *core.Graph,
	indexer *core.Indexer,
	searcher *core.Searcher,
	jsonFlag bool,
) error {
	stats, err := indexer.IndexIncremental(projectRoot, cfg, graph, searcher)
	if err != nil {
		return err
	}

	if stats.UpToDate {
		if jsonFlag {
			result := map[string]interface{}{
				"command":    "index",
				"mode":       "incremental",
				"up_to_date": true,
			}
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", data)
		} else {
			fmt.Fprintln(os.Stderr, "Index up to date.")
		}
		return nil
	}

	if jsonFlag {
		result := map[string]interface{}{
			"command":       "index",
			"mode":          "incremental",
			"up_to_date":    false,
			"modified":      stats.Modified,
			"added":         stats.Added,
			"deleted":       stats.Deleted,
			"symbol_count":  stats.SymbolCount,
			"edge_count":    stats.EdgeCount,
			"duration_secs": stats.Duration.Seconds(),
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", data)
		return nil
	}

	output.PrintIncrementalResult(&output.IncrementalStats{
		Modified:     stats.Modified,
		Added:        stats.Added,
		Deleted:      stats.Deleted,
		DurationSecs: stats.Duration.Seconds(),
	})
	return nil
}

// runWatchMode runs watch mode: initial index, then watch for changes.
func runWatchMode(
	projectRoot string,
	cfg *config.ProjectConfig,
	graph *core.Graph,
	indexer *core.Indexer,
	searcher *core.Searcher,
	inariDir string,
	dbPath string,
	fullFlag bool,
	jsonFlag bool,
) error {
	// Acquire watch lock.
	lock := core.NewWatchLock(inariDir)
	if err := lock.Acquire(); err != nil {
		return err
	}
	defer lock.Release()

	// Set up signal handler for clean shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Run initial index.
	if fullFlag {
		stats, err := indexer.IndexFull(projectRoot, cfg, graph, searcher)
		if err != nil {
			return err
		}
		if !jsonFlag {
			for _, ls := range stats.LanguageStats {
				fmt.Fprintf(os.Stderr, "  %-12s %d files  %d symbols\n",
					ls.Language, ls.FileCount, ls.SymbolCount)
			}
			fmt.Fprintf(os.Stderr, "Initial index: %.1fs. %d symbols, %d edges.\n",
				stats.Duration.Seconds(), stats.SymbolCount, stats.EdgeCount)
		}
	} else {
		stats, err := indexer.IndexIncremental(projectRoot, cfg, graph, searcher)
		if err != nil {
			return err
		}
		if !jsonFlag {
			if stats.UpToDate {
				fmt.Fprintln(os.Stderr, "Index up to date.")
			} else {
				total := len(stats.Modified) + len(stats.Added) + len(stats.Deleted)
				fmt.Fprintf(os.Stderr, "Initial index: %.1fs. %d files changed.\n",
					stats.Duration.Seconds(), total)
			}
		}
	}

	// Build supported extensions list from config languages.
	supportedExtensions := getSupportedExtensions(cfg)

	// Emit start event.
	watchStart := time.Now()
	if jsonFlag {
		startEvent := map[string]interface{}{
			"event":     "start",
			"project":   cfg.Project.Name,
			"languages": cfg.Project.Languages,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		data, err := json.Marshal(startEvent)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		fmt.Fprintln(os.Stderr, "Watching for changes... (Ctrl+C to stop)")
	}

	// Create watcher.
	watcher := core.NewWatcher(
		projectRoot,
		cfg.Index.Ignore,
		supportedExtensions,
		300*time.Millisecond,
	)

	eventCh, stopWatcher, err := watcher.Start()
	if err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}
	defer stopWatcher()

	var totalReindexes uint64
	var totalFilesProcessed uint64

	// Event loop.
	for {
		select {
		case <-sigCh:
			// Clean shutdown on Ctrl+C / SIGTERM.
			goto shutdown

		case changedPaths, ok := <-eventCh:
			if !ok {
				// Watcher channel closed.
				log.Println("File watcher disconnected unexpectedly")
				goto shutdown
			}

			batchStart := time.Now()
			filesChanged := len(changedPaths)

			// Re-open graph for each batch to avoid stale connections.
			graph.Close()
			graph, err = core.Open(dbPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to open graph for re-index: %v. Skipping batch.\n", err)
				continue
			}

			// Re-open searcher for each batch.
			var batchSearcher *core.Searcher
			batchSearcher, err = core.OpenSearcher(dbPath)
			if err != nil {
				log.Printf("Search index unavailable for re-index: %v", err)
				batchSearcher = nil
			}

			// Get symbol/edge counts before re-index for delta calculation.
			symbolsBefore, _ := graph.SymbolCount()
			edgesBefore, _ := graph.EdgeCount()

			// Run incremental index -- skip batch on transient failure.
			stats, err := indexer.IndexIncremental(projectRoot, cfg, graph, batchSearcher)
			if batchSearcher != nil {
				batchSearcher.Close()
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: re-index failed: %v. Skipping batch.\n", err)
				continue
			}

			durationMs := time.Since(batchStart).Milliseconds()

			// Compute deltas.
			symbolsAfter := stats.SymbolCount
			edgesAfter := stats.EdgeCount
			symbolsAdded := max(symbolsAfter-symbolsBefore, 0)
			symbolsRemoved := max(symbolsBefore-symbolsAfter, 0)
			edgesAdded := max(edgesAfter-edgesBefore, 0)
			edgesRemoved := max(edgesBefore-edgesAfter, 0)

			totalReindexes++
			totalFilesProcessed += uint64(filesChanged)

			if jsonFlag {
				reindexEvent := map[string]interface{}{
					"event":           "reindex",
					"files_changed":   filesChanged,
					"symbols_added":   symbolsAdded,
					"symbols_removed": symbolsRemoved,
					"edges_added":     edgesAdded,
					"edges_removed":   edgesRemoved,
					"duration_ms":     durationMs,
					"timestamp":       time.Now().UTC().Format(time.RFC3339),
				}
				data, err := json.Marshal(reindexEvent)
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			} else {
				totalChanged := len(stats.Modified) + len(stats.Added) + len(stats.Deleted)
				if totalChanged > 0 {
					suffix := "s"
					if totalChanged == 1 {
						suffix = ""
					}
					fmt.Fprintf(os.Stderr,
						"Re-indexed %d file%s in %dms (%d symbols, %d edges)\n",
						totalChanged, suffix, durationMs, symbolsAfter, edgesAfter)
				}
			}
		}
	}

shutdown:
	// Close the current graph connection (may have been reopened during watch loop).
	graph.Close()

	// Print shutdown summary.
	uptimeSecs := int64(time.Since(watchStart).Seconds())

	if jsonFlag {
		stopEvent := map[string]interface{}{
			"event":                "stop",
			"total_reindexes":      totalReindexes,
			"total_files_processed": totalFilesProcessed,
			"uptime_seconds":       uptimeSecs,
			"timestamp":            time.Now().UTC().Format(time.RFC3339),
		}
		data, err := json.Marshal(stopEvent)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		suffix := "es"
		if totalReindexes == 1 {
			suffix = ""
		}
		fmt.Fprintf(os.Stderr,
			"Stopped. %d re-index%s, %d files processed, uptime %ds.\n",
			totalReindexes, suffix, totalFilesProcessed, uptimeSecs)
	}

	return nil
}

// getSupportedExtensions returns file extensions to watch based on configured languages.
func getSupportedExtensions(cfg *config.ProjectConfig) []string {
	var extensions []string
	for _, lang := range cfg.Project.Languages {
		switch strings.ToLower(lang) {
		case "typescript":
			extensions = append(extensions, "ts", "tsx")
		case "csharp", "c#":
			extensions = append(extensions, "cs")
		case "python":
			extensions = append(extensions, "py")
		case "go":
			extensions = append(extensions, "go")
		case "java":
			extensions = append(extensions, "java")
		case "rust":
			extensions = append(extensions, "rs")
		default:
			log.Printf("Unknown language for watch mode: %s", lang)
		}
	}
	return extensions
}

