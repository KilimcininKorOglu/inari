package runner

import (
	"fmt"
	"math"
	"strings"
)

// BehaviorMetrics holds behavior metrics derived from an agent's action sequence.
type BehaviorMetrics struct {
	ActionsBeforeFirstEdit       uint32   `json:"actions_before_first_edit"`
	NavigationToEditRatio        float64  `json:"navigation_to_edit_ratio"`
	UniqueFilesRead              uint32   `json:"unique_files_read"`
	RedundantReads               uint32   `json:"redundant_reads"`
	TotalActions                 uint32   `json:"total_actions"`
	InariCommandsTotal           uint32   `json:"inari_commands_total"`
	InariCommandsBeforeFirstEdit uint32   `json:"inari_commands_before_first_edit"`
	InariThenReadSameFile        uint32   `json:"inari_then_read_same_file"`
	InariCommandSequence         []string `json:"inari_command_sequence"`
	GrepAfterInariFind           uint32   `json:"grep_after_inari_find"`
	CallersAndRefsSameSymbol     bool     `json:"callers_and_refs_same_symbol"`
}

// BehaviorComparison aggregates behavior metrics between conditions.
type BehaviorComparison struct {
	MeanActionsBeforeEditWith    float64           `json:"mean_actions_before_edit_with"`
	MeanActionsBeforeEditWithout float64           `json:"mean_actions_before_edit_without"`
	MeanNavRatioWith             float64           `json:"mean_nav_ratio_with"`
	MeanNavRatioWithout          float64           `json:"mean_nav_ratio_without"`
	MeanUniqueReadsWith          float64           `json:"mean_unique_reads_with"`
	MeanUniqueReadsWithout       float64           `json:"mean_unique_reads_without"`
	MeanRedundantReadsWith       float64           `json:"mean_redundant_reads_with"`
	MeanRedundantReadsWithout    float64           `json:"mean_redundant_reads_without"`
	MeanInariCommands            float64           `json:"mean_inari_commands"`
	InariAntiPatterns            InariAntiPatterns `json:"inari_anti_patterns"`
}

// InariAntiPatterns counts detected anti-patterns in inari command usage.
type InariAntiPatterns struct {
	SketchThenReadCount uint32 `json:"sketch_then_read_count"`
	GrepAfterFindCount  uint32 `json:"grep_after_find_count"`
	CallersAndRefsCount uint32 `json:"callers_and_refs_count"`
}

// ComputeBehaviorMetrics analyzes the action log to derive navigation efficiency,
// inari usage patterns, and tool overlap metrics.
func ComputeBehaviorMetrics(actions []AgentAction) BehaviorMetrics {
	totalActions := uint32(len(actions))

	// Find first edit index
	firstEditIdx := -1
	for i, a := range actions {
		if a.IsEdit {
			firstEditIdx = i
			break
		}
	}

	// actionsBeforeFirstEdit
	var actionsBeforeFirstEdit uint32
	if firstEditIdx >= 0 {
		actionsBeforeFirstEdit = uint32(firstEditIdx)
	} else {
		actionsBeforeFirstEdit = totalActions
	}

	// navigationToEditRatio
	var navCount, editCount float64
	for _, a := range actions {
		if a.IsNavigation {
			navCount++
		}
		if a.IsEdit {
			editCount++
		}
	}
	var navigationToEditRatio float64
	if editCount == 0 {
		navigationToEditRatio = math.Inf(1)
	} else {
		navigationToEditRatio = navCount / editCount
	}

	// uniqueFilesRead and redundantReads
	readFiles := make([]string, 0)
	uniqueReadSet := make(map[string]struct{})
	for _, a := range actions {
		if a.ToolName == "Read" {
			readFiles = append(readFiles, a.ArgumentsSummary)
			uniqueReadSet[a.ArgumentsSummary] = struct{}{}
		}
	}
	uniqueFilesRead := uint32(len(uniqueReadSet))
	redundantReads := uint32(len(readFiles)) - uniqueFilesRead

	// inariCommandsTotal
	var inariCommandsTotal uint32
	for _, a := range actions {
		if a.IsInariCommand {
			inariCommandsTotal++
		}
	}

	// inariCommandsBeforeFirstEdit
	var inariCommandsBeforeFirstEdit uint32
	if firstEditIdx >= 0 {
		for _, a := range actions[:firstEditIdx] {
			if a.IsInariCommand {
				inariCommandsBeforeFirstEdit++
			}
		}
	} else {
		inariCommandsBeforeFirstEdit = inariCommandsTotal
	}

	// inariThenReadSameFile
	inariThenReadSameFile := computeInariThenRead(actions)

	// inariCommandSequence
	inariCommandSequence := make([]string, 0)
	for _, a := range actions {
		if a.IsInariCommand {
			if sub := extractInariSubcommand(a.ArgumentsSummary); sub != "" {
				inariCommandSequence = append(inariCommandSequence, sub)
			}
		}
	}

	// grepAfterInariFind
	grepAfterInariFind := computeGrepAfterInariFind(actions)

	// callersAndRefsSameSymbol
	callersAndRefsSameSymbol := computeCallersAndRefsOverlap(actions)

	return BehaviorMetrics{
		ActionsBeforeFirstEdit:       actionsBeforeFirstEdit,
		NavigationToEditRatio:        navigationToEditRatio,
		UniqueFilesRead:              uniqueFilesRead,
		RedundantReads:               redundantReads,
		TotalActions:                 totalActions,
		InariCommandsTotal:           inariCommandsTotal,
		InariCommandsBeforeFirstEdit: inariCommandsBeforeFirstEdit,
		InariThenReadSameFile:        inariThenReadSameFile,
		InariCommandSequence:         inariCommandSequence,
		GrepAfterInariFind:           grepAfterInariFind,
		CallersAndRefsSameSymbol:     callersAndRefsSameSymbol,
	}
}

