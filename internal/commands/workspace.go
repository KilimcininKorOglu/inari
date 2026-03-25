package commands

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/KilimcininKorOglu/inari/internal/config"
	"github.com/KilimcininKorOglu/inari/internal/core"
	"github.com/KilimcininKorOglu/inari/internal/output"
	"github.com/spf13/cobra"
)

// memberStatus holds status information for a single workspace member (list subcommand).
type memberStatus struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	Status        string `json:"status"`
	FileCount     int    `json:"file_count"`
	SymbolCount   int    `json:"symbol_count"`
	LastIndexedAt *int64 `json:"last_indexed_at"`
}

// workspaceListData is the JSON data payload for `inari workspace list`.
type workspaceListData struct {
	WorkspaceName string         `json:"workspace_name"`
	Members       []memberStatus `json:"members"`
}

// memberIndexResult holds the result of indexing a single workspace member.
type memberIndexResult struct {
	Name         string  `json:"name"`
	Path         string  `json:"path"`
	Status       string  `json:"status"`
	Mode         string  `json:"mode"`
	SymbolCount  int     `json:"symbol_count"`
	EdgeCount    int     `json:"edge_count"`
	DurationSecs float64 `json:"duration_secs"`
}

// workspaceIndexData is the JSON data payload for `inari workspace index`.
type workspaceIndexData struct {
	WorkspaceName     string              `json:"workspace_name"`
	Members           []memberIndexResult `json:"members"`
	TotalSymbols      int                 `json:"total_symbols"`
	TotalEdges        int                 `json:"total_edges"`
	TotalDurationSecs float64             `json:"total_duration_secs"`
}

// newWorkspaceCmd creates the `inari workspace` command with subcommands:
// init, list, index.
func newWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Manage multi-project workspaces",
		Long: `Manage multi-project workspaces.

A workspace groups multiple Inari projects (each with its own .inari/
directory) and enables federated queries across all members.

Use 'inari workspace init' to create a workspace manifest by discovering
existing Inari projects in subdirectories. Use 'inari workspace list'
to check the health of all members.`,
	}

	cmd.AddCommand(newWorkspaceInitCmd())
	cmd.AddCommand(newWorkspaceListCmd())
	cmd.AddCommand(newWorkspaceIndexCmd())

	return cmd
}

// ---------------------------------------------------------------------------
// Init subcommand
// ---------------------------------------------------------------------------

// newWorkspaceInitCmd creates the `inari workspace init` subcommand.
func newWorkspaceInitCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Discover projects and create inari-workspace.toml",
		Long: `Discover projects in the current directory tree and create inari-workspace.toml.

Walks subdirectories (max depth 3) looking for .inari/config.toml markers.
Each discovered project becomes a [[workspace.members]] entry.

If projects have not been initialised yet, run 'inari init' in each
project first, then run 'inari workspace init' from the parent directory.`,
		Example: `  inari workspace init
  inari workspace init --name my-workspace`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			return runWorkspaceInit(name, cwd)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Workspace name. Defaults to the current directory name")

	return cmd
}

// runWorkspaceInit discovers projects and writes inari-workspace.toml.
func runWorkspaceInit(wsName string, projectRoot string) error {
	manifestPath := filepath.Join(projectRoot, "inari-workspace.toml")

	if _, err := os.Stat(manifestPath); err == nil {
		return fmt.Errorf("workspace already initialized. Edit inari-workspace.toml directly")
	}

	// Check if the workspace root itself is an Inari project.
	var members []config.MemberInfo
	rootInariDir := filepath.Join(projectRoot, ".inari")
	rootConfigPath := filepath.Join(rootInariDir, "config.toml")
	if _, err := os.Stat(rootConfigPath); err == nil {
		rootName := filepath.Base(projectRoot)
		if cfg, loadErr := config.LoadProjectConfig(rootInariDir); loadErr == nil && cfg.Project.Name != "" {
			rootName = cfg.Project.Name
		}
		members = append(members, config.MemberInfo{
			Path: ".",
			Name: rootName,
		})
	}

	// Discover projects by walking subdirectories (max depth 3).
	if err := discoverProjects(projectRoot, projectRoot, 0, 3, &members); err != nil {
		return err
	}

	if len(members) == 0 {
		return fmt.Errorf(
			"no Inari projects found.\n" +
				"Run 'inari init' in each project directory first, then retry",
		)
	}

	// Sort members by path for deterministic output.
	sort.Slice(members, func(i, j int) bool {
		return members[i].Path < members[j].Path
	})

	// Determine workspace name.
	if wsName == "" {
		wsName = filepath.Base(projectRoot)
		if wsName == "" || wsName == "." || wsName == "/" {
			wsName = "workspace"
		}
	}

	tomlContent := config.GenerateTOML(wsName, members)
	if err := os.WriteFile(manifestPath, []byte(tomlContent), 0644); err != nil {
		return fmt.Errorf("failed to write inari-workspace.toml: %w", err)
	}

	// Report to stderr (progress/info goes to stderr).
	names := make([]string, len(members))
	for i, m := range members {
		names[i] = m.Name
	}
	fmt.Fprintf(os.Stderr, "Found %d projects: %s\n", len(members), strings.Join(names, ", "))
	fmt.Fprintln(os.Stderr, "Created inari-workspace.toml")

	return nil
}

