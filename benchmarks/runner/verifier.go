package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// VerificationResult holds the result of verifying an agent's work on a task.
type VerificationResult struct {
	CompilationPass bool     `json:"compilation_pass"`
	TestsPass       bool     `json:"tests_pass"`
	CallerCoverage  *float64 `json:"caller_coverage,omitempty"`
	OverallScore    uint32   `json:"overall_score"`
	Errors          []string `json:"errors,omitempty"`
}

// Verify checks the correctness of an agent's work on a task.
// Runs compilation, tests, and (where applicable) caller coverage checks
// against the corpus after the agent has made its changes.
func Verify(task TaskDef, workDir string) (*VerificationResult, error) {
	var errors []string

	compilationPass := true
	if task.Correctness.RequireCompilation {
		pass, err := runCompilation(workDir, task.Task.Language)
		if err != nil {
			errors = append(errors, fmt.Sprintf("compilation check error: %v", err))
			compilationPass = false
		} else {
			compilationPass = pass
		}
	}

	testsPass := true
	if task.Correctness.RequireTestsPass {
		pass, err := runTests(workDir, task.Task.Language)
		if err != nil {
			errors = append(errors, fmt.Sprintf("test check error: %v", err))
			testsPass = false
		} else {
			testsPass = pass
		}
	}

	var callerCoverage *float64
	if task.Correctness.RequireCallerCoverage {
		coverage, err := checkCallerCoverage(workDir, task)
		if err != nil {
			errors = append(errors, fmt.Sprintf("caller coverage error: %v", err))
			zero := 0.0
			callerCoverage = &zero
		} else {
			callerCoverage = &coverage
		}
	}

	overallScore := calculateScore(compilationPass, testsPass, callerCoverage, task)

	if errors == nil {
		errors = []string{}
	}

	return &VerificationResult{
		CompilationPass: compilationPass,
		TestsPass:       testsPass,
		CallerCoverage:  callerCoverage,
		OverallScore:    overallScore,
		Errors:          errors,
	}, nil
}

// verifyTypeScript runs TypeScript compilation and test checks.
func verifyTypeScript(workDir string) (bool, bool) {
	compPass, _ := runCompilation(workDir, "typescript")
	testPass, _ := runTests(workDir, "typescript")
	return compPass, testPass
}

// verifyCSharp runs C# compilation and test checks.
func verifyCSharp(workDir string) (bool, bool) {
	compPass, _ := runCompilation(workDir, "csharp")
	testPass, _ := runTests(workDir, "csharp")
	return compPass, testPass
}

// verifyCallerCoverage checks inari refs output for the task.
func verifyCallerCoverage(task TaskDef, workDir string) (float64, error) {
	return checkCallerCoverage(workDir, task)
}

// runCompilation runs the language-specific compilation check.
func runCompilation(workDir, language string) (bool, error) {
	var cmd *exec.Cmd

	switch language {
	case "typescript":
		cmd = exec.Command("npx", "tsc", "--noEmit")
	case "csharp":
		cmd = exec.Command("dotnet", "build", "--no-restore")
	default:
		return false, fmt.Errorf("unsupported language for compilation check: %s", language)
	}

	cmd.Dir = workDir
	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, fmt.Errorf("failed to run compilation for %s: %w", language, err)
	}
	return true, nil
}

// runTests runs the language-specific test suite.
func runTests(workDir, language string) (bool, error) {
	var cmd *exec.Cmd

	switch language {
	case "typescript":
		cmd = exec.Command("npm", "test")
	case "csharp":
		cmd = exec.Command("dotnet", "test", "--no-restore")
	default:
		return false, fmt.Errorf("unsupported language for test check: %s", language)
	}

	cmd.Dir = workDir
	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, fmt.Errorf("failed to run tests for %s: %w", language, err)
	}
	return true, nil
}

// parseDiffModifications parses a unified diff and extracts modified line numbers per file.
// Returns a map from file path to set of modified line numbers.
func parseDiffModifications(diffText string) map[string]map[int64]struct{} {
	modifications := make(map[string]map[int64]struct{})
	var currentFile string
	var currentLine int64

	var pendingFromFile string

	for _, line := range strings.Split(diffText, "\n") {
		// Track the "from" file for deleted-file diffs.
		if rest, found := strings.CutPrefix(line, "--- a/"); found {
			pendingFromFile = rest
			continue
		}
		// Track current file from "+++ b/path" lines.
		// For deleted files, +++ is "/dev/null" — use the --- path instead.
		if rest, found := strings.CutPrefix(line, "+++ b/"); found {
			currentFile = rest
			pendingFromFile = ""
			continue
		}
		if strings.HasPrefix(line, "+++ /dev/null") {
			currentFile = pendingFromFile
			pendingFromFile = ""
			continue
		}
		if strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") {
			continue
		}

		// Parse hunk headers: @@ -old_start,old_count +new_start,new_count @@
		if strings.HasPrefix(line, "@@ ") {
			plusIdx := strings.Index(line, "+")
			if plusIdx >= 0 {
				afterPlus := line[plusIdx+1:]
				var numStr strings.Builder
				for _, ch := range afterPlus {
					if ch >= '0' && ch <= '9' {
						numStr.WriteRune(ch)
					} else {
						break
					}
				}
				if start, err := strconv.ParseInt(numStr.String(), 10, 64); err == nil {
					currentLine = start
				}
			}
			continue
		}

		// Track modifications
		if currentFile != "" {
			if strings.HasPrefix(line, "+") {
				if modifications[currentFile] == nil {
					modifications[currentFile] = make(map[int64]struct{})
				}
				modifications[currentFile][currentLine] = struct{}{}
				currentLine++
			} else if strings.HasPrefix(line, "-") {
				// Deleted line — don't increment new line counter
			} else {
				// Context line
				currentLine++
			}
		}
	}

	return modifications
}

