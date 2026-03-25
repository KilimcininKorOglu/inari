// Package commands provides Cobra command constructors for all Inari CLI commands.
package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KilimcininKorOglu/inari/internal/config"
	"github.com/KilimcininKorOglu/inari/internal/core"
	"github.com/KilimcininKorOglu/inari/internal/output"
	"github.com/spf13/cobra"
)

// newEntrypointsCmd creates the `inari entrypoints` command.
//
// Lists entry points -- symbols with no incoming call edges, grouped by type:
// API Controllers, Background Workers, Event Handlers, and Other.
func newEntrypointsCmd() *cobra.Command {
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "entrypoints",
		Short: "List entry points -- API controllers, workers, and event handlers",
		Long: `List entry points -- symbols with no incoming call edges, grouped by type.

These are the starting points for request flows: HTTP endpoints,
background workers, event handlers, and standalone functions.

In workspace mode (--workspace), shows entry points from all members,
grouped first by project, then by type.`,
		Example: `  inari entrypoints
  inari entrypoints --json
  inari entrypoints --workspace`,
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
				return runEntrypointsWorkspace(jsonFlag, wsRoot, cfg)
			}

			return runEntrypointsSingle(jsonFlag, cwd)
		},
	}

	cmd.Flags().BoolVarP(&jsonFlag, "json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}

// runEntrypointsSingle runs the entrypoints command for a single project.
func runEntrypointsSingle(jsonOut bool, projectRoot string) error {
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

	rawEntrypoints, err := graph.GetEntrypoints()
	if err != nil {
		return err
	}

	groups, total, fileCount := collapseAndGroup(rawEntrypoints, graph)

	if jsonOut {
		return output.PrintJSON(output.JsonOutput[[]output.EntrypointGroup]{
			Command:   "entrypoints",
			Data:      groups,
			Truncated: false,
			Total:     total,
		})
	}

	output.PrintEntrypoints(groups, total, fileCount)
	return nil
}

// runEntrypointsWorkspace runs entrypoints across all workspace members.
func runEntrypointsWorkspace(jsonOut bool, wsRoot string, cfg *config.WorkspaceConfig) error {
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

	wsEntrypoints := wg.GetEntrypoints()

	var allGroups []output.EntrypointGroup
	total := 0
	fileCount := 0

	groupOrder := []string{"API Controllers", "Background Workers", "Event Handlers", "Other"}

	for _, wep := range wsEntrypoints {
		var infos []output.EntrypointInfo
		uniqueFiles := make(map[string]struct{})

		for _, ep := range wep.Entries {
			info := output.EntrypointInfo{
				Name:              fmt.Sprintf("%s::%s", wep.Project, ep.Symbol.Name),
				FilePath:          ep.Symbol.FilePath,
				MethodCount:       0,
				OutgoingCallCount: ep.FanOut,
				Kind:              ep.Symbol.Kind,
			}
			infos = append(infos, info)
			uniqueFiles[ep.Symbol.FilePath] = struct{}{}
		}

		fileCount += len(uniqueFiles)
		total += len(infos)

		for _, groupName := range groupOrder {
			var members []output.EntrypointInfo
			for _, e := range infos {
				if classifyGroup(e.FilePath) == groupName {
					members = append(members, e)
				}
			}
			if len(members) > 0 {
				label := fmt.Sprintf("%s \u2014 %s", wep.Project, groupName)
				allGroups = append(allGroups, output.EntrypointGroup{
					Name:    label,
					Entries: members,
				})
			}
		}
	}

	if jsonOut {
		return output.PrintJSON(output.JsonOutput[[]output.EntrypointGroup]{
			Command:   "entrypoints",
			Data:      allGroups,
			Truncated: false,
			Total:     total,
		})
	}

	output.PrintWorkspaceEntrypoints(allGroups, total, fileCount)
	return nil
}

// classifyGroup classifies an entry point into a group based on its file path.
func classifyGroup(filePath string) string {
	lower := strings.ToLower(filePath)
	if strings.Contains(lower, "controller") {
		return "API Controllers"
	}
	if strings.Contains(lower, "worker") || strings.Contains(lower, "job") {
		return "Background Workers"
	}
	if strings.Contains(lower, "handler") || strings.Contains(lower, "listener") {
		return "Event Handlers"
	}
	return "Other"
}

// collapseAndGroup collapses class-level entries (counts child methods, skips
// individual methods belonging to entry-point classes), then groups by
// classification (API Controllers, Background Workers, Event Handlers, Other).
//
// Returns (groups, totalCount, fileCount).
func collapseAndGroup(raw []core.EntrypointResult, graph *core.Graph) ([]output.EntrypointGroup, int, int) {
	classIDs := make(map[string]struct{})
	classMethodCounts := make(map[string]int)

	for _, ep := range raw {
		if ep.Symbol.Kind == "class" {
			classIDs[ep.Symbol.ID] = struct{}{}
			methods, err := graph.GetMethods(ep.Symbol.ID)
			if err == nil {
				classMethodCounts[ep.Symbol.ID] = len(methods)
			}
		}
	}

	var infos []output.EntrypointInfo
	for _, ep := range raw {
		// Skip methods that belong to an entry-point class.
		if ep.Symbol.ParentID != nil {
			if _, ok := classIDs[*ep.Symbol.ParentID]; ok {
				continue
			}
		}

		methodCount := classMethodCounts[ep.Symbol.ID] // 0 if not a class

		infos = append(infos, output.EntrypointInfo{
			Name:              ep.Symbol.Name,
			FilePath:          ep.Symbol.FilePath,
			MethodCount:       methodCount,
			OutgoingCallCount: ep.FanOut,
			Kind:              ep.Symbol.Kind,
		})
	}

	uniqueFiles := make(map[string]struct{})
	for _, e := range infos {
		uniqueFiles[e.FilePath] = struct{}{}
	}
	fileCount := len(uniqueFiles)
	total := len(infos)

	groupOrder := []string{"API Controllers", "Background Workers", "Event Handlers", "Other"}
	var groups []output.EntrypointGroup

	for _, groupName := range groupOrder {
		var members []output.EntrypointInfo
		for _, e := range infos {
			if classifyGroup(e.FilePath) == groupName {
				members = append(members, e)
			}
		}
		if len(members) > 0 {
			groups = append(groups, output.EntrypointGroup{
				Name:    groupName,
				Entries: members,
			})
		}
	}

	return groups, total, fileCount
}
