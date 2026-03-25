package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// TaskCategory represents the type of benchmark task.
type TaskCategory string

const (
	TaskCategoryDiscovery          TaskCategory = "discovery"
	TaskCategoryBugFix             TaskCategory = "bug_fix"
	TaskCategoryRefactoring        TaskCategory = "refactoring"
	TaskCategoryNewFeature         TaskCategory = "new_feature"
	TaskCategoryFocusedExploration TaskCategory = "focused_exploration"
	TaskCategoryCrossCutting       TaskCategory = "cross_cutting"
)

// TaskDef is a complete task definition loaded from a TOML file.
type TaskDef struct {
	Task        TaskMeta       `toml:"task"`
	Prompt      PromptDef      `toml:"prompt"`
	Target      TargetDef      `toml:"target"`
	Correctness CorrectnessDef `toml:"correctness"`
	GroundTruth GroundTruthDef `toml:"ground_truth"`
	Inari       InariDef       `toml:"inari"`
}

// TaskMeta holds metadata about the task.
type TaskMeta struct {
	ID          string `toml:"id"`
	Category    string `toml:"category"`
	Language    string `toml:"language"`
	Corpus      string `toml:"corpus"`
	Description string `toml:"description"`
}

// PromptDef contains the prompt sent to the coding agent.
type PromptDef struct {
	Text string `toml:"text"`
}

// TargetDef identifies the target symbol and file for the task.
type TargetDef struct {
	Symbol string `toml:"symbol"`
	File   string `toml:"file"`
}

// CorrectnessDef specifies correctness criteria for verification.
type CorrectnessDef struct {
	RequireCompilation      bool    `toml:"require_compilation"`
	RequireTestsPass        bool    `toml:"require_tests_pass"`
	RequireCallerCoverage   bool    `toml:"require_caller_coverage"`
	CallerCoverageThreshold float64 `toml:"caller_coverage_threshold"`
	PatternMatchThreshold   float64 `toml:"pattern_match_threshold"`
}

// GroundTruthDef holds ground truth data for verification.
type GroundTruthDef struct {
	Callers []string `toml:"callers"`
}

// InariDef contains expected Inari commands for the task.
type InariDef struct {
	ExpectedCommands []string `toml:"expected_commands"`
}

// TaskCondition pairs a condition label with a CLAUDE.md variant.
type TaskCondition struct {
	Condition string
	ClaudeMD  string
}

// LoadTasks walks a directory recursively and loads all .toml task files.
// Returns tasks sorted by ID for deterministic ordering.
func LoadTasks(dir string) ([]TaskDef, error) {
	var tasks []TaskDef

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".toml") {
			return nil
		}

		task, loadErr := loadTask(path)
		if loadErr != nil {
			return fmt.Errorf("failed to load task from %s: %w", path, loadErr)
		}
		tasks = append(tasks, task)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Task.ID < tasks[j].Task.ID
	})

	return tasks, nil
}

// loadTask parses a single TOML task file and validates required fields.
func loadTask(path string) (TaskDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TaskDef{}, fmt.Errorf("could not read task file %s: %w", path, err)
	}

	var task TaskDef
	if err := toml.Unmarshal(data, &task); err != nil {
		return TaskDef{}, fmt.Errorf("invalid TOML in task file %s: %w", path, err)
	}

	if task.Task.ID == "" {
		return TaskDef{}, fmt.Errorf("task in %s has empty id", path)
	}
	if task.Prompt.Text == "" {
		return TaskDef{}, fmt.Errorf("task %s has empty prompt text", task.Task.ID)
	}

	return task, nil
}