// isLineModified checks if a line (within a context window) appears in the diff modifications.
func isLineModified(modifications map[string]map[int64]struct{}, filePath string, line, context int64) bool {
	fileMods, ok := modifications[filePath]
	if !ok {
		return false
	}
	for modLine := range fileMods {
		if abs64(line-modLine) <= context {
			return true
		}
	}
	return false
}

// parseCallerLocation parses a caller location string in "file_path:line" format.
func parseCallerLocation(s string) (string, int64, error) {
	colonIdx := strings.LastIndex(s, ":")
	if colonIdx < 0 {
		return "", 0, fmt.Errorf("invalid caller location format '%s': expected 'file:line'", s)
	}
	filePath := s[:colonIdx]
	line, err := strconv.ParseInt(s[colonIdx+1:], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid line number in caller location '%s': %w", s, err)
	}
	return filePath, line, nil
}

// checkCallerCoverage checks caller coverage by comparing git diff against ground truth callers.
func checkCallerCoverage(workDir string, task TaskDef) (float64, error) {
	// Ensure the work directory is a git repo so `git diff HEAD` works.
	// The temp corpus may not have been initialized with git.
	gitDir := filepath.Join(workDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		initCmd := exec.Command("git", "init")
		initCmd.Dir = workDir
		if out, initErr := initCmd.CombinedOutput(); initErr != nil {
			return 0, fmt.Errorf("failed to init git in work dir: %s: %w", string(out), initErr)
		}
		addCmd := exec.Command("git", "add", "-A")
		addCmd.Dir = workDir
		if out, addErr := addCmd.CombinedOutput(); addErr != nil {
			return 0, fmt.Errorf("failed to stage files in work dir: %s: %w", string(out), addErr)
		}
		commitCmd := exec.Command("git", "commit", "-m", "baseline", "--allow-empty")
		commitCmd.Dir = workDir
		if out, commitErr := commitCmd.CombinedOutput(); commitErr != nil {
			return 0, fmt.Errorf("failed to create baseline commit in work dir: %s: %w", string(out), commitErr)
		}
	}

	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to run git diff for caller coverage check: %w", err)
	}

	diffText := string(output)
	if diffText == "" {
		return 0, nil
	}

	callers := task.GroundTruth.Callers
	if len(callers) == 0 {
		fmt.Fprintf(os.Stderr, "  [verifier] No ground truth callers defined for %s. Skipping coverage check.\n", task.Target.Symbol)
		return 0, nil
	}

	modifications := parseDiffModifications(diffText)

	const contextWindow int64 = 5
	total := len(callers)
	covered := 0

	for _, caller := range callers {
		filePath, line, err := parseCallerLocation(caller)
		if err != nil {
			return 0, err
		}
		if isLineModified(modifications, filePath, line, contextWindow) {
			covered++
		}
	}

	coverage := float64(covered) / float64(total)
	fmt.Fprintf(os.Stderr, "  [verifier] Caller coverage for %s: %d/%d (%.0f%%)\n",
		task.Target.Symbol, covered, total, coverage*100)

	return coverage, nil
}

// calculateScore computes the overall correctness score (0-100) from individual checks.
func calculateScore(compilationPass, testsPass bool, callerCoverage *float64, task TaskDef) uint32 {
	if callerCoverage != nil {
		var score uint32
		if compilationPass {
			score += 40
		}
		if testsPass {
			score += 40
		}
		coverage := *callerCoverage
		if coverage >= task.Correctness.CallerCoverageThreshold {
			score += 20
		} else if task.Correctness.CallerCoverageThreshold > 0 {
			score += uint32(coverage / task.Correctness.CallerCoverageThreshold * 20)
		}
		return score
	}

	var score uint32
	if compilationPass {
		score += 50
	}
	if testsPass {
		score += 50
	}
	return score
}

// abs64 returns the absolute value of an int64.
func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