// AggregateBehavior aggregates behavior metrics from with-inari and without-inari
// runs into a comparison.
func AggregateBehavior(withInari, withoutInari []BehaviorMetrics) BehaviorComparison {
	meanActionsBeforeEditWith := meanUint32(withInari, func(m BehaviorMetrics) uint32 { return m.ActionsBeforeFirstEdit })
	meanActionsBeforeEditWithout := meanUint32(withoutInari, func(m BehaviorMetrics) uint32 { return m.ActionsBeforeFirstEdit })

	meanNavRatioWith := meanFloat64Finite(withInari, func(m BehaviorMetrics) float64 { return m.NavigationToEditRatio })
	meanNavRatioWithout := meanFloat64Finite(withoutInari, func(m BehaviorMetrics) float64 { return m.NavigationToEditRatio })

	meanUniqueReadsWith := meanUint32(withInari, func(m BehaviorMetrics) uint32 { return m.UniqueFilesRead })
	meanUniqueReadsWithout := meanUint32(withoutInari, func(m BehaviorMetrics) uint32 { return m.UniqueFilesRead })

	meanRedundantReadsWith := meanUint32(withInari, func(m BehaviorMetrics) uint32 { return m.RedundantReads })
	meanRedundantReadsWithout := meanUint32(withoutInari, func(m BehaviorMetrics) uint32 { return m.RedundantReads })

	meanInariCommands := meanUint32(withInari, func(m BehaviorMetrics) uint32 { return m.InariCommandsTotal })

	var sketchThenReadCount uint32
	var grepAfterFindCount uint32
	var callersAndRefsCount uint32
	for _, m := range withInari {
		sketchThenReadCount += m.InariThenReadSameFile
		grepAfterFindCount += m.GrepAfterInariFind
		if m.CallersAndRefsSameSymbol {
			callersAndRefsCount++
		}
	}

	return BehaviorComparison{
		MeanActionsBeforeEditWith:    meanActionsBeforeEditWith,
		MeanActionsBeforeEditWithout: meanActionsBeforeEditWithout,
		MeanNavRatioWith:             meanNavRatioWith,
		MeanNavRatioWithout:          meanNavRatioWithout,
		MeanUniqueReadsWith:          meanUniqueReadsWith,
		MeanUniqueReadsWithout:       meanUniqueReadsWithout,
		MeanRedundantReadsWith:       meanRedundantReadsWith,
		MeanRedundantReadsWithout:    meanRedundantReadsWithout,
		MeanInariCommands:            meanInariCommands,
		InariAntiPatterns: InariAntiPatterns{
			SketchThenReadCount: sketchThenReadCount,
			GrepAfterFindCount:  grepAfterFindCount,
			CallersAndRefsCount: callersAndRefsCount,
		},
	}
}

