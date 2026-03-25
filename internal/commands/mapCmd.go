package commands

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/KilimcininKorOglu/inari/internal/config"
	"github.com/KilimcininKorOglu/inari/internal/core"
	"github.com/KilimcininKorOglu/inari/internal/output"
	"github.com/spf13/cobra"
)

// mapData is the full JSON data payload for the map command.
type mapData struct {
	// Repository statistics.
	Stats output.MapStats `json:"stats"`
	// Entry points grouped by type.
	Entrypoints []output.EntrypointGroup `json:"entrypoints"`
	// Core symbols by importance.
	CoreSymbols []output.CoreSymbol `json:"core_symbols"`
	// Directory-level architecture.
	Architecture []output.DirStats `json:"architecture"`
}

// newMapCmd creates the `inari map` command.
//
// Shows a structural overview of the entire repository: entry points,
// core symbols ranked by importance, architecture layers, and key statistics.
func newMapCmd() *cobra.Command {
	var (
		limit    int
		jsonFlag bool
	)

	cmd := &cobra.Command{
		Use:   "map",
		Short: "Show a structural overview of the entire repository",
		Long: `Show a structural overview of the entire repository.

Displays entry points, core symbols ranked by importance, architecture
layers, and key statistics. Designed to give an LLM agent a complete
mental model of the codebase in ~500-1000 tokens.

Use this at the start of any complex task to understand the repo before
diving into specific files.`,
		Example: `  inari map
  inari map --limit 5
  inari map --json
  inari map --workspace`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			// Check for workspace mode.
			wsManifest, wsErr := findWorkspaceManifest(cwd)
			if wsErr == nil {
				wsRoot := filepath.Dir(wsManifest)
				cfg, err := config.LoadWorkspaceConfig(wsManifest)
				if err != nil {
					return err
				}
				return runMapWorkspace(limit, jsonFlag, wsRoot, cfg)
			}

			return runMapSingle(limit, jsonFlag, cwd)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum symbols to show in the core symbols section")
	cmd.Flags().BoolVarP(&jsonFlag, "json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}

// runMapSingle runs the map command for a single project.
func runMapSingle(limit int, jsonOut bool, projectRoot string) error {
	inariDir := filepath.Join(projectRoot, ".inari")

	if _, err := os.Stat(inariDir); os.IsNotExist(err) {
		return fmt.Errorf("no .inari/ directory found. Run 'inari init' first")
	}

	dbPath := filepath.Join(inariDir, "graph.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("no index found. Run 'inari index' to build one first")
	}

	graph, err := core.Open(dbPath)
	if err != nil {
		return err
	}
	defer graph.Close()

	// 1. Gather statistics.
	fileCount, err := graph.FileCount()
	if err != nil {
		return err
	}
	symbolCount, err := graph.SymbolCount()
	if err != nil {
		return err
	}
	edgeCount, err := graph.EdgeCount()
	if err != nil {
		return err
	}
	languages, err := graph.GetLanguages()
	if err != nil {
		return err
	}

	stats := output.MapStats{
		FileCount:   fileCount,
		SymbolCount: symbolCount,
		EdgeCount:   edgeCount,
		Languages:   languages,
	}

	// 2. Get entry points (reuse shared collapse_and_group logic).
	rawEntrypoints, err := graph.GetEntrypoints()
	if err != nil {
		return err
	}
	epGroups, epTotal, _ := collapseAndGroup(rawEntrypoints, graph)

	// 3. Get core symbols by importance.
	rawCore, err := graph.GetSymbolsByImportance(limit)
	if err != nil {
		return err
	}
	coreSymbols := make([]output.CoreSymbol, 0, len(rawCore))
	for _, si := range rawCore {
		coreSymbols = append(coreSymbols, output.CoreSymbol{
			Name:        si.Symbol.Name,
			Kind:        si.Symbol.Kind,
			FilePath:    si.Symbol.FilePath,
			CallerCount: si.CallerCount,
			Project:     nil,
		})
	}

	// 4. Get directory-level architecture.
	rawDirs, err := graph.GetDirectoryStats()
	if err != nil {
		return err
	}
	architecture := make([]output.DirStats, 0, len(rawDirs))
	for _, ds := range rawDirs {
		architecture = append(architecture, output.DirStats{
			Directory:   ds.Directory,
			FileCount:   ds.FileCount,
			SymbolCount: ds.SymbolCount,
		})
	}

	// 5. Detect project name from the directory name.
	projectName := filepath.Base(projectRoot)
	if projectName == "" || projectName == "." || projectName == "/" {
		projectName = "unknown"
	}

	if jsonOut {
		data := mapData{
			Stats:        stats,
			Entrypoints:  epGroups,
			CoreSymbols:  coreSymbols,
			Architecture: architecture,
		}
		return output.PrintJSON(output.JsonOutput[mapData]{
			Command:   "map",
			Data:      data,
			Truncated: false,
			Total:     epTotal,
		})
	}

	output.PrintMap(projectName, &stats, epGroups, coreSymbols, architecture)
	return nil
}

// runMapWorkspace runs the map command across all workspace members.
func runMapWorkspace(limit int, jsonOut bool, wsRoot string, cfg *config.WorkspaceConfig) error {
	var inputs []core.WorkspaceMemberInput
	for _, entry := range cfg.Workspace.Members {
		name := config.ResolveMemberName(&entry)
		memberPath := filepath.Join(wsRoot, entry.Path)
		inputs = append(inputs, core.WorkspaceMemberInput{
			Name: name,
			Root: memberPath,
		})
	}

	wg, err := core.OpenWorkspaceGraph(inputs)
	if err != nil {
		return err
	}
	defer wg.Close()

	// 1. Aggregate statistics.
	stats := output.MapStats{
		FileCount:   wg.FileCount(),
		SymbolCount: wg.SymbolCount(),
		EdgeCount:   wg.EdgeCount(),
		Languages:   wg.GetLanguages(),
	}

	// 2. Get entry points per project.
	wsEntrypoints := wg.GetEntrypoints()

	// Flatten into grouped format: merge all members' entrypoints then group by type.
	type taggedInfo struct {
		info    output.EntrypointInfo
		project string
	}
	var allEPInfos []taggedInfo
	for _, wep := range wsEntrypoints {
		for _, ep := range wep.Entries {
			allEPInfos = append(allEPInfos, taggedInfo{
				info: output.EntrypointInfo{
					Name:              fmt.Sprintf("%s::%s", wep.Project, ep.Symbol.Name),
					FilePath:          ep.Symbol.FilePath,
					MethodCount:       0,
					OutgoingCallCount: ep.FanOut,
					Kind:              ep.Symbol.Kind,
				},
				project: wep.Project,
			})
		}
	}

	// Group by type for display.
	groupOrder := []string{"API Controllers", "Background Workers", "Event Handlers", "Other"}
	var epGroups []output.EntrypointGroup
	for _, groupName := range groupOrder {
		var members []output.EntrypointInfo
		for _, ti := range allEPInfos {
			if classifyGroup(ti.info.FilePath) == groupName {
				members = append(members, ti.info)
			}
		}
		if len(members) > 0 {
			epGroups = append(epGroups, output.EntrypointGroup{
				Name:    groupName,
				Entries: members,
			})
		}
	}
	epTotal := len(allEPInfos)

	// 3. Core symbols from each member, merged and re-sorted.
	perMemberLimit := limit * 2
	var allCore []output.CoreSymbol

	for _, member := range wg.Members() {
		symbols, err := member.Graph.GetSymbolsByImportance(perMemberLimit)
		if err != nil {
			log.Printf("Error getting core symbols from '%s': %v", member.Name, err)
			continue
		}
		for _, si := range symbols {
			project := member.Name
			allCore = append(allCore, output.CoreSymbol{
				Name:        si.Symbol.Name,
				Kind:        si.Symbol.Kind,
				FilePath:    si.Symbol.FilePath,
				CallerCount: si.CallerCount,
				Project:     &project,
			})
		}
	}
	sort.Slice(allCore, func(i, j int) bool {
		return allCore[i].CallerCount > allCore[j].CallerCount
	})
	if len(allCore) > limit {
		allCore = allCore[:limit]
	}

	// 4. Directory stats per project.
	var architecture []output.DirStats
	for _, member := range wg.Members() {
		dirs, err := member.Graph.GetDirectoryStats()
		if err != nil {
			log.Printf("Error getting directory stats from '%s': %v", member.Name, err)
			continue
		}
		for _, ds := range dirs {
			architecture = append(architecture, output.DirStats{
				Directory:   fmt.Sprintf("%s/%s", member.Name, ds.Directory),
				FileCount:   ds.FileCount,
				SymbolCount: ds.SymbolCount,
			})
		}
	}
	sort.Slice(architecture, func(i, j int) bool {
		return architecture[i].SymbolCount > architecture[j].SymbolCount
	})

	if jsonOut {
		data := mapData{
			Stats:        stats,
			Entrypoints:  epGroups,
			CoreSymbols:  allCore,
			Architecture: architecture,
		}
		return output.PrintJSON(output.JsonOutput[mapData]{
			Command:   "map",
			Data:      data,
			Truncated: false,
			Total:     epTotal,
		})
	}

	output.PrintWorkspaceMap(cfg.Workspace.Name, &stats, epGroups, allCore, architecture)
	return nil
}