// discoverProjects recursively discovers Inari projects by looking for .inari/config.toml.
func discoverProjects(
	baseRoot string,
	current string,
	depth int,
	maxDepth int,
	members *[]config.MemberInfo,
) error {
	if depth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(current)
	if err != nil {
		log.Printf("Cannot read directory %s: %v. Skipping.", current, err)
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()

		// Skip hidden directories and common non-project dirs.
		if strings.HasPrefix(dirName, ".") ||
			dirName == "node_modules" ||
			dirName == "target" ||
			dirName == "dist" ||
			dirName == "build" {
			continue
		}

		entryPath := filepath.Join(current, dirName)

		// Check if this directory has .inari/config.toml.
		inariConfig := filepath.Join(entryPath, ".inari", "config.toml")
		if _, err := os.Stat(inariConfig); err == nil {
			// Compute relative path from workspace root.
			relPath, err := filepath.Rel(baseRoot, entryPath)
			if err != nil {
				relPath = entryPath
			}
			relPath = strings.ReplaceAll(relPath, "\\", "/")

			*members = append(*members, config.MemberInfo{
				Path: relPath,
				Name: dirName,
			})

			// Don't recurse into discovered projects (they're self-contained).
			continue
		}

		// Recurse into subdirectories.
		if err := discoverProjects(baseRoot, entryPath, depth+1, maxDepth, members); err != nil {
			return err
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// List subcommand
// ---------------------------------------------------------------------------

// newWorkspaceListCmd creates the `inari workspace list` subcommand.
func newWorkspaceListCmd() *cobra.Command {
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show all workspace members and their index status",
		Long: `Show all workspace members and their index status.

Reads inari-workspace.toml and checks each member for .inari/graph.db
existence, symbol count, and last indexed time.

Output columns: name, path, status, files, symbols, last indexed.`,
		Example: `  inari workspace list
  inari workspace list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			return runWorkspaceList(jsonFlag, cwd)
		},
	}

	cmd.Flags().BoolVarP(&jsonFlag, "json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}

// runWorkspaceList shows workspace members and their status.
func runWorkspaceList(jsonOut bool, projectRoot string) error {
	manifestPath, err := findWorkspaceManifest(projectRoot)
	if err != nil {
		return err
	}
	wsRoot := filepath.Dir(manifestPath)

	cfg, err := config.LoadWorkspaceConfig(manifestPath)
	if err != nil {
		return err
	}

	var statuses []memberStatus

	for _, entry := range cfg.Workspace.Members {
		name := config.ResolveMemberName(&entry)
		memberPath := filepath.Join(wsRoot, entry.Path)

		inariDir := filepath.Join(memberPath, ".inari")
		dbPath := filepath.Join(inariDir, "graph.db")

		if _, err := os.Stat(inariDir); os.IsNotExist(err) {
			statuses = append(statuses, memberStatus{
				Name:          name,
				Path:          entry.Path,
				Status:        "not initialised",
				FileCount:     0,
				SymbolCount:   0,
				LastIndexedAt: nil,
			})
			continue
		}

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			statuses = append(statuses, memberStatus{
				Name:          name,
				Path:          entry.Path,
				Status:        "not indexed",
				FileCount:     0,
				SymbolCount:   0,
				LastIndexedAt: nil,
			})
			continue
		}

		graph, err := core.Open(dbPath)
		if err != nil {
			log.Printf("Failed to open graph for member '%s': %v", name, err)
			statuses = append(statuses, memberStatus{
				Name:          name,
				Path:          entry.Path,
				Status:        fmt.Sprintf("error: %v", err),
				FileCount:     0,
				SymbolCount:   0,
				LastIndexedAt: nil,
			})
			continue
		}

		symbolCount, _ := graph.SymbolCount()
		fileCount, _ := graph.FileCount()
		lastIndexedAt, _ := graph.LastIndexedAt()
		graph.Close()

		statuses = append(statuses, memberStatus{
			Name:          name,
			Path:          entry.Path,
			Status:        "indexed",
			FileCount:     fileCount,
			SymbolCount:   symbolCount,
			LastIndexedAt: lastIndexedAt,
		})
	}

	if jsonOut {
		data := workspaceListData{
			WorkspaceName: cfg.Workspace.Name,
			Members:       statuses,
		}
		return output.PrintJSON(output.JsonOutput[workspaceListData]{
			Command:   "workspace list",
			Data:      data,
			Truncated: false,
			Total:     len(cfg.Workspace.Members),
		})
	}

	// Convert to output.MemberListEntry for the formatter.
	listEntries := make([]output.MemberListEntry, len(statuses))
	for i, s := range statuses {
		listEntries[i] = output.MemberListEntry{
			Name:        s.Name,
			Path:        s.Path,
			Status:      s.Status,
			FileCount:   s.FileCount,
			SymbolCount: s.SymbolCount,
		}
	}
	output.PrintWorkspaceList(cfg.Workspace.Name, listEntries)
	return nil
}

// ---------------------------------------------------------------------------
// Index subcommand
// ---------------------------------------------------------------------------

// newWorkspaceIndexCmd creates the `inari workspace index` subcommand.
func newWorkspaceIndexCmd() *cobra.Command {
	var (
		fullFlag  bool
		watchFlag bool
		jsonFlag  bool
	)

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index all workspace members",
		Long: `Index all workspace members.

Reads inari-workspace.toml and indexes each member project sequentially.
Members without .inari/config.toml are skipped with a warning.

With --watch, starts a file watcher for each member that auto re-indexes
on file changes. Press Ctrl+C to stop all watchers.`,
		Example: `  inari workspace index
  inari workspace index --full
  inari workspace index --watch
  inari workspace index --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			if watchFlag {
				return runWorkspaceWatch(cwd)
			}

			return runWorkspaceIndex(fullFlag, jsonFlag, cwd)
		},
	}

	cmd.Flags().BoolVar(&fullFlag, "full", false, "Rebuild the entire index from scratch for all members")
	cmd.Flags().BoolVar(&watchFlag, "watch", false, "Watch all members for file changes and auto re-index")
	cmd.Flags().BoolVarP(&jsonFlag, "json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}

// runWorkspaceIndex indexes all workspace members sequentially.
func runWorkspaceIndex(full bool, jsonOut bool, projectRoot string) error {
	manifestPath, err := findWorkspaceManifest(projectRoot)
	if err != nil {
		return err
	}
	wsRoot := filepath.Dir(manifestPath)

	cfg, err := config.LoadWorkspaceConfig(manifestPath)
	if err != nil {
		return err
	}

	totalMembers := len(cfg.Workspace.Members)
	overallStart := time.Now()

	var results []memberIndexResult
	indexedCount := 0
	totalSymbols := 0
	totalEdges := 0

	for _, entry := range cfg.Workspace.Members {
		name := config.ResolveMemberName(&entry)
		memberPath := filepath.Join(wsRoot, entry.Path)
		inariDir := filepath.Join(memberPath, ".inari")
		configPath := filepath.Join(inariDir, "config.toml")

		mode := "incremental"
		if full {
			mode = "full"
		}

		// Skip members without .inari/config.toml.
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[%s] Skipped: no .inari/config.toml found\n", name)
			results = append(results, memberIndexResult{
				Name:         name,
				Path:         entry.Path,
				Status:       "skipped",
				Mode:         "skipped",
				SymbolCount:  0,
				EdgeCount:    0,
				DurationSecs: 0,
			})
			continue
		}

		memberStart := time.Now()

		// Load project config.
		projectCfg, err := config.LoadProjectConfig(inariDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Error loading config: %v\n", name, err)
			results = append(results, memberIndexResult{
				Name:         name,
				Path:         entry.Path,
				Status:       "error",
				Mode:         mode,
				SymbolCount:  0,
				EdgeCount:    0,
				DurationSecs: time.Since(memberStart).Seconds(),
			})
			continue
		}

		// Open graph database.
		dbPath := filepath.Join(inariDir, "graph.db")
		graph, err := core.Open(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Error opening graph: %v\n", name, err)
			results = append(results, memberIndexResult{
				Name:         name,
				Path:         entry.Path,
				Status:       "error",
				Mode:         mode,
				SymbolCount:  0,
				EdgeCount:    0,
				DurationSecs: time.Since(memberStart).Seconds(),
			})
			continue
		}

		// Create indexer.
		indexer, err := core.NewIndexer()
		if err != nil {
			graph.Close()
			fmt.Fprintf(os.Stderr, "[%s] Error creating indexer: %v\n", name, err)
			results = append(results, memberIndexResult{
				Name:         name,
				Path:         entry.Path,
				Status:       "error",
				Mode:         mode,
				SymbolCount:  0,
				EdgeCount:    0,
				DurationSecs: time.Since(memberStart).Seconds(),
			})
			continue
		}

		// Open search index (optional).
		searcher, searchErr := core.OpenSearcher(dbPath)
		if searchErr != nil {
			log.Printf("[%s] Search index unavailable: %v", name, searchErr)
			searcher = nil
		}

		// Run indexing.
		var symbolCount, edgeCount int
		if full {
			stats, err := indexer.IndexFull(memberPath, projectCfg, graph, searcher)
			if err != nil {
				if searcher != nil {
					searcher.Close()
				}
				graph.Close()
				fmt.Fprintf(os.Stderr, "[%s] Error during indexing: %v\n", name, err)
				results = append(results, memberIndexResult{
					Name:         name,
					Path:         entry.Path,
					Status:       "error",
					Mode:         mode,
					SymbolCount:  0,
					EdgeCount:    0,
					DurationSecs: time.Since(memberStart).Seconds(),
				})
				continue
			}
			symbolCount = stats.SymbolCount
			edgeCount = stats.EdgeCount
		} else {
			stats, err := indexer.IndexIncremental(memberPath, projectCfg, graph, searcher)
			if err != nil {
				if searcher != nil {
					searcher.Close()
				}
				graph.Close()
				fmt.Fprintf(os.Stderr, "[%s] Error during indexing: %v\n", name, err)
				results = append(results, memberIndexResult{
					Name:         name,
					Path:         entry.Path,
					Status:       "error",
					Mode:         mode,
					SymbolCount:  0,
					EdgeCount:    0,
					DurationSecs: time.Since(memberStart).Seconds(),
				})
				continue
			}
			symbolCount = stats.SymbolCount
			edgeCount = stats.EdgeCount
		}

		if searcher != nil {
			searcher.Close()
		}
		graph.Close()

		duration := time.Since(memberStart)
		totalSymbols += symbolCount
		totalEdges += edgeCount
		indexedCount++

		if !jsonOut {
			fmt.Fprintf(os.Stderr, "[%s] Indexed: %d symbols, %d edges (%.1fs)\n",
				name, symbolCount, edgeCount, duration.Seconds())
		}

		results = append(results, memberIndexResult{
			Name:         name,
			Path:         entry.Path,
			Status:       "indexed",
			Mode:         mode,
			SymbolCount:  symbolCount,
			EdgeCount:    edgeCount,
			DurationSecs: duration.Seconds(),
		})
	}

	totalDuration := time.Since(overallStart)

	if jsonOut {
		data := workspaceIndexData{
			WorkspaceName:     cfg.Workspace.Name,
			Members:           results,
			TotalSymbols:      totalSymbols,
			TotalEdges:        totalEdges,
			TotalDurationSecs: totalDuration.Seconds(),
		}
		return output.PrintJSON(output.JsonOutput[workspaceIndexData]{
			Command:   "workspace index",
			Data:      data,
			Truncated: false,
			Total:     totalMembers,
		})
	}

	fmt.Fprintf(os.Stderr, "Workspace indexed: %d/%d members, %d total symbols\n",
		indexedCount, totalMembers, totalSymbols)
	return nil
}

// ---------------------------------------------------------------------------
// Watch mode
// ---------------------------------------------------------------------------

// childProcess represents a running watcher child.
type childProcess struct {
	name    string
	process *exec.Cmd
}

// runWorkspaceWatch spawns an `inari index --watch` child process per member
// and monitors them until Ctrl+C.
func runWorkspaceWatch(projectRoot string) error {
	manifestPath, err := findWorkspaceManifest(projectRoot)
	if err != nil {
		return err
	}
	wsRoot := filepath.Dir(manifestPath)

	cfg, err := config.LoadWorkspaceConfig(manifestPath)
	if err != nil {
		return err
	}

	if len(cfg.Workspace.Members) == 0 {
		return fmt.Errorf("workspace has no members. Add projects to inari-workspace.toml")
	}

	// Find the inari binary path (same binary that's running now).
	inariBin, err := os.Executable()
	if err != nil {
		inariBin = "inari"
	}

	var children []childProcess

	for _, entry := range cfg.Workspace.Members {
		name := config.ResolveMemberName(&entry)
		memberPath := filepath.Join(wsRoot, entry.Path)
		inariDir := filepath.Join(memberPath, ".inari")

		if _, err := os.Stat(filepath.Join(inariDir, "config.toml")); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[%s] Skipped: no .inari/config.toml\n", name)
			continue
		}

		child := exec.Command(inariBin, "index", "--watch")
		child.Dir = memberPath
		child.Stderr = os.Stderr
		child.Stdout = os.Stdout

		if err := child.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Failed to start watcher: %v\n", name, err)
			continue
		}

		fmt.Fprintf(os.Stderr, "[%s] Watcher started (PID %d)\n", name, child.Process.Pid)
		children = append(children, childProcess{name: name, process: child})
	}

	if len(children) == 0 {
		return fmt.Errorf("no watchers started. Ensure workspace members are initialised")
	}

	suffix := "s"
	if len(children) == 1 {
		suffix = ""
	}
	fmt.Fprintf(os.Stderr, "\nWatching %d member%s (Ctrl+C to stop all)...\n", len(children), suffix)

	// Set up signal handler for Ctrl+C.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Also monitor child exits in a goroutine.
	doneCh := make(chan struct{})
	go func() {
		for {
			select {
			case <-doneCh:
				return
			case <-time.After(500 * time.Millisecond):
				for _, c := range children {
					if c.process.ProcessState != nil {
						continue // already exited
					}
					// Check if process is still alive by sending signal 0.
					if c.process.Process != nil {
						if err := c.process.Process.Signal(syscall.Signal(0)); err != nil {
							fmt.Fprintf(os.Stderr, "[%s] Watcher exited unexpectedly\n", c.name)
						}
					}
				}
			}
		}
	}()

	// Wait for signal.
	<-sigCh
	close(doneCh)

	// Kill all children.
	fmt.Fprintln(os.Stderr, "\nShutting down watchers...")
	for _, c := range children {
		if c.process.Process != nil {
			if err := c.process.Process.Kill(); err != nil {
				// Already exited, ignore.
				if !strings.Contains(err.Error(), "process already finished") {
					fmt.Fprintf(os.Stderr, "[%s] Failed to stop: %v\n", c.name, err)
				}
			}
			_ = c.process.Wait()
			fmt.Fprintf(os.Stderr, "[%s] Stopped\n", c.name)
		}
	}

	fmt.Fprintln(os.Stderr, "All watchers stopped.")
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// findWorkspaceManifest walks upward from the given directory looking for
// inari-workspace.toml. Returns the path to the manifest file if found.
func findWorkspaceManifest(start string) (string, error) {
	current := start

	for {
		candidate := filepath.Join(current, "inari-workspace.toml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", fmt.Errorf(
		"no inari-workspace.toml found.\n" +
			"Run 'inari workspace init' to create one",
	)
}