// FormatBehaviorMarkdown generates a markdown section summarizing the behavior comparison.
func FormatBehaviorMarkdown(comparison BehaviorComparison) string {
	var out strings.Builder

	out.WriteString("### Agent Behavior Analysis\n\n")

	out.WriteString("#### Navigation Efficiency\n")
	out.WriteString("| Metric                   | With Inari | Without Inari |\n")
	out.WriteString("|--------------------------|-----------|---------------|\n")
	out.WriteString(fmt.Sprintf("| Actions before first edit | %.1f       | %.1f           |\n",
		comparison.MeanActionsBeforeEditWith, comparison.MeanActionsBeforeEditWithout))
	out.WriteString(fmt.Sprintf("| Navigation:edit ratio    | %.1f       | %.1f           |\n",
		comparison.MeanNavRatioWith, comparison.MeanNavRatioWithout))
	out.WriteString(fmt.Sprintf("| Unique files read        | %.1f       | %.1f           |\n",
		comparison.MeanUniqueReadsWith, comparison.MeanUniqueReadsWithout))
	out.WriteString(fmt.Sprintf("| Redundant file reads     | %.1f       | %.1f           |\n",
		comparison.MeanRedundantReadsWith, comparison.MeanRedundantReadsWithout))

	out.WriteString("\n#### Inari Anti-Patterns Detected\n")
	out.WriteString("| Pattern                      | Count |\n")
	out.WriteString("|------------------------------|-------|\n")
	out.WriteString(fmt.Sprintf("| Sketch then read same file   | %d     |\n", comparison.InariAntiPatterns.SketchThenReadCount))
	out.WriteString(fmt.Sprintf("| Grep after inari find        | %d     |\n", comparison.InariAntiPatterns.GrepAfterFindCount))
	out.WriteString(fmt.Sprintf("| callers + refs same symbol   | %d     |\n", comparison.InariAntiPatterns.CallersAndRefsCount))

	out.WriteString(fmt.Sprintf("\n#### Inari Command Usage\n- Mean inari commands per task: %.1f\n",
		comparison.MeanInariCommands))

	return out.String()
}

// GenerateRecommendations produces data-driven CLI recommendations based on behavior analysis.
func GenerateRecommendations(comparison BehaviorComparison, inariCommandSequences [][]string) string {
	var recommendations []string

	// Check if inari refs is never used but inari callers is
	hasCallers := false
	hasRefs := false
	for _, seq := range inariCommandSequences {
		for _, cmd := range seq {
			if strings.HasPrefix(cmd, "inari callers") {
				hasCallers = true
			}
			if strings.HasPrefix(cmd, "inari refs") {
				hasRefs = true
			}
		}
	}

	if hasCallers && !hasRefs {
		recommendations = append(recommendations,
			"**`inari refs` never used** \u2014 agents use `inari callers` exclusively. "+
				"Consider merging refs into callers or improving refs discoverability.")
	}

	if comparison.InariAntiPatterns.SketchThenReadCount > 0 {
		recommendations = append(recommendations, fmt.Sprintf(
			"**Sketch-then-read anti-pattern detected %d time(s)** \u2014 "+
				"agents read files already summarized by sketch. "+
				"Strengthen guidance that sketch output replaces full file reads.",
			comparison.InariAntiPatterns.SketchThenReadCount))
	}

	if comparison.InariAntiPatterns.GrepAfterFindCount > 0 {
		recommendations = append(recommendations, fmt.Sprintf(
			"**Grep after inari find detected %d time(s)** \u2014 "+
				"agents grep for the same thing inari find already answered. "+
				"Improve inari find output to include enough context.",
			comparison.InariAntiPatterns.GrepAfterFindCount))
	}

	if comparison.MeanInariCommands > 5.0 {
		recommendations = append(recommendations, fmt.Sprintf(
			"**High inari command usage (%.1f per task)** \u2014 "+
				"agents may be over-relying on inari. "+
				"Consider enforcing a command limit or improving per-command output density.",
			comparison.MeanInariCommands))
	}

	if comparison.MeanActionsBeforeEditWith > comparison.MeanActionsBeforeEditWithout {
		recommendations = append(recommendations, fmt.Sprintf(
			"**Over-navigation with inari (%.1f vs %.1f actions before first edit)** \u2014 "+
				"inari may be encouraging exploration over action. "+
				"Review whether inari output is actionable enough.",
			comparison.MeanActionsBeforeEditWith, comparison.MeanActionsBeforeEditWithout))
	}

	var out strings.Builder
	out.WriteString("### CLI Recommendations\n\n")

	if len(recommendations) == 0 {
		out.WriteString("No actionable recommendations from current data.\n")
	} else {
		out.WriteString("Based on agent behavior data:\n\n")
		for i, rec := range recommendations {
			out.WriteString(fmt.Sprintf("%d. %s\n", i+1, rec))
		}
	}

	return out.String()
}

