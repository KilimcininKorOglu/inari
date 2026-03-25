// Benchmark runner for Inari — measures token consumption reduction
// across 3-arm experiment design (without-inari, with-inari, with-inari-preloaded).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	runner "github.com/KilimcininKorOglu/inari/benchmarks/runner"
	"github.com/spf13/cobra"
)

var version = "1.3.0"
var verbose bool

func main() {
	rootCmd := &cobra.Command{
		Use:     "inari-benchmark",
		Short:   "Benchmark harness for Inari CLI",
		Version: version,
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	rootCmd.AddCommand(
		newRunCmd(),
		newPrepareCmd(),
		newImportCmd(),
		newReportCmd(),
		newManifestCmd(),
		newVerifyCmd(),
		newTestCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// run
// ---------------------------------------------------------------------------

func newRunCmd() *cobra.Command {
	var (
		all        bool
		task       string
		category   string
		language   string
		reps       uint32
		compare    bool
		noInari    bool
		inariOnly  bool
		model      string
		conditions uint32
		saveNdjson string
		outputDir  string
		parallel   int
		resume     bool
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run benchmark tasks against codebases with and without Inari",
		Long: `Run benchmark tasks against codebases with and without Inari.

By default, runs all tasks with Inari enabled. Use --compare to run both
with and without Inari for a side-by-side comparison. Use --conditions 3
for the full 3-arm experiment (without-inari, with-inari, with-inari-preloaded).

Requires ANTHROPIC_API_KEY and the 'claude' CLI installed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchmarks(all, task, category, language, reps, compare, noInari, inariOnly, model, conditions, saveNdjson, outputDir, parallel, resume)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Run all tasks in the task suite")
	cmd.Flags().StringVar(&task, "task", "", "Run a single task by ID (e.g. ts-cat-a-01)")
	cmd.Flags().StringVar(&category, "category", "", "Run all tasks in a specific category")
	cmd.Flags().StringVar(&language, "language", "", "Run all tasks for a specific language")
	cmd.Flags().Uint32Var(&reps, "reps", 3, "Number of repetitions per task per condition")
	cmd.Flags().BoolVar(&compare, "compare", false, "Run both with-Inari and without-Inari conditions")
	cmd.Flags().BoolVar(&noInari, "no-inari", false, "Run only the baseline condition (no Inari)")
	cmd.Flags().BoolVar(&inariOnly, "inari-only", false, "Run only the Inari-enabled condition")
	cmd.Flags().StringVar(&model, "model", "", "Model to use for agent runs (e.g. sonnet, opus, haiku)")
	cmd.Flags().Uint32Var(&conditions, "conditions", 1, "Number of experimental conditions (1, 2, or 3)")
	cmd.Flags().StringVar(&saveNdjson, "save-ndjson", "", "Directory to save raw NDJSON streams")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory for results")
	cmd.Flags().IntVar(&parallel, "parallel", 1, "Number of parallel agent runs")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume a previous run by skipping completed task/condition/rep combinations")
	cmd.MarkFlagsMutuallyExclusive("no-inari", "inari-only")

	return cmd
}

// ---------------------------------------------------------------------------
// prepare
// ---------------------------------------------------------------------------

func newPrepareCmd() *cobra.Command {
	var (
		all        bool
		task       string
		language   string
		compare    bool
		conditions uint32
		outputDir  string
	)

	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepare isolated work directories and print prompts for manual runs",
		Long: `Sets up temp directories with CLAUDE.md variants and .inari/ indexes,
then prints the exact prompts to use. Does NOT require an API key.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return prepareBenchmarks(all, task, language, compare, conditions, outputDir)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Prepare all tasks")
	cmd.Flags().StringVar(&task, "task", "", "Prepare a single task by ID")
	cmd.Flags().StringVar(&language, "language", "", "Prepare all tasks for a specific language")
	cmd.Flags().BoolVar(&compare, "compare", false, "Also prepare without-inari variant")
	cmd.Flags().Uint32Var(&conditions, "conditions", 1, "Number of experimental conditions (1, 2, or 3)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory for prepared work dirs")

	return cmd
}

// ---------------------------------------------------------------------------
// import
// ---------------------------------------------------------------------------

func newImportCmd() *cobra.Command {
	var (
		input     string
		ndjsonDir string
		output    string
		outputDir string
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import manually captured benchmark results for analysis",
		RunE: func(cmd *cobra.Command, args []string) error {
			return importResults(input, ndjsonDir, output, outputDir)
		},
	}

	cmd.Flags().StringVar(&input, "input", "", "Path to a JSON file with manually captured results")
	cmd.Flags().StringVar(&ndjsonDir, "ndjson-dir", "", "Directory containing raw NDJSON files")
	cmd.Flags().StringVar(&output, "output", "markdown", "Output format (json or markdown)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory for results")
	_ = cmd.MarkFlagRequired("input")

	return cmd
}

// ---------------------------------------------------------------------------
// report
// ---------------------------------------------------------------------------

func newReportCmd() *cobra.Command {
	var (
		input  string
		output string
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a report from existing benchmark results",
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateReport(input, output)
		},
	}

	cmd.Flags().StringVar(&input, "input", "", "Path to a JSON results file or directory")
	cmd.Flags().StringVar(&output, "output", "markdown", "Output format (json or markdown)")
	_ = cmd.MarkFlagRequired("input")

	return cmd
}

// ---------------------------------------------------------------------------
// manifest
// ---------------------------------------------------------------------------

func newManifestCmd() *cobra.Command {
	var (
		generate bool
		verify   bool
		fixture  string
	)

	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Generate or verify fixture integrity manifests",
		RunE: func(cmd *cobra.Command, args []string) error {
			return manageManifests(generate, verify, fixture)
		},
	}

	cmd.Flags().BoolVar(&generate, "generate", false, "Generate a new manifest from the current fixture state")
	cmd.Flags().BoolVar(&verify, "verify", false, "Verify fixtures against their stored manifests")
	cmd.Flags().StringVar(&fixture, "fixture", "", "Path to a specific fixture directory")
	cmd.MarkFlagsMutuallyExclusive("generate", "verify")

	return cmd
}

// ---------------------------------------------------------------------------
// verify
// ---------------------------------------------------------------------------

func newVerifyCmd() *cobra.Command {
	var (
		dir    string
		task   string
		asJSON bool
	)

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify correctness of a completed benchmark work directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return verifyWorkDir(dir, task, asJSON)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Path to a completed work directory to verify")
	cmd.Flags().StringVar(&task, "task", "", "Task ID to verify against")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON instead of human-readable")
	_ = cmd.MarkFlagRequired("dir")

	return cmd
}

// ---------------------------------------------------------------------------
// test
// ---------------------------------------------------------------------------

func newTestCmd() *cobra.Command {
	var (
		task     string
		language string
		model    string
	)

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run a single-task validation test across all 3 conditions",
		Long: `Runs one task with 1 rep across without-inari, with-inari, and
with-inari-preloaded conditions. Validates that telemetry data is captured
correctly before committing to a full benchmark run.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTest(task, language, model)
		},
	}

	cmd.Flags().StringVar(&task, "task", "", "Task ID to test (e.g. ts-cat-a-01)")
	cmd.Flags().StringVar(&language, "language", "", "Language to pick a task from if --task not specified")
	cmd.Flags().StringVar(&model, "model", "", "Model to use (e.g. sonnet, opus, haiku)")
	_ = cmd.MarkFlagRequired("model")

	return cmd
}

// ===========================================================================
// Command implementations
// ===========================================================================

// runJob is a fully-specified benchmark job ready for execution.
type runJob struct {
	taskDefIdx     int
	rep            uint32
	conditionLabel string
	inariEnabled   bool
	corpusPath     string
	backupPath     string
	ndjsonPath     string
}

func runBenchmarks(all bool, taskFilter, categoryFilter, languageFilter string, reps uint32, compare, noInari, inariOnly bool, model string, conditionsFlag uint32, saveNdjson, outputDir string, parallel int, resume bool) error {
	tasksDir, err := findTasksDir()
	if err != nil {
		return err
	}
	allTasks, err := runner.LoadTasks(tasksDir)
	if err != nil {
		return err
	}

	if len(allTasks) == 0 {
		return fmt.Errorf("no task files found in %s. Expected TOML files in benchmarks/tasks/", tasksDir)
	}

	tasks := filterTasks(allTasks, all, taskFilter, categoryFilter, languageFilter)
	if len(tasks) == 0 {
		return fmt.Errorf("no tasks matched the given filters")
	}

	fmt.Fprintf(os.Stderr, "Selected %d task(s), %d rep(s) each\n", len(tasks), reps)

	// Determine conditions
	type condDef struct {
		label        string
		inariEnabled bool
	}

	var conditions []condDef
	if conditionsFlag >= 3 {
		conditions = []condDef{
			{"without-inari", false},
			{"with-inari", true},
			{"with-inari-preloaded", true},
		}
	} else if compare || conditionsFlag == 2 {
		conditions = []condDef{
			{"with-inari", true},
			{"without-inari", false},
		}
	} else if noInari {
		conditions = []condDef{{"without-inari", false}}
	} else {
		conditions = []condDef{{"with-inari", true}}
	}

	// Determine NDJSON directory
	var ndjsonDir string
	if saveNdjson != "" {
		ndjsonDir = saveNdjson
	} else if outputDir != "" {
		ndjsonDir = filepath.Join(outputDir, "ndjson")
	}
	if ndjsonDir != "" {
		os.MkdirAll(ndjsonDir, 0o755)
	}

	// Set up results directory
	resultsDir := outputDir
	if resultsDir == "" {
		resultsDir = findResultsDir()
	}
	os.MkdirAll(resultsDir, 0o755)
	jsonPath := filepath.Join(resultsDir, "full_results.json")

	// Build all jobs
	var jobs []runJob
	corpusBackups := make(map[string]string) // corpusPath -> backupPath

	for taskIdx, taskDef := range tasks {
		corpusPath, err := resolveCorpusPath(taskDef)
		if err != nil {
			return err
		}

		backup, ok := corpusBackups[corpusPath]
		if !ok {
			inariDir := filepath.Join(corpusPath, ".inari")
			if runner.DirExistsPublic(inariDir) {
				b, err := runner.BackupInariIndex(corpusPath)
				if err != nil {
					return err
				}
				backup = b
			}
			corpusBackups[corpusPath] = backup
		}

		for rep := uint32(1); rep <= reps; rep++ {
			for _, cond := range conditions {
				ndjsonPath := ""
				if ndjsonDir != "" {
					ndjsonPath = filepath.Join(ndjsonDir, fmt.Sprintf("%s-%s-rep%d.ndjson", taskDef.Task.ID, cond.label, rep))
				}

				jobs = append(jobs, runJob{
					taskDefIdx:     taskIdx,
					rep:            rep,
					conditionLabel: cond.label,
					inariEnabled:   cond.inariEnabled,
					corpusPath:     corpusPath,
					backupPath:     backup,
					ndjsonPath:     ndjsonPath,
				})
			}
		}
	}

	// Resume: load existing results and skip completed jobs
	var resumedRuns []runner.BenchmarkRun
	completedKeys := make(map[string]struct{})

	if resume {
		if data, err := os.ReadFile(jsonPath); err == nil {
			var wrapper runner.ResultsWrapper
			if err := json.Unmarshal(data, &wrapper); err == nil {
				for _, r := range wrapper.Runs {
					key := fmt.Sprintf("%s|%s|%d", r.TaskID, r.Condition, r.Repetition)
					completedKeys[key] = struct{}{}
				}
				fmt.Fprintf(os.Stderr, "Resuming: found %d completed runs in %s\n", len(wrapper.Runs), jsonPath)
				resumedRuns = wrapper.Runs
			}
		}
	}

	// Filter out completed jobs
	originalCount := len(jobs)
	if len(completedKeys) > 0 {
		var remaining []runJob
		for _, job := range jobs {
			key := fmt.Sprintf("%s|%s|%d", tasks[job.taskDefIdx].Task.ID, job.conditionLabel, job.rep)
			if _, done := completedKeys[key]; !done {
				remaining = append(remaining, job)
			}
		}
		jobs = remaining
		fmt.Fprintf(os.Stderr, "Skipping %d completed, %d remaining\n\n", originalCount-len(jobs), len(jobs))
	}

	totalRuns := len(jobs)
	if totalRuns == 0 {
		fmt.Fprintln(os.Stderr, "All runs already completed. Nothing to do.")
		mdPath := filepath.Join(resultsDir, "summary.md")
		runner.WriteMarkdownSummary(resumedRuns, mdPath)
		envPath := filepath.Join(resultsDir, "environment.json")
		runner.WriteEnvironment(envPath)
		return nil
	}

	modelLabel := model
	if modelLabel == "" {
		modelLabel = "default"
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  inari benchmark")
	fmt.Fprintln(os.Stderr, "  ---------------------------------------------------------------")
	fmt.Fprintf(os.Stderr, "  model:      %s\n", modelLabel)
	fmt.Fprintf(os.Stderr, "  runs:       %d total (%d tasks x %d conditions x %d reps)\n",
		totalRuns, len(tasks), len(conditions), reps)
	fmt.Fprintf(os.Stderr, "  parallel:   %d\n", parallel)
	fmt.Fprintf(os.Stderr, "  output:     %s\n", resultsDir)
	fmt.Fprintln(os.Stderr, "  ---------------------------------------------------------------")
	fmt.Fprintln(os.Stderr)

	benchmarkStart := time.Now()

	var allRuns []runner.BenchmarkRun

	if parallel <= 1 {
		// Sequential execution with incremental save
		allRuns = resumedRuns

		for i, job := range jobs {
			taskDef := tasks[job.taskDefIdx]

			elapsed := time.Since(benchmarkStart).Seconds()
			eta := "calculating"
			if i > 0 {
				avgPerRun := elapsed / float64(i)
				remaining := avgPerRun * float64(totalRuns-i)
				eta = formatDuration(uint64(remaining))
			}

			fmt.Fprintf(os.Stderr, "  [%d/%d] %s %s | %s | rep %d\n",
				i+1, totalRuns, taskDef.Task.ID, job.conditionLabel, taskDef.Task.Category, job.rep)
			fmt.Fprintf(os.Stderr, "         %s  eta %s\n", formatDuration(uint64(elapsed)), eta)

			agentRun, workDir, err := runner.RunAgent(taskDef, job.inariEnabled, job.conditionLabel, model, job.corpusPath, job.backupPath, job.ndjsonPath)
			if err != nil {
				if workDir != "" {
					os.RemoveAll(workDir)
				}
				fmt.Fprintf(os.Stderr, "  ERROR: %v\n\n", err)
				continue
			}

			verification, _ := runner.Verify(taskDef, workDir)
			bm := runner.ComputeBehaviorMetrics(agentRun.Actions)

			benchRun := runner.BenchmarkRun{
				TaskID:                   taskDef.Task.ID,
				Repetition:               job.rep,
				InariEnabled:             job.inariEnabled,
				Condition:                job.conditionLabel,
				InputTokens:              agentRun.InputTokens,
				OutputTokens:             agentRun.OutputTokens,
				CacheCreationInputTokens: agentRun.CacheCreationInputTokens,
				CacheReadInputTokens:     agentRun.CacheReadInputTokens,
				FileReads:                agentRun.FileReads,
				InariCommandsCalled:      agentRun.InariCommandsCalled,
				Correctness: runner.CorrectnessResult{
					CompilationPass: verification.CompilationPass,
					TestsPass:       verification.TestsPass,
					CallerCoverage:  verification.CallerCoverage,
					OverallScore:    verification.OverallScore,
				},
				DurationMs: agentRun.DurationMs,
				Actions:    agentRun.Actions,
				Behavior:   &bm,
			}
			allRuns = append(allRuns, benchRun)

			// Incremental save
			runner.WriteJSONResults(allRuns, jsonPath)

			compStr := "FAIL"
			if verification.CompilationPass {
				compStr = "pass"
			}
			fmt.Fprintf(os.Stderr, "         done in %ds | %d tokens out | %d reads | %d actions | %s\n\n",
				agentRun.DurationMs/1000, agentRun.OutputTokens, agentRun.FileReads, len(agentRun.Actions), compStr)

			// Clean up temp directory
			os.RemoveAll(workDir)
		}
	} else {
		// Parallel execution
		var mu sync.Mutex
		allRuns = resumedRuns
		var completed int64
		var failed int64

		for start := 0; start < len(jobs); start += parallel {
			end := start + parallel
			if end > len(jobs) {
				end = len(jobs)
			}
			chunk := jobs[start:end]

			var wg sync.WaitGroup
			for _, job := range chunk {
				wg.Add(1)
				go func(j runJob) {
					defer wg.Done()
					taskDef := tasks[j.taskDefIdx]

					fmt.Fprintf(os.Stderr, "  [parallel] Starting %s (%s, rep %d)\n",
						taskDef.Task.ID, j.conditionLabel, j.rep)

					agentRun, workDir, err := runner.RunAgent(taskDef, j.inariEnabled, j.conditionLabel, model, j.corpusPath, j.backupPath, j.ndjsonPath)
					if err != nil {
						if workDir != "" {
							os.RemoveAll(workDir)
						}
						atomic.AddInt64(&failed, 1)
						fmt.Fprintf(os.Stderr, "  [parallel] Run failed: %v\n", err)
						return
					}

					verification, _ := runner.Verify(taskDef, workDir)
					bm := runner.ComputeBehaviorMetrics(agentRun.Actions)

					benchRun := runner.BenchmarkRun{
						TaskID:                   taskDef.Task.ID,
						Repetition:               j.rep,
						InariEnabled:             j.inariEnabled,
						Condition:                j.conditionLabel,
						InputTokens:              agentRun.InputTokens,
						OutputTokens:             agentRun.OutputTokens,
						CacheCreationInputTokens: agentRun.CacheCreationInputTokens,
						CacheReadInputTokens:     agentRun.CacheReadInputTokens,
						FileReads:                agentRun.FileReads,
						InariCommandsCalled:      agentRun.InariCommandsCalled,
						Correctness: runner.CorrectnessResult{
							CompilationPass: verification.CompilationPass,
							TestsPass:       verification.TestsPass,
							CallerCoverage:  verification.CallerCoverage,
							OverallScore:    verification.OverallScore,
						},
						DurationMs: agentRun.DurationMs,
						Actions:    agentRun.Actions,
						Behavior:   &bm,
					}

					mu.Lock()
					allRuns = append(allRuns, benchRun)
					runner.WriteJSONResults(allRuns, jsonPath)
					mu.Unlock()

					c := atomic.AddInt64(&completed, 1)
					fmt.Fprintf(os.Stderr, "  [parallel] Completed %d/%d\n", c, totalRuns)

					os.RemoveAll(workDir)
				}(job)
			}
			wg.Wait()
		}

		failCount := atomic.LoadInt64(&failed)
		if failCount > 0 {
			fmt.Fprintf(os.Stderr, "  Warning: %d of %d runs failed\n", failCount, totalRuns)
		}
	}

	// Final summary
	totalElapsed := uint64(time.Since(benchmarkStart).Seconds())
	var totalOutput uint64
	for _, r := range allRuns {
		totalOutput += r.OutputTokens
	}
	compPass := 0
	for _, r := range allRuns {
		if r.Correctness.CompilationPass {
			compPass++
		}
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  benchmark complete")
	fmt.Fprintln(os.Stderr, "  ---------------------------------------------------------------")
	fmt.Fprintf(os.Stderr, "  runs:         %d/%d\n", len(allRuns), totalRuns)
	fmt.Fprintf(os.Stderr, "  duration:     %s\n", formatDuration(totalElapsed))
	fmt.Fprintf(os.Stderr, "  compilation:  %d/%d pass (%.0f%%)\n", compPass, len(allRuns), float64(compPass)/float64(len(allRuns))*100)
	fmt.Fprintf(os.Stderr, "  output tkns:  %d\n", totalOutput)
	fmt.Fprintln(os.Stderr, "  ---------------------------------------------------------------")

	// Final saves
	runner.WriteJSONResults(allRuns, jsonPath)
	fmt.Fprintf(os.Stderr, "\n  %s\n", jsonPath)

	mdPath := filepath.Join(resultsDir, "summary.md")
	runner.WriteMarkdownSummary(allRuns, mdPath)
	fmt.Fprintf(os.Stderr, "  %s\n", mdPath)

	envPath := filepath.Join(resultsDir, "environment.json")
	runner.WriteEnvironment(envPath)
	fmt.Fprintf(os.Stderr, "  %s\n", envPath)

	return nil
}

func prepareBenchmarks(all bool, taskFilter, languageFilter string, compare bool, conditionsFlag uint32, outputDir string) error {
	tasksDir, err := findTasksDir()
	if err != nil {
		return err
	}
	allTasks, err := runner.LoadTasks(tasksDir)
	if err != nil {
		return err
	}

	tasks := filterTasks(allTasks, all, taskFilter, "", languageFilter)
	if len(tasks) == 0 {
		return fmt.Errorf("no tasks matched the given filters. Specify --all, --task <id>, or --language <lang>")
	}

	outputBase := outputDir
	if outputBase == "" {
		outputBase = "benchmarks/prepared"
	}
	os.MkdirAll(outputBase, 0o755)

	ndjsonDir := filepath.Join(outputBase, "ndjson")
	os.MkdirAll(ndjsonDir, 0o755)

	type condDef struct {
		label        string
		inariEnabled bool
	}

	var conditions []condDef
	if conditionsFlag >= 3 {
		conditions = []condDef{
			{"without-inari", false},
			{"with-inari", true},
			{"with-inari-preloaded", true},
		}
	} else if compare || conditionsFlag == 2 {
		conditions = []condDef{
			{"with-inari", true},
			{"without-inari", false},
		}
	} else {
		conditions = []condDef{{"with-inari", true}}
	}

	var manifest []map[string]interface{}
	verifiedFixtures := make(map[string]struct{})

	for _, taskDef := range tasks {
		corpusPath, err := resolveCorpusPath(taskDef)
		if err != nil {
			return err
		}

		// Verify fixture integrity (once per fixture)
		if _, ok := verifiedFixtures[corpusPath]; !ok {
			manifestFile := filepath.Join(corpusPath, ".fixture-manifest.sha256")
			if runner.FileExistsPublic(manifestFile) {
				ok, failures := runner.VerifyManifest(corpusPath, nil)
				if !ok {
					return fmt.Errorf("fixture integrity check failed for %s:\n%s", corpusPath, strings.Join(failures, "\n"))
				}
			} else {
				fmt.Fprintf(os.Stderr, "  Warning: No manifest found for %s. Skipping integrity check.\n", corpusPath)
			}
			verifiedFixtures[corpusPath] = struct{}{}
		}

		for _, cond := range conditions {
			dirName := fmt.Sprintf("%s-%s", taskDef.Task.ID, cond.label)
			workDir := filepath.Join(outputBase, dirName)

			if runner.DirExistsPublic(workDir) {
				os.RemoveAll(workDir)
			}
			os.MkdirAll(workDir, 0o755)

			runner.CopyDirForPrepare(corpusPath, workDir)

			// Install CLAUDE.md variant
			variant := "CLAUDE.md.without-inari"
			if cond.inariEnabled {
				variant = "CLAUDE.md.with-inari"
			}
			variantSrc := filepath.Join(workDir, variant)
			if runner.FileExistsPublic(variantSrc) {
				data, _ := os.ReadFile(variantSrc)
				os.WriteFile(filepath.Join(workDir, "CLAUDE.md"), data, 0o644)
			}

			// Handle .inari/ directory
			if !cond.inariEnabled {
				inariDir := filepath.Join(workDir, ".inari")
				if runner.DirExistsPublic(inariDir) {
					os.RemoveAll(inariDir)
				}
			}

			// Handle preloaded inari map variant
			if cond.label == "with-inari-preloaded" {
				preloadedVariant := filepath.Join(workDir, "CLAUDE.md.with-inari-preloaded")
				if runner.FileExistsPublic(preloadedVariant) {
					mapOutput, err := runInariMap(workDir)
					if err == nil {
						template, _ := os.ReadFile(preloadedVariant)
						rendered := strings.ReplaceAll(string(template), "{{INARI_MAP_OUTPUT}}", mapOutput)
						os.WriteFile(filepath.Join(workDir, "CLAUDE.md"), []byte(rendered), 0o644)
					} else {
						fmt.Fprintf(os.Stderr, "  Warning: inari map failed for %s. Falling back to with-inari variant.\n", dirName)
						fallback := filepath.Join(workDir, "CLAUDE.md.with-inari")
						if runner.FileExistsPublic(fallback) {
							data, _ := os.ReadFile(fallback)
							os.WriteFile(filepath.Join(workDir, "CLAUDE.md"), data, 0o644)
						}
					}
				} else {
					fallback := filepath.Join(workDir, "CLAUDE.md.with-inari")
					if runner.FileExistsPublic(fallback) {
						data, _ := os.ReadFile(fallback)
						os.WriteFile(filepath.Join(workDir, "CLAUDE.md"), data, 0o644)
					}
				}
			}

			entry := map[string]interface{}{
				"task_id":       taskDef.Task.ID,
				"category":      taskDef.Task.Category,
				"language":      taskDef.Task.Language,
				"inari_enabled": cond.inariEnabled,
				"condition":     cond.label,
				"work_dir":      workDir,
				"prompt":        taskDef.Prompt.Text,
			}
			manifest = append(manifest, entry)

			fmt.Fprintf(os.Stderr, "  Prepared: %s\n", dirName)
		}
	}

	// Write manifest
	manifestPath := filepath.Join(outputBase, "manifest.json")
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(manifestPath, manifestJSON, 0o644)

	fmt.Fprintf(os.Stderr, "\nPrepared %d work directories. Manifest: %s\n", len(manifest), manifestPath)
	fmt.Fprintf(os.Stderr, "\nTo run manually, for each entry in manifest.json:\n")
	fmt.Fprintf(os.Stderr, "  1. cd into the work_dir\n")
	fmt.Fprintf(os.Stderr, "  2. Run the prompt as a Claude Code agent\n")
	fmt.Fprintf(os.Stderr, "  3. Save the raw NDJSON output\n")
	fmt.Fprintf(os.Stderr, "  4. Use 'benchmark import --input <results.json> --ndjson-dir %s' to ingest\n", ndjsonDir)

	return nil
}

func importResults(inputPath, ndjsonDir, outputFormat, outputDir string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read import file %s: %w", inputPath, err)
	}

	// Try parsing as ResultsWrapper first, then as array
	var runs []runner.BenchmarkRun
	var wrapper runner.ResultsWrapper
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Runs) > 0 {
		runs = wrapper.Runs
	} else if err := json.Unmarshal(data, &runs); err != nil {
		return fmt.Errorf("failed to parse import file. Expected JSON array of runs or ResultsWrapper")
	}

	if len(runs) == 0 {
		return fmt.Errorf("import file contains no runs")
	}

	// Recompute behavior metrics if missing
	for i := range runs {
		if runs[i].Behavior == nil && len(runs[i].Actions) > 0 {
			bm := runner.ComputeBehaviorMetrics(runs[i].Actions)
			runs[i].Behavior = &bm
		}
	}

	// Populate actions from NDJSON files if provided
	if ndjsonDir != "" {
		if !runner.DirExistsPublic(ndjsonDir) {
			return fmt.Errorf("NDJSON directory does not exist: %s", ndjsonDir)
		}

		var parsedCount uint32
		for i := range runs {
			if len(runs[i].Actions) > 0 {
				continue
			}

			condLabel := runs[i].Condition
			if condLabel == "" {
				if runs[i].InariEnabled {
					condLabel = "with-inari"
				} else {
					condLabel = "without-inari"
				}
			}

			ndjsonFile := filepath.Join(ndjsonDir, fmt.Sprintf("%s-%s.ndjson", runs[i].TaskID, condLabel))
			if runner.FileExistsPublic(ndjsonFile) {
				f, err := os.Open(ndjsonFile)
				if err != nil {
					continue
				}
				actions, _ := runner.ParseNDJSON(f)
				f.Close()

				runs[i].Actions = actions
				if len(runs[i].Actions) > 0 {
					bm := runner.ComputeBehaviorMetrics(runs[i].Actions)
					runs[i].Behavior = &bm
				}
				parsedCount++
				fmt.Fprintf(os.Stderr, "  Parsed %d actions from NDJSON for %s (%s)\n",
					len(runs[i].Actions), runs[i].TaskID, condLabel)
			}
		}

		if parsedCount > 0 {
			fmt.Fprintf(os.Stderr, "Enriched %d runs with NDJSON action data.\n", parsedCount)
		} else {
			fmt.Fprintln(os.Stderr, "Warning: --ndjson-dir provided but no matching NDJSON files found.")
		}
	}

	fmt.Fprintf(os.Stderr, "Imported %d runs.\n", len(runs))

	outDir := outputDir
	if outDir == "" {
		outDir = "benchmarks/results/latest"
	}
	os.MkdirAll(outDir, 0o755)

	switch outputFormat {
	case "json":
		path := filepath.Join(outDir, "full_results.json")
		runner.WriteJSONResults(runs, path)
		fmt.Fprintf(os.Stderr, "Results written to %s\n", path)
	default:
		path := filepath.Join(outDir, "summary.md")
		runner.WriteMarkdownSummary(runs, path)
		fmt.Fprintf(os.Stderr, "Summary written to %s\n", path)
	}

	envPath := filepath.Join(outDir, "environment.json")
	runner.WriteEnvironment(envPath)

	return nil
}

func generateReport(inputPath, outputFormat string) error {
	info, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("input path does not exist: %s", inputPath)
	}

	resultsPath := inputPath
	if info.IsDir() {
		resultsPath = filepath.Join(inputPath, "full_results.json")
	}

	data, err := os.ReadFile(resultsPath)
	if err != nil {
		return fmt.Errorf("failed to read results from %s: %w", resultsPath, err)
	}

	var wrapper runner.ResultsWrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("failed to parse results JSON: %w", err)
	}

	switch outputFormat {
	case "json":
		out, _ := json.MarshalIndent(wrapper, "", "  ")
		fmt.Println(string(out))
	default:
		var outputPath string
		if info.IsDir() {
			outputPath = filepath.Join(inputPath, "summary.md")
		} else {
			outputPath = strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + ".md"
		}
		runner.WriteMarkdownSummary(wrapper.Runs, outputPath)
		fmt.Fprintf(os.Stderr, "Summary written to %s\n", outputPath)
	}

	return nil
}

func manageManifests(generate, verify bool, fixture string) error {
	if !generate && !verify {
		return fmt.Errorf("specify --generate or --verify")
	}

	var fixtures []string
	if fixture != "" {
		fixtures = []string{fixture}
	} else {
		var err error
		fixtures, err = findAllFixtureDirs()
		if err != nil {
			return err
		}
	}

	if len(fixtures) == 0 {
		return fmt.Errorf("no fixture directories found")
	}

	for _, f := range fixtures {
		if generate {
			if _, err := runner.GenerateManifest(f); err != nil {
				return err
			}
		}
		if verify {
			ok, failures := runner.VerifyManifest(f, nil)
			if !ok {
				return fmt.Errorf("fixture integrity check failed for %s:\n%s", f, strings.Join(failures, "\n"))
			}
		}
	}

	return nil
}

func verifyWorkDir(dir, taskID string, asJSON bool) error {
	if !runner.DirExistsPublic(dir) {
		return fmt.Errorf("work directory does not exist: %s", dir)
	}

	// Determine task ID from args or directory name
	if taskID == "" {
		dirName := filepath.Base(dir)
		taskID = extractTaskIDFromDirName(dirName)
		if taskID == "" {
			return fmt.Errorf("could not extract task ID from directory name '%s'. Use --task to specify", dirName)
		}
	}

	tasksDir, err := findTasksDir()
	if err != nil {
		return err
	}
	allTasks, err := runner.LoadTasks(tasksDir)
	if err != nil {
		return err
	}

	var taskDef *runner.TaskDef
	for i := range allTasks {
		if allTasks[i].Task.ID == taskID {
			taskDef = &allTasks[i]
			break
		}
	}
	if taskDef == nil {
		return fmt.Errorf("task '%s' not found in task definitions", taskID)
	}

	fmt.Fprintf(os.Stderr, "Verifying work directory for task '%s'...\n", taskID)
	result, err := runner.Verify(*taskDef, dir)
	if err != nil {
		return err
	}

	if asJSON {
		output := map[string]interface{}{
			"task_id":          taskID,
			"compilation_pass": result.CompilationPass,
			"tests_pass":       result.TestsPass,
			"caller_coverage":  result.CallerCoverage,
			"overall_score":    result.OverallScore,
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
	} else {
		pass := func(b bool) string {
			if b {
				return "PASS"
			}
			return "FAIL"
		}
		coverageStr := "N/A"
		if result.CallerCoverage != nil {
			coverageStr = fmt.Sprintf("%.0f%%", *result.CallerCoverage*100)
		}
		fmt.Printf("Verification: %s\n", taskID)
		fmt.Printf("  Compilation: %s\n", pass(result.CompilationPass))
		fmt.Printf("  Tests:       %s\n", pass(result.TestsPass))
		fmt.Printf("  Coverage:    %s\n", coverageStr)
		fmt.Printf("  Score:       %d\n", result.OverallScore)
	}

	return nil
}

func runTest(taskFilter, languageFilter, model string) error {
	tasksDir, err := findTasksDir()
	if err != nil {
		return err
	}
	allTasks, err := runner.LoadTasks(tasksDir)
	if err != nil {
		return err
	}

	// Select task
	var taskDef *runner.TaskDef
	if taskFilter != "" {
		for i := range allTasks {
			if allTasks[i].Task.ID == taskFilter {
				taskDef = &allTasks[i]
				break
			}
		}
		if taskDef == nil {
			return fmt.Errorf("task '%s' not found", taskFilter)
		}
	} else if languageFilter != "" {
		for i := range allTasks {
			if allTasks[i].Task.Language == languageFilter {
				taskDef = &allTasks[i]
				break
			}
		}
		if taskDef == nil {
			return fmt.Errorf("no tasks found for language '%s'", languageFilter)
		}
	} else {
		for i := range allTasks {
			if allTasks[i].Task.ID == "ts-cat-a-01" {
				taskDef = &allTasks[i]
				break
			}
		}
		if taskDef == nil && len(allTasks) > 0 {
			taskDef = &allTasks[0]
		}
		if taskDef == nil {
			return fmt.Errorf("no tasks found")
		}
	}

	fmt.Fprintln(os.Stderr, "=== BENCHMARK TEST ===")
	fmt.Fprintf(os.Stderr, "Task: %s (%s)\n", taskDef.Task.ID, taskDef.Task.Description)
	fmt.Fprintf(os.Stderr, "Model: %s\n", model)
	fmt.Fprintln(os.Stderr, "Conditions: without-inari, with-inari, with-inari-preloaded")
	fmt.Fprintln(os.Stderr)

	corpusPath, err := resolveCorpusPath(*taskDef)
	if err != nil {
		return err
	}

	// Verify fixture integrity
	manifestFile := filepath.Join(corpusPath, ".fixture-manifest.sha256")
	if runner.FileExistsPublic(manifestFile) {
		ok, failures := runner.VerifyManifest(corpusPath, nil)
		if !ok {
			return fmt.Errorf("fixture integrity check failed:\n%s", strings.Join(failures, "\n"))
		}
		fmt.Fprintln(os.Stderr, "Fixture integrity: OK")
	}

	// Back up inari index
	inariDir := filepath.Join(corpusPath, ".inari")
	if !runner.DirExistsPublic(inariDir) {
		return fmt.Errorf("no .inari/ directory found in fixture. Run 'inari index' first")
	}
	backup, err := runner.BackupInariIndex(corpusPath)
	if err != nil {
		return err
	}

	// Create temp output dir
	testOutput := "benchmarks/results/test"
	os.MkdirAll(testOutput, 0o755)
	ndjsonDir := filepath.Join(testOutput, "ndjson")
	os.MkdirAll(ndjsonDir, 0o755)

	type condDef struct {
		label        string
		inariEnabled bool
	}

	conditions := []condDef{
		{"without-inari", false},
		{"with-inari", true},
		{"with-inari-preloaded", true},
	}

	var allRuns []runner.BenchmarkRun
	allValid := true

	for _, cond := range conditions {
		fmt.Fprintf(os.Stderr, "\n--- %s ---\n", cond.label)

		ndjsonPath := filepath.Join(ndjsonDir, fmt.Sprintf("%s-%s.ndjson", taskDef.Task.ID, cond.label))

		agentRun, workDir, err := runner.RunAgent(*taskDef, cond.inariEnabled, cond.label, model, corpusPath, backup, ndjsonPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
			allValid = false
			continue
		}

		verification, _ := runner.Verify(*taskDef, workDir)
		bm := runner.ComputeBehaviorMetrics(agentRun.Actions)

		// Validate telemetry
		var issues []string
		if agentRun.InputTokens == 0 {
			issues = append(issues, "input_tokens = 0")
		}
		if agentRun.OutputTokens == 0 {
			issues = append(issues, "output_tokens = 0")
		}
		if len(agentRun.Actions) == 0 {
			issues = append(issues, "actions[] is empty")
		}
		if agentRun.FileReads == 0 {
			issues = append(issues, "file_reads = 0")
		}
		if cond.inariEnabled && len(agentRun.InariCommandsCalled) == 0 {
			issues = append(issues, "inari_commands_called is empty (agent may not have used inari)")
		}

		if runner.FileExistsPublic(ndjsonPath) {
			info, _ := os.Stat(ndjsonPath)
			if info != nil && info.Size() == 0 {
				issues = append(issues, "NDJSON file is empty")
			} else if info != nil {
				fmt.Fprintf(os.Stderr, "  NDJSON saved: %s (%d bytes)\n", ndjsonPath, info.Size())
			}
		} else {
			issues = append(issues, "NDJSON file was not saved")
		}

		fmt.Fprintf(os.Stderr, "  Input tokens:    %d\n", agentRun.InputTokens)
		fmt.Fprintf(os.Stderr, "  Output tokens:   %d\n", agentRun.OutputTokens)
		fmt.Fprintf(os.Stderr, "  Cache read:      %d\n", agentRun.CacheReadInputTokens)
		fmt.Fprintf(os.Stderr, "  Cache creation:  %d\n", agentRun.CacheCreationInputTokens)
		fmt.Fprintf(os.Stderr, "  File reads:      %d\n", agentRun.FileReads)
		fmt.Fprintf(os.Stderr, "  Actions:         %d\n", len(agentRun.Actions))
		fmt.Fprintf(os.Stderr, "  Inari commands:  %v\n", agentRun.InariCommandsCalled)
		fmt.Fprintf(os.Stderr, "  Duration:        %dms\n", agentRun.DurationMs)

		compStr := "FAIL"
		if verification.CompilationPass {
			compStr = "PASS"
		}
		fmt.Fprintf(os.Stderr, "  Compilation:     %s\n", compStr)

		if len(issues) == 0 {
			fmt.Fprintln(os.Stderr, "  Telemetry:       VALID")
		} else {
			fmt.Fprintln(os.Stderr, "  Telemetry:       ISSUES DETECTED")
			for _, issue := range issues {
				fmt.Fprintf(os.Stderr, "    WARNING: %s\n", issue)
			}
			allValid = false
		}

		allRuns = append(allRuns, runner.BenchmarkRun{
			TaskID:                   taskDef.Task.ID,
			Repetition:               1,
			InariEnabled:             cond.inariEnabled,
			Condition:                cond.label,
			InputTokens:              agentRun.InputTokens,
			OutputTokens:             agentRun.OutputTokens,
			CacheCreationInputTokens: agentRun.CacheCreationInputTokens,
			CacheReadInputTokens:     agentRun.CacheReadInputTokens,
			FileReads:                agentRun.FileReads,
			InariCommandsCalled:      agentRun.InariCommandsCalled,
			Correctness: runner.CorrectnessResult{
				CompilationPass: verification.CompilationPass,
				TestsPass:       verification.TestsPass,
				CallerCoverage:  verification.CallerCoverage,
				OverallScore:    verification.OverallScore,
			},
			DurationMs: agentRun.DurationMs,
			Actions:    agentRun.Actions,
			Behavior:   &bm,
		})

		os.RemoveAll(workDir)
	}

	// Write test results
	jsonPath := filepath.Join(testOutput, "test_results.json")
	runner.WriteJSONResults(allRuns, jsonPath)
	mdPath := filepath.Join(testOutput, "test_summary.md")
	runner.WriteMarkdownSummary(allRuns, mdPath)

	fmt.Fprintln(os.Stderr, "\n=== TEST COMPLETE ===")
	fmt.Fprintf(os.Stderr, "Results: %s\n", testOutput)

	if allValid {
		fmt.Fprintln(os.Stderr, "Telemetry: ALL VALID - safe to proceed with full benchmark run")
	} else {
		fmt.Fprintln(os.Stderr, "Telemetry: ISSUES DETECTED - fix before running full benchmarks")
		return fmt.Errorf("telemetry validation failed. See warnings above")
	}

	return nil
}

// ===========================================================================
// Helper functions
// ===========================================================================

func filterTasks(allTasks []runner.TaskDef, all bool, taskFilter, categoryFilter, languageFilter string) []runner.TaskDef {
	if all {
		return allTasks
	}

	var result []runner.TaskDef
	for _, t := range allTasks {
		if taskFilter != "" && t.Task.ID == taskFilter {
			result = append(result, t)
		} else if categoryFilter != "" && t.Task.Category == categoryFilter {
			result = append(result, t)
		} else if languageFilter != "" && t.Task.Language == languageFilter {
			result = append(result, t)
		}
	}
	return result
}

func findTasksDir() (string, error) {
	candidates := []string{
		"benchmarks/tasks",
		"../tasks",
		"../../benchmarks/tasks",
	}
	for _, c := range candidates {
		if runner.DirExistsPublic(c) {
			return c, nil
		}
	}
	return "", fmt.Errorf("could not find benchmarks/tasks/ directory. Run from the project root or benchmarks/runner/")
}

func findResultsDir() string {
	candidates := []string{
		"benchmarks/results/latest",
		"../results/latest",
		"../../benchmarks/results/latest",
	}
	for _, c := range candidates {
		parent := filepath.Dir(c)
		info, err := os.Stat(parent)
		if err == nil && info.IsDir() {
			return c
		}
	}
	return "benchmarks/results/latest"
}

func resolveCorpusPath(taskDef runner.TaskDef) (string, error) {
	corpusName := taskDef.Task.Corpus

	var candidates []string
	if corpusName == "fixture" {
		lang := taskDef.Task.Language
		candidates = []string{
			fmt.Sprintf("benchmarks/fixtures/%s-large", lang),
			fmt.Sprintf("../fixtures/%s-large", lang),
			fmt.Sprintf("../../benchmarks/fixtures/%s-large", lang),
			fmt.Sprintf("benchmarks/fixtures/%s-api", lang),
			fmt.Sprintf("../fixtures/%s-api", lang),
			fmt.Sprintf("../../benchmarks/fixtures/%s-api", lang),
		}
	} else {
		candidates = []string{
			fmt.Sprintf("benchmarks/corpora/%s", corpusName),
			fmt.Sprintf("../corpora/%s", corpusName),
			fmt.Sprintf("../../benchmarks/corpora/%s", corpusName),
		}
	}

	for _, c := range candidates {
		if runner.DirExistsPublic(c) {
			return c, nil
		}
	}

	return "", fmt.Errorf("corpus '%s' for task '%s' not found", corpusName, taskDef.Task.ID)
}

func findAllFixtureDirs() ([]string, error) {
	candidates := []string{
		"benchmarks/fixtures",
		"../fixtures",
		"../../benchmarks/fixtures",
	}

	for _, base := range candidates {
		if !runner.DirExistsPublic(base) {
			continue
		}
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		var dirs []string
		for _, e := range entries {
			if e.IsDir() {
				name := e.Name()
				if strings.Contains(name, "-large") || strings.Contains(name, "-api") {
					dirs = append(dirs, filepath.Join(base, name))
				}
			}
		}
		if len(dirs) > 0 {
			return dirs, nil
		}
	}

	return nil, fmt.Errorf("could not find benchmarks/fixtures/ directory")
}

func extractTaskIDFromDirName(dirName string) string {
	suffixes := []string{"-with-inari-preloaded", "-without-inari", "-with-inari"}
	for _, suffix := range suffixes {
		if idx := strings.Index(dirName, suffix); idx >= 0 {
			return dirName[:idx]
		}
	}
	return ""
}

func formatDuration(secs uint64) string {
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	} else if secs < 3600 {
		return fmt.Sprintf("%dm %ds", secs/60, secs%60)
	}
	return fmt.Sprintf("%dh %dm %ds", secs/3600, (secs%3600)/60, secs%60)
}

func runInariMap(dir string) (string, error) {
	cmd := exec.Command("inari", "map")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
