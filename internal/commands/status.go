package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/KilimcininKorOglu/inari/internal/config"
	"github.com/KilimcininKorOglu/inari/internal/core"
	"github.com/KilimcininKorOglu/inari/internal/output"
	"github.com/spf13/cobra"
)

// newStatusCmd creates the `inari status` command.
//
// Quick health check: is the index built? How many symbols and files?
// When was the last index run?
func newStatusCmd() *cobra.Command {
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show index status and freshness",
		Long: `Show index status and freshness.

Quick health check: is the index built? How many symbols and files?
When was the last index run? Use this to check if the index is stale
before running queries.

In workspace mode (--workspace), shows per-member status and aggregate totals.`,
		Example: `  inari status
  inari status --json
  inari status --workspace`,
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
				return runStatusWorkspace(jsonFlag, wsRoot, cfg)
			}

			return runStatusSingle(jsonFlag, cwd)
		},
	}

	cmd.Flags().BoolVarP(&jsonFlag, "json", "j", false, "Output as JSON instead of human-readable format")

	return cmd
}

// runStatusSingle runs the status command for a single project.
func runStatusSingle(jsonOut bool, projectRoot string) error {
	inariDir := filepath.Join(projectRoot, ".inari")

	if _, err := os.Stat(inariDir); os.IsNotExist(err) {
		if jsonOut {
			data := output.StatusData{
				IndexExists:         false,
				SymbolCount:         0,
				FileCount:           0,
				EdgeCount:           0,
				LastIndexedAt:       nil,
				LastIndexedRelative: nil,
			}
			return output.PrintJSON(output.JsonOutput[output.StatusData]{
				Command:   "status",
				Data:      data,
				Truncated: false,
				Total:     0,
			})
		}
		fmt.Println("Index status: not initialised")
		fmt.Println("  Run 'inari init' to set up Inari for this project.")
		return nil
	}

	dbPath := filepath.Join(inariDir, "graph.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if jsonOut {
			data := output.StatusData{
				IndexExists:         false,
				SymbolCount:         0,
				FileCount:           0,
				EdgeCount:           0,
				LastIndexedAt:       nil,
				LastIndexedRelative: nil,
			}
			return output.PrintJSON(output.JsonOutput[output.StatusData]{
				Command:   "status",
				Data:      data,
				Truncated: false,
				Total:     0,
			})
		}
		fmt.Println("Index status: not built")
		fmt.Println("  Run 'inari index' to build the index.")
		return nil
	}

	graph, err := core.Open(dbPath)
	if err != nil {
		return err
	}
	defer graph.Close()

	symbolCount, err := graph.SymbolCount()
	if err != nil {
		return err
	}
	fileCount, err := graph.FileCount()
	if err != nil {
		return err
	}
	edgeCount, err := graph.EdgeCount()
	if err != nil {
		return err
	}
	lastIndexedAt, err := graph.LastIndexedAt()
	if err != nil {
		return err
	}

	var lastIndexedRelative *string
	if lastIndexedAt != nil {
		rel := formatRelativeTime(*lastIndexedAt)
		lastIndexedRelative = &rel
	}

	// Check search index availability.
	searchAvailable := false
	searcher, searchErr := core.OpenSearcher(dbPath)
	if searchErr == nil {
		searchAvailable = true
		searcher.Close()
	}

	if jsonOut {
		data := output.StatusData{
			IndexExists:         true,
			SearchAvailable:     searchAvailable,
			SymbolCount:         symbolCount,
			FileCount:           fileCount,
			EdgeCount:           edgeCount,
			LastIndexedAt:       lastIndexedAt,
			LastIndexedRelative: lastIndexedRelative,
		}
		return output.PrintJSON(output.JsonOutput[output.StatusData]{
			Command:   "status",
			Data:      data,
			Truncated: false,
			Total:     symbolCount,
		})
	}

	data := &output.StatusData{
		IndexExists:         true,
		SearchAvailable:     searchAvailable,
		SymbolCount:         symbolCount,
		FileCount:           fileCount,
		EdgeCount:           edgeCount,
		LastIndexedAt:       lastIndexedAt,
		LastIndexedRelative: lastIndexedRelative,
	}
	output.PrintStatus(data)
	return nil
}