// extractInariSubcommand extracts the inari subcommand from an arguments_summary string.
// E.g. "inari sketch PaymentService" -> "inari sketch"
func extractInariSubcommand(argumentsSummary string) string {
	trimmed := strings.TrimSpace(argumentsSummary)
	inariIdx := strings.Index(trimmed, "inari ")
	if inariIdx < 0 {
		return ""
	}
	fromInari := trimmed[inariIdx:]
	parts := strings.SplitN(fromInari, " ", 3)
	if len(parts) >= 2 {
		return parts[0] + " " + parts[1]
	}
	return fromInari
}

// computeInariThenRead detects the anti-pattern where an inari command is followed
// by a Read of a file mentioned in the inari command's arguments.
func computeInariThenRead(actions []AgentAction) uint32 {
	var count uint32
	for i, action := range actions {
		if !action.IsInariCommand {
			continue
		}
		for _, later := range actions[i+1:] {
			if later.ToolName == "Read" && strings.Contains(action.ArgumentsSummary, later.ArgumentsSummary) {
				count++
				break
			}
			if later.IsEdit {
				break
			}
		}
	}
	return count
}

// computeGrepAfterInariFind counts Grep actions that follow an "inari find" action within 3 actions.
func computeGrepAfterInariFind(actions []AgentAction) uint32 {
	var count uint32
	for i, action := range actions {
		if !action.IsInariCommand {
			continue
		}
		if !strings.Contains(action.ArgumentsSummary, "inari find") {
			continue
		}
		end := i + 4
		if end > len(actions) {
			end = len(actions)
		}
		for _, later := range actions[i+1 : end] {
			if later.ToolName == "Grep" {
				count++
				break
			}
		}
	}
	return count
}

// computeCallersAndRefsOverlap checks if both "inari callers X" and "inari refs X"
// appear for the same symbol X.
func computeCallersAndRefsOverlap(actions []AgentAction) bool {
	callersSymbols := make(map[string]struct{})
	refsSymbols := make(map[string]struct{})

	for _, action := range actions {
		if !action.IsInariCommand {
			continue
		}
		summary := strings.TrimSpace(action.ArgumentsSummary)
		if rest, found := strings.CutPrefix(summary, "inari callers "); found {
			callersSymbols[strings.TrimSpace(rest)] = struct{}{}
		} else if rest, found := strings.CutPrefix(summary, "inari refs "); found {
			refsSymbols[strings.TrimSpace(rest)] = struct{}{}
		}
	}

	for sym := range callersSymbols {
		if _, ok := refsSymbols[sym]; ok {
			return true
		}
	}
	return false
}

// meanUint32 computes the mean of a uint32 field across a slice of BehaviorMetrics.
func meanUint32(metrics []BehaviorMetrics, f func(BehaviorMetrics) uint32) float64 {
	if len(metrics) == 0 {
		return 0
	}
	var total uint64
	for _, m := range metrics {
		total += uint64(f(m))
	}
	return float64(total) / float64(len(metrics))
}

// meanFloat64Finite computes the mean of an f64 field, treating Inf values as
// the max finite value seen in the set (to avoid poisoning the mean).
func meanFloat64Finite(metrics []BehaviorMetrics, f func(BehaviorMetrics) float64) float64 {
	if len(metrics) == 0 {
		return 0
	}

	values := make([]float64, len(metrics))
	for i, m := range metrics {
		values[i] = f(m)
	}

	var finiteValues []float64
	for _, v := range values {
		if !math.IsInf(v, 0) && !math.IsNaN(v) {
			finiteValues = append(finiteValues, v)
		}
	}

	if len(finiteValues) == 0 {
		return math.Inf(1)
	}

	maxFinite := math.Inf(-1)
	for _, v := range finiteValues {
		if v > maxFinite {
			maxFinite = v
		}
	}

	var sum float64
	for _, v := range values {
		if math.IsInf(v, 0) || math.IsNaN(v) {
			sum += maxFinite
		} else {
			sum += v
		}
	}

	return sum / float64(len(values))
}
