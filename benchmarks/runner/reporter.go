package runner

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// BenchmarkRun holds a single benchmark run result.
type BenchmarkRun struct {
	TaskID                   string           `json:"task_id"`
	Repetition               uint32           `json:"repetition"`
	InariEnabled             bool             `json:"inari_enabled"`
	Condition                string           `json:"condition"`
	InputTokens              uint64           `json:"input_tokens"`
	OutputTokens             uint64           `json:"output_tokens"`
	CacheCreationInputTokens uint64           `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     uint64           `json:"cache_read_input_tokens"`
	FileReads                uint32           `json:"file_reads"`
	InariCommandsCalled      []string         `json:"inari_commands_called"`
	Correctness              CorrectnessResult `json:"correctness"`
	DurationMs               uint64           `json:"duration_ms"`
	Actions                  []AgentAction    `json:"actions"`
	Behavior                 *BehaviorMetrics `json:"behavior,omitempty"`
}

// CorrectnessResult holds correctness verification results for a single run.
type CorrectnessResult struct {
	CompilationPass bool     `json:"compilation_pass"`
	TestsPass       bool     `json:"tests_pass"`
	CallerCoverage  *float64 `json:"caller_coverage,omitempty"`
	OverallScore    uint32   `json:"overall_score"`
}

// ResultsWrapper is the top-level container for serialized results.
type ResultsWrapper struct {
	InariVersion     string         `json:"inari_version"`
	BenchmarkVersion string         `json:"benchmark_version"`
	RunDate          string         `json:"run_date"`
	Runs             []BenchmarkRun `json:"runs"`
}

// TaskResult summarizes a single task's result for reporting.
type TaskResult struct {
	TaskID          string           `json:"task_id"`
	Condition       string           `json:"condition"`
	Compiles        bool             `json:"compiles"`
	TestsPass       bool             `json:"tests_pass"`
	TokenUsage      uint64           `json:"token_usage"`
	Duration        uint64           `json:"duration"`
	BehaviorMetrics *BehaviorMetrics `json:"behavior_metrics,omitempty"`
}

// ReportSummary holds aggregate statistics for a report.
type ReportSummary struct {
	TotalRuns          int     `json:"total_runs"`
	MeanTokensWith     float64 `json:"mean_tokens_with"`
	MeanTokensWithout  float64 `json:"mean_tokens_without"`
	TokenReduction     float64 `json:"token_reduction"`
	CompilationPassPct float64 `json:"compilation_pass_pct"`
	TestsPassPct       float64 `json:"tests_pass_pct"`
}

// GenerateJSONReport writes benchmark results as JSON.
func GenerateJSONReport(results ResultsWrapper, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	path := filepath.Join(outputDir, "full_results.json")
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize results to JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to create results file %s: %w", path, err)
	}

	return nil
}

// WriteJSONResults writes an array of benchmark runs as a wrapped JSON file.
func WriteJSONResults(runs []BenchmarkRun, path string) error {
	wrapper := ResultsWrapper{
		InariVersion:     detectInariVersion(),
		BenchmarkVersion: "1",
		RunDate:          time.Now().UTC().Format(time.RFC3339),
		Runs:             runs,
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize results to JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to create results file %s: %w", path, err)
	}

	return nil
}

// GenerateMarkdownReport writes a Markdown summary of benchmark results.
func GenerateMarkdownReport(results ResultsWrapper, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	path := filepath.Join(outputDir, "summary.md")
	content := buildMarkdownSummary(results.Runs)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to create summary file %s: %w", path, err)
	}

	return nil
}

// WriteMarkdownSummary writes a Markdown summary to a specific path.
func WriteMarkdownSummary(runs []BenchmarkRun, path string) error {
	content := buildMarkdownSummary(runs)
	return os.WriteFile(path, []byte(content), 0o644)
}

// WriteEnvironment writes environment information to a JSON file.
func WriteEnvironment(path string) error {
	env := map[string]interface{}{
		"inari_version":     detectInariVersion(),
		"benchmark_version": "1",
		"run_date":          time.Now().UTC().Format(time.RFC3339),
		"machine": map[string]string{
			"os":   runtime.GOOS,
			"arch": runtime.GOARCH,
		},
	}

	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize environment info: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// buildMarkdownSummary creates a full Markdown report from benchmark runs.
func buildMarkdownSummary(runs []BenchmarkRun) string {
	var out strings.Builder

	runDate := time.Now().Format("2006-01-02")
	inariVersion := detectInariVersion()

	out.WriteString(fmt.Sprintf("## Inari %s Benchmark Results\n\n", inariVersion))
	out.WriteString(fmt.Sprintf("**Benchmark date:** %s\n", runDate))

	taskIDSet := make(map[string]struct{})
	for _, r := range runs {
		taskIDSet[r.TaskID] = struct{}{}
	}
	out.WriteString(fmt.Sprintf("**Tasks:** %d\n", len(taskIDSet)))

	var maxRep uint32
	for _, r := range runs {
		if r.Repetition > maxRep {
			maxRep = r.Repetition
		}
	}
	if maxRep == 0 {
		maxRep = 1
	}
	out.WriteString(fmt.Sprintf("**Repetitions:** %d per task per condition\n\n", maxRep))

	// Split by inari_enabled
	var withInari, withoutInari []*BenchmarkRun
	for i := range runs {
		if runs[i].InariEnabled {
			withInari = append(withInari, &runs[i])
		} else {
			withoutInari = append(withoutInari, &runs[i])
		}
	}

	hasConditions := false
	for _, r := range runs {
		if r.Condition != "" {
			hasConditions = true
			break
		}
	}

	// Token consumption summary
	out.WriteString("### Token Consumption\n\n")
	out.WriteString("| Condition     | Mean input tokens | Std Dev   | Reduction |\n")
	out.WriteString("|---------------|-------------------|-----------|-----------|\n")

	meanWithout := meanTokens(withoutInari)
	meanWith := meanTokens(withInari)

	if !hasConditions {
		if len(withoutInari) > 0 {
			out.WriteString(fmt.Sprintf("| Without Inari | %.0f               | \u00b1%.0f     | \u2014         |\n",
				meanWithout, stdDevTokens(withoutInari)))
		}
		if len(withInari) > 0 {
			reduction := "N/A"
			if meanWithout > 0 {
				reduction = fmt.Sprintf("**%.1f%%**", (1.0-meanWith/meanWithout)*100)
			}
			out.WriteString(fmt.Sprintf("| With Inari    | %.0f               | \u00b1%.0f     | %s       |\n",
				meanWith, stdDevTokens(withInari), reduction))
		}
	}

	if hasConditions {
		out.WriteString("\n### Token Consumption by Condition\n\n")
		out.WriteString("| Condition             | Mean input tokens | Std Dev   | Runs |\n")
		out.WriteString("|-----------------------|-------------------|-----------|----- |\n")

		conditions := uniqueConditions(runs)
		for _, cond := range conditions {
			condRuns := filterByCondition(runs, cond)
			out.WriteString(fmt.Sprintf("| %-21s | %.0f               | \u00b1%.0f     | %d   |\n",
				cond, meanTokens(condRuns), stdDevTokens(condRuns), len(condRuns)))
		}
	}

	// Correctness summary
	out.WriteString("\n### Task Correctness\n\n")
	out.WriteString("| Condition     | Compilation pass | Tests pass | Mean score |\n")
	out.WriteString("|---------------|-----------------|------------|------------|\n")

	if len(withoutInari) > 0 {
		compPct, testPct, meanScore := correctnessStats(withoutInari)
		out.WriteString(fmt.Sprintf("| Without Inari | %.0f%%             | %.0f%%       | %.0f        |\n",
			compPct, testPct, meanScore))
	}
	if len(withInari) > 0 {
		compPct, testPct, meanScore := correctnessStats(withInari)
		out.WriteString(fmt.Sprintf("| With Inari    | %.0f%%             | %.0f%%       | %.0f        |\n",
			compPct, testPct, meanScore))
	}

	// File reads summary
	out.WriteString("\n### File Reads per Task\n\n")
	out.WriteString("| Condition     | Mean file reads |\n")
	out.WriteString("|---------------|----------------|\n")

	if len(withoutInari) > 0 {
		out.WriteString(fmt.Sprintf("| Without Inari | %.1f            |\n", meanFileReads(withoutInari)))
	}
	if len(withInari) > 0 {
		out.WriteString(fmt.Sprintf("| With Inari    | %.1f            |\n", meanFileReads(withInari)))
	}

	// Token decomposition (cache vs fresh)
	hasCacheData := false
	for _, r := range runs {
		if r.CacheReadInputTokens > 0 || r.CacheCreationInputTokens > 0 {
			hasCacheData = true
			break
		}
	}

	if hasCacheData {
		out.WriteString("\n### Token Decomposition\n\n")
		out.WriteString("| Condition     | Total input | Fresh input | Cache read | Cache hit rate |\n")
		out.WriteString("|---------------|------------|-------------|------------|----------------|\n")

		formatDecomp := func(runPtrs []*BenchmarkRun, label string) string {
			if len(runPtrs) == 0 {
				return ""
			}
			var totalInput, cacheRead uint64
			for _, r := range runPtrs {
				totalInput += r.InputTokens
				cacheRead += r.CacheReadInputTokens
			}
			n := float64(len(runPtrs))
			meanTotal := float64(totalInput) / n
			meanCache := float64(cacheRead) / n
			meanFresh := meanTotal - meanCache
			hitRate := 0.0
			if totalInput > 0 {
				hitRate = float64(cacheRead) / float64(totalInput) * 100
			}
			return fmt.Sprintf("| %-13s | %.0f      | %.0f       | %.0f      | %.1f%%          |\n",
				label, meanTotal, meanFresh, meanCache, hitRate)
		}

		if !hasConditions {
			if len(withoutInari) > 0 {
				out.WriteString(formatDecomp(withoutInari, "Without Inari"))
			}
			if len(withInari) > 0 {
				out.WriteString(formatDecomp(withInari, "With Inari"))
			}
		}

		if hasConditions {
			conditions := uniqueConditions(runs)
			for _, cond := range conditions {
				condRuns := filterByCondition(runs, cond)
				out.WriteString(formatDecomp(condRuns, cond))
			}
		}
	}

	// Per-category breakdown
	out.WriteString("\n### By Category\n\n")
	out.WriteString("| Category  | With Inari (tokens) | Without Inari (tokens) | Reduction |\n")
	out.WriteString("|-----------|--------------------|-----------------------|-----------|\n")

	categories := extractCategories(runs)
	for _, cat := range categories {
		catWith := filterByCategoryAndInari(withInari, cat)
		catWithout := filterByCategoryAndInari(withoutInari, cat)

		mWith := meanTokens(catWith)
		mWithout := meanTokens(catWithout)
		reduction := "N/A"
		if mWithout > 0 {
			reduction = fmt.Sprintf("%.1f%%", (1.0-mWith/mWithout)*100)
		}

		out.WriteString(fmt.Sprintf("| %-9s | %.0f                | %.0f                   | %s       |\n",
			cat, mWith, mWithout, reduction))
	}

	out.WriteString(fmt.Sprintf("\n*All results are means across %d repetitions per task.*\n", maxRep))

	// Behavior analysis section
	var withBehaviors, withoutBehaviors []BehaviorMetrics
	for _, r := range withInari {
		if r.Behavior != nil {
			withBehaviors = append(withBehaviors, *r.Behavior)
		}
	}
	for _, r := range withoutInari {
		if r.Behavior != nil {
			withoutBehaviors = append(withoutBehaviors, *r.Behavior)
		}
	}

	if len(withBehaviors) > 0 || len(withoutBehaviors) > 0 {
		comparison := AggregateBehavior(withBehaviors, withoutBehaviors)
		out.WriteString("\n")
		out.WriteString(FormatBehaviorMarkdown(comparison))

		var inariSequences [][]string
		for _, b := range withBehaviors {
			inariSequences = append(inariSequences, b.InariCommandSequence)
		}
		out.WriteString("\n")
		out.WriteString(GenerateRecommendations(comparison, inariSequences))
	}

	return out.String()
}

// detectInariVersion detects the Inari version by running `inari --version`.
func detectInariVersion() string {
	cmd := exec.Command("inari", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// meanTokens calculates the mean input tokens from a set of runs.
func meanTokens(runs []*BenchmarkRun) float64 {
	if len(runs) == 0 {
		return 0
	}
	var total uint64
	for _, r := range runs {
		total += r.InputTokens
	}
	return float64(total) / float64(len(runs))
}

// stdDevTokens calculates the standard deviation of input tokens.
func stdDevTokens(runs []*BenchmarkRun) float64 {
	if len(runs) < 2 {
		return 0
	}
	mean := meanTokens(runs)
	var variance float64
	for _, r := range runs {
		diff := float64(r.InputTokens) - mean
		variance += diff * diff
	}
	variance /= float64(len(runs) - 1)
	return math.Sqrt(variance)
}

// meanFileReads calculates mean file reads from a set of runs.
func meanFileReads(runs []*BenchmarkRun) float64 {
	if len(runs) == 0 {
		return 0
	}
	var total uint32
	for _, r := range runs {
		total += r.FileReads
	}
	return float64(total) / float64(len(runs))
}

// correctnessStats returns (compilation_pass_%, tests_pass_%, mean_score).
func correctnessStats(runs []*BenchmarkRun) (float64, float64, float64) {
	if len(runs) == 0 {
		return 0, 0, 0
	}
	n := float64(len(runs))
	var comp, test float64
	var score uint32
	for _, r := range runs {
		if r.Correctness.CompilationPass {
			comp++
		}
		if r.Correctness.TestsPass {
			test++
		}
		score += r.Correctness.OverallScore
	}
	return comp / n * 100, test / n * 100, float64(score) / n
}

// uniqueConditions returns sorted unique condition labels from results.
func uniqueConditions(runs []BenchmarkRun) []string {
	condSet := make(map[string]struct{})
	for _, r := range runs {
		if r.Condition != "" {
			condSet[r.Condition] = struct{}{}
		}
	}
	var conditions []string
	for c := range condSet {
		conditions = append(conditions, c)
	}
	sort.Strings(conditions)
	return conditions
}

// filterByCondition returns pointers to runs matching a condition.
func filterByCondition(runs []BenchmarkRun, condition string) []*BenchmarkRun {
	var result []*BenchmarkRun
	for i := range runs {
		if runs[i].Condition == condition {
			result = append(result, &runs[i])
		}
	}
	return result
}

// extractCategories extracts unique categories from task IDs.
// Derives category from task ID pattern: "ts-cat-X-NN" => "cat-X"
func extractCategories(runs []BenchmarkRun) []string {
	catSet := make(map[string]struct{})
	for _, r := range runs {
		catSet[taskCategory(r.TaskID)] = struct{}{}
	}
	var categories []string
	for c := range catSet {
		categories = append(categories, c)
	}
	sort.Strings(categories)
	return categories
}

// taskCategory extracts category from a task ID (e.g. "ts-cat-a-01" => "cat-a").
func taskCategory(taskID string) string {
	parts := strings.Split(taskID, "-")
	if len(parts) >= 3 {
		return parts[1] + "-" + parts[2]
	}
	return "unknown"
}

// filterByCategoryAndInari filters runs by category.
func filterByCategoryAndInari(runs []*BenchmarkRun, category string) []*BenchmarkRun {
	var result []*BenchmarkRun
	for _, r := range runs {
		if taskCategory(r.TaskID) == category {
			result = append(result, r)
		}
	}
	return result
}