// runStatusWorkspace runs the status command across all workspace members.
func runStatusWorkspace(jsonOut bool, wsRoot string, cfg *config.WorkspaceConfig) error {
	var memberStatuses []output.MemberStatusData
	totalSymbols := 0
	totalFiles := 0
	totalEdges := 0
	var latestIndexed *int64
	allIndexed := true

	for _, entry := range cfg.Workspace.Members {
		name := config.ResolveMemberName(&entry)
		memberPath := filepath.Join(wsRoot, entry.Path)
		dbPath := filepath.Join(memberPath, ".inari", "graph.db")

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			allIndexed = false
			memberStatuses = append(memberStatuses, output.MemberStatusData{
				Name: name,
				Status: output.StatusData{
					IndexExists:         false,
					SymbolCount:         0,
					FileCount:           0,
					EdgeCount:           0,
					LastIndexedAt:       nil,
					LastIndexedRelative: nil,
				},
			})
			continue
		}

		graph, err := core.Open(dbPath)
		if err != nil {
			allIndexed = false
			memberStatuses = append(memberStatuses, output.MemberStatusData{
				Name: name,
				Status: output.StatusData{
					IndexExists:         false,
					SymbolCount:         0,
					FileCount:           0,
					EdgeCount:           0,
					LastIndexedAt:       nil,
					LastIndexedRelative: nil,
				},
			})
			continue
		}

		sc, _ := graph.SymbolCount()
		fc, _ := graph.FileCount()
		ec, _ := graph.EdgeCount()
		lia, _ := graph.LastIndexedAt()
		graph.Close()

		totalSymbols += sc
		totalFiles += fc
		totalEdges += ec

		if lia != nil {
			ts := *lia
			if latestIndexed == nil || ts > *latestIndexed {
				latestIndexed = &ts
			}
		}

		var liaRelative *string
		if lia != nil {
			rel := formatRelativeTime(*lia)
			liaRelative = &rel
		}

		memberStatuses = append(memberStatuses, output.MemberStatusData{
			Name: name,
			Status: output.StatusData{
				IndexExists:         true,
				SymbolCount:         sc,
				FileCount:           fc,
				EdgeCount:           ec,
				LastIndexedAt:       lia,
				LastIndexedRelative: liaRelative,
			},
		})
	}

	var totalsRelative *string
	if latestIndexed != nil {
		rel := formatRelativeTime(*latestIndexed)
		totalsRelative = &rel
	}

	if jsonOut {
		data := output.WorkspaceStatusData{
			WorkspaceName: cfg.Workspace.Name,
			Members:       memberStatuses,
			Totals: output.StatusData{
				IndexExists:         allIndexed,
				SymbolCount:         totalSymbols,
				FileCount:           totalFiles,
				EdgeCount:           totalEdges,
				LastIndexedAt:       latestIndexed,
				LastIndexedRelative: totalsRelative,
			},
		}
		return output.PrintJSON(output.JsonOutput[output.WorkspaceStatusData]{
			Command:   "status",
			Data:      data,
			Truncated: false,
			Total:     totalSymbols,
		})
	}

	data := &output.WorkspaceStatusData{
		WorkspaceName: cfg.Workspace.Name,
		Members:       memberStatuses,
		Totals: output.StatusData{
			IndexExists:         allIndexed,
			SymbolCount:         totalSymbols,
			FileCount:           totalFiles,
			EdgeCount:           totalEdges,
			LastIndexedAt:       latestIndexed,
			LastIndexedRelative: totalsRelative,
		},
	}
	output.PrintWorkspaceStatus(data)
	return nil
}

// formatRelativeTime formats a Unix timestamp as a human-readable relative
// time string.
//
// Examples: "just now", "1 minute ago", "2 minutes ago", "1 hour ago",
// "3 hours ago", "yesterday", "5 days ago".
func formatRelativeTime(unixTS int64) string {
	now := time.Now().Unix()
	delta := now - unixTS

	if delta < 0 {
		return "just now"
	}

	seconds := delta
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24

	switch {
	case seconds < 60:
		return "just now"
	case minutes == 1:
		return "1 minute ago"
	case minutes < 60:
		return fmt.Sprintf("%d minutes ago", minutes)
	case hours == 1:
		return "1 hour ago"
	case hours < 24:
		return fmt.Sprintf("%d hours ago", hours)
	case days == 1:
		return "yesterday"
	default:
		return fmt.Sprintf("%d days ago", days)
	}
}
