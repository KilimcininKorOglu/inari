package runner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// stderrMu synchronizes progress writes to stderr in parallel mode.
var stderrMu sync.Mutex

// AgentAction represents a single tool call made by the agent during a benchmark run.
type AgentAction struct {
	Sequence         uint32 `json:"sequence"`
	ToolName         string `json:"tool_name"`
	ArgumentsSummary string `json:"arguments_summary"`
	IsNavigation     bool   `json:"is_navigation"`
	IsInariCommand   bool   `json:"is_inari_command"`
	IsEdit           bool   `json:"is_edit"`
}

// AgentRun captures data from a single agent run against a task.
type AgentRun struct {
	TaskID                   string        `json:"task_id"`
	InariEnabled             bool          `json:"inari_enabled"`
	InputTokens              uint64        `json:"input_tokens"`
	OutputTokens             uint64        `json:"output_tokens"`
	CacheCreationInputTokens uint64        `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     uint64        `json:"cache_read_input_tokens"`
	FileReads                uint32        `json:"file_reads"`
	FileEdits                uint32        `json:"file_edits"`
	GrepCalls                uint32        `json:"grep_calls"`
	GlobCalls                uint32        `json:"glob_calls"`
	BashCalls                uint32        `json:"bash_calls"`
	InariCommandsCalled      []string      `json:"inari_commands_called"`
	DurationMs               uint64        `json:"duration_ms"`
	ExitCode                 int           `json:"exit_code"`
	Actions                  []AgentAction `json:"actions"`
}

// NdjsonParseResult holds parsed data from a saved NDJSON stream.
type NdjsonParseResult struct {
	Actions                  []AgentAction
	InputTokens              uint64
	OutputTokens             uint64
	CacheCreationInputTokens uint64
	CacheReadInputTokens     uint64
	InariCommandsCalled      []string
	FileReads                uint32
}

// RunAgent spawns the claude CLI against a task and captures the NDJSON stream.
// It creates an isolated temp directory with the corpus, installs the appropriate
// CLAUDE.md variant, and parses the output for tool calls and token usage.
//
// The caller must keep the returned workDir path available until verification
// completes. The workDir is a temporary directory that should be cleaned up
// by the caller when done.
func RunAgent(task TaskDef, inariEnabled bool, condition, model string, corpusPath string, inariBackup string, ndjsonSavePath string) (*AgentRun, string, error) {
	// Pre-flight: check that the claude CLI is available
	if _, err := exec.LookPath("claude"); err != nil {
		return nil, "", fmt.Errorf("'claude' CLI not found in PATH. Install Claude Code first")
	}

	// Set up isolated temp directory
	workDir, err := setupTempCorpus(corpusPath, inariEnabled, condition, inariBackup)
	if err != nil {
		return nil, "", fmt.Errorf("failed to set up temp corpus: %w", err)
	}

	// Build the claude command
	args := []string{
		"-p", task.Prompt.Text,
		"--add-dir", workDir,
		"--output-format", "stream-json",
		"--verbose",
		"--max-turns", "30",
		"--allowedTools", "Read,Edit,Write,Glob,Grep,Bash",
	}

	if model != "" {
		args = append(args, "--model", model)
	}

	if !inariEnabled {
		args = append(args, "--disallowedTools", "Bash(inari:*)")
	}

	cmd := exec.Command("claude", args...)
	cmd.Dir = workDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, workDir, fmt.Errorf("failed to capture stdout: %w", err)
	}
	cmd.Stderr = nil // suppress stderr

	start := time.Now()

	if err := cmd.Start(); err != nil {
		return nil, workDir, fmt.Errorf("failed to spawn 'claude' CLI. Is it installed and on PATH? %w", err)
	}

	// Parse the NDJSON stream
	var (
		inputTokens              uint64
		outputTokens             uint64
		cacheCreationInputTokens uint64
		cacheReadInputTokens     uint64
		fileReads                uint32
		fileEdits                uint32
		grepCalls                uint32
		globCalls                uint32
		bashCalls                uint32
		inariCommandsCalled      []string
		actions                  []AgentAction
		sequence                 uint32
		rawLines                 []string
	)

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		line := scanner.Text()
		rawLines = append(rawLines, line)

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		var value map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
			continue
		}

		// Accumulate usage — only from one source per message to avoid double-counting.
		// Result messages carry usage inside result.usage; others at top level.
		if getStringField(value, "type") == "result" {
			if result, ok := value["result"].(map[string]interface{}); ok {
				accumulateUsage(result, &inputTokens, &outputTokens, &cacheCreationInputTokens, &cacheReadInputTokens)
			}
		} else {
			accumulateUsage(value, &inputTokens, &outputTokens, &cacheCreationInputTokens, &cacheReadInputTokens)
		}

		// Extract tool_use events
		toolItems := extractToolUseItems(value)

		for _, item := range toolItems {
			toolName := getStringField(item, "name")
			if toolName == "" {
				continue
			}

			inputObj, _ := item["input"].(map[string]interface{})
			argsSummary := extractArgumentSummary(toolName, inputObj)

			isNav := isNavigationTool(toolName)
			isEd := isEditTool(toolName)
			isInariCmd := false

			switch toolName {
			case "Read":
				fileReads++
			case "Edit", "Write":
				fileEdits++
			case "Grep":
				grepCalls++
			case "Glob":
				globCalls++
			case "Bash":
				bashCalls++
				if cmdStr, ok := inputObj["command"].(string); ok {
					if inariCmd := extractInariCommand(cmdStr); inariCmd != "" {
						isInariCmd = true
						if !containsString(inariCommandsCalled, inariCmd) {
							inariCommandsCalled = append(inariCommandsCalled, inariCmd)
						}
					}
				}
			}

			sequence++
			actions = append(actions, AgentAction{
				Sequence:         sequence,
				ToolName:         toolName,
				ArgumentsSummary: argsSummary,
				IsNavigation:     isNav,
				IsInariCommand:   isInariCmd,
				IsEdit:           isEd,
			})

			elapsed := time.Since(start).Seconds()
			stderrMu.Lock()
			fmt.Fprintf(os.Stderr, "\r         %ds | %d actions | %d reads | %d out tokens | last: %s        ",
				int(elapsed), sequence, fileReads, outputTokens, toolName)
			stderrMu.Unlock()
		}
	}
	stderrMu.Lock()
	fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", 80))
	stderrMu.Unlock()

	waitErr := cmd.Wait()
	durationMs := uint64(time.Since(start).Milliseconds())

	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	if exitCode != 0 || len(rawLines) == 0 {
		if len(rawLines) == 0 {
			stderrMu.Lock()
			fmt.Fprintf(os.Stderr, "  [agent] WARNING: claude CLI produced no stdout output (0 NDJSON lines)\n")
			fmt.Fprintf(os.Stderr, "  [agent] Exit code: %d, Duration: %dms\n", exitCode, durationMs)
			stderrMu.Unlock()
		}
	}

	// Save raw NDJSON if path provided
	if ndjsonSavePath != "" {
		parentDir := filepath.Dir(ndjsonSavePath)
		if err := os.MkdirAll(parentDir, 0o755); err != nil {
			return nil, workDir, fmt.Errorf("failed to create NDJSON parent dir: %w", err)
		}
		content := strings.Join(rawLines, "\n")
		if err := os.WriteFile(ndjsonSavePath, []byte(content), 0o644); err != nil {
			return nil, workDir, fmt.Errorf("failed to save NDJSON to %s: %w", ndjsonSavePath, err)
		}
	}

	if inariCommandsCalled == nil {
		inariCommandsCalled = []string{}
	}
	if actions == nil {
		actions = []AgentAction{}
	}

	run := &AgentRun{
		TaskID:                   task.Task.ID,
		InariEnabled:             inariEnabled,
		InputTokens:              inputTokens,
		OutputTokens:             outputTokens,
		CacheCreationInputTokens: cacheCreationInputTokens,
		CacheReadInputTokens:     cacheReadInputTokens,
		FileReads:                fileReads,
		FileEdits:                fileEdits,
		GrepCalls:                grepCalls,
		GlobCalls:                globCalls,
		BashCalls:                bashCalls,
		InariCommandsCalled:      inariCommandsCalled,
		DurationMs:               durationMs,
		ExitCode:                 exitCode,
		Actions:                  actions,
	}

	return run, workDir, nil
}

// ParseNDJSON parses a saved NDJSON stream from a reader into structured action data.
func ParseNDJSON(reader io.Reader) ([]AgentAction, error) {
	result := parseNDJSONFull(reader)
	return result.Actions, nil
}

// parseNDJSONFull parses a saved NDJSON stream into a full NdjsonParseResult.
func parseNDJSONFull(reader io.Reader) NdjsonParseResult {
	var (
		inputTokens              uint64
		outputTokens             uint64
		cacheCreationInputTokens uint64
		cacheReadInputTokens     uint64
		fileReads                uint32
		inariCommandsCalled      []string
		actions                  []AgentAction
		sequence                 uint32
		skippedLines             uint32
	)

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "" {
			continue
		}

		var value map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
			skippedLines++
			continue
		}

		// Accumulate usage — only from one source per message to avoid double-counting.
		// Result messages carry usage inside result.usage; others at top level.
		if getStringField(value, "type") == "result" {
			if result, ok := value["result"].(map[string]interface{}); ok {
				accumulateUsage(result, &inputTokens, &outputTokens, &cacheCreationInputTokens, &cacheReadInputTokens)
			}
		} else {
			accumulateUsage(value, &inputTokens, &outputTokens, &cacheCreationInputTokens, &cacheReadInputTokens)
		}

		// Extract tool_use events
		toolItems := extractToolUseItems(value)

		for _, item := range toolItems {
			toolName := getStringField(item, "name")
			if toolName == "" {
				continue
			}

			inputObj, _ := item["input"].(map[string]interface{})
			argsSummary := extractArgumentSummary(toolName, inputObj)

			isNav := isNavigationTool(toolName)
			isEd := isEditTool(toolName)
			isInariCmd := false

			if toolName == "Read" {
				fileReads++
			}
			if toolName == "Bash" {
				if cmdStr, ok := inputObj["command"].(string); ok {
					if inariCmd := extractInariCommand(cmdStr); inariCmd != "" {
						isInariCmd = true
						if !containsString(inariCommandsCalled, inariCmd) {
							inariCommandsCalled = append(inariCommandsCalled, inariCmd)
						}
					}
				}
			}

			sequence++
			actions = append(actions, AgentAction{
				Sequence:         sequence,
				ToolName:         toolName,
				ArgumentsSummary: argsSummary,
				IsNavigation:     isNav,
				IsInariCommand:   isInariCmd,
				IsEdit:           isEd,
			})
		}
	}

	if skippedLines > 0 {
		fmt.Fprintf(os.Stderr, "  [ndjson] Skipped %d non-JSON lines\n", skippedLines)
	}

	if inariCommandsCalled == nil {
		inariCommandsCalled = []string{}
	}
	if actions == nil {
		actions = []AgentAction{}
	}

	return NdjsonParseResult{
		Actions:                  actions,
		InputTokens:              inputTokens,
		OutputTokens:             outputTokens,
		CacheCreationInputTokens: cacheCreationInputTokens,
		CacheReadInputTokens:     cacheReadInputTokens,
		InariCommandsCalled:      inariCommandsCalled,
		FileReads:                fileReads,
	}
}

// setupTempCorpus creates an isolated temporary directory for a benchmark run.
// Copies the fixture corpus, installs the appropriate CLAUDE.md variant, and
// optionally restores the .inari/ index for inari-enabled runs.
func setupTempCorpus(corpusPath string, inariEnabled bool, condition string, inariBackup string) (string, error) {
	tempDir, err := os.MkdirTemp("", "inari-benchmark-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	// cleanupOnError removes the temp directory if setup fails partway through.
	cleanupOnError := func(wrapped error) (string, error) {
		os.RemoveAll(tempDir)
		return "", wrapped
	}

	// Copy the entire fixture directory into the temp dir
	if err := copyDirRecursive(corpusPath, tempDir); err != nil {
		return cleanupOnError(fmt.Errorf("failed to copy corpus from %s: %w", corpusPath, err))
	}

	// Select and install the right CLAUDE.md variant
	variantName := "CLAUDE.md.without-inari"
	if inariEnabled {
		variantName = "CLAUDE.md.with-inari"
	}
	variantSrc := filepath.Join(tempDir, variantName)
	claudeMDDest := filepath.Join(tempDir, "CLAUDE.md")

	if fileExists(variantSrc) {
		if err := copyFile(variantSrc, claudeMDDest); err != nil {
			return cleanupOnError(fmt.Errorf("failed to copy %s to CLAUDE.md: %w", variantSrc, err))
		}
	} else {
		// Check the original corpus path
		variantOriginal := filepath.Join(corpusPath, variantName)
		if fileExists(variantOriginal) {
			if err := copyFile(variantOriginal, claudeMDDest); err != nil {
				return cleanupOnError(fmt.Errorf("failed to copy %s to CLAUDE.md: %w", variantOriginal, err))
			}
		}
	}

	if inariEnabled {
		// Restore the .inari/ directory from backup if provided
		if inariBackup != "" {
			inariDest := filepath.Join(tempDir, ".inari")
			if dirExists(inariDest) {
				os.RemoveAll(inariDest)
			}
			if err := copyDirRecursive(inariBackup, inariDest); err != nil {
				return cleanupOnError(fmt.Errorf("failed to restore .inari/ backup: %w", err))
			}
		}
	} else {
		// Ensure no .inari/ directory exists for no-inari runs
		inariDir := filepath.Join(tempDir, ".inari")
		if dirExists(inariDir) {
			os.RemoveAll(inariDir)
		}
	}

	// Handle preloaded inari map variant
	if condition == "with-inari-preloaded" {
		preloadedSrc := filepath.Join(tempDir, "CLAUDE.md.with-inari-preloaded")
		if fileExists(preloadedSrc) {
			// Resolve inari binary path — check PATH, then fall back to common locations
			inariBin, lookErr := exec.LookPath("inari")
			if lookErr != nil {
				// Try relative to current working directory
				cwd, _ := os.Getwd()
				candidates := []string{
					filepath.Join(cwd, "bin", "inari"),
					filepath.Join(filepath.Dir(corpusPath), "..", "bin", "inari"),
				}
				for _, c := range candidates {
					if fileExists(c) {
						inariBin = c
						break
					}
				}
			}

			if inariBin != "" {
				mapCmd := exec.Command(inariBin, "map")
				mapCmd.Dir = tempDir
				mapOutput, err := mapCmd.CombinedOutput()
				if err == nil {
					template, readErr := os.ReadFile(preloadedSrc)
					if readErr == nil {
						rendered := strings.ReplaceAll(string(template), "{{INARI_MAP_OUTPUT}}", string(mapOutput))
						os.WriteFile(claudeMDDest, []byte(rendered), 0o644)
					}
				} else {
					fmt.Fprintf(os.Stderr, "  Warning: inari map failed in temp dir: %s. Falling back to with-inari variant.\n", string(mapOutput))
					fallback := filepath.Join(tempDir, "CLAUDE.md.with-inari")
					if fileExists(fallback) {
						copyFile(fallback, claudeMDDest)
					}
				}
			} else {
				fmt.Fprintf(os.Stderr, "  Warning: inari binary not found. Falling back to with-inari variant.\n")
				fallback := filepath.Join(tempDir, "CLAUDE.md.with-inari")
				if fileExists(fallback) {
					copyFile(fallback, claudeMDDest)
				}
			}
		}
	}

	return tempDir, nil
}

// CopyDirForPrepare is a public wrapper for copying fixture directories.
func CopyDirForPrepare(src, dst string) error {
	return copyDirRecursive(src, dst)
}

// extractToolUseItems extracts tool_use items from a stream-json event.
// Handles multiple nesting patterns:
// - {"type": "assistant", "message": {"content": [{"type": "tool_use", ...}]}}
// - {"type": "content_block_start", "content_block": {"type": "tool_use", ...}}
func extractToolUseItems(value map[string]interface{}) []map[string]interface{} {
	var items []map[string]interface{}

	// Pattern 1: assistant message with content array
	if message, ok := value["message"].(map[string]interface{}); ok {
		if content, ok := message["content"].([]interface{}); ok {
			for _, item := range content {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if getStringField(itemMap, "type") == "tool_use" {
						items = append(items, itemMap)
					}
				}
			}
		}
	}

	// Pattern 2: content_block_start with tool_use block
	if getStringField(value, "type") == "content_block_start" {
		if block, ok := value["content_block"].(map[string]interface{}); ok {
			if getStringField(block, "type") == "tool_use" {
				items = append(items, block)
			}
		}
	}

	return items
}

// extractArgumentSummary returns a short human-readable summary of a tool call's arguments.
func extractArgumentSummary(toolName string, input map[string]interface{}) string {
	if input == nil {
		return ""
	}

	switch toolName {
	case "Read":
		if fp, ok := input["file_path"].(string); ok {
			return fp
		}
	case "Edit", "Write":
		if fp, ok := input["file_path"].(string); ok {
			return fp
		}
	case "Grep":
		if p, ok := input["pattern"].(string); ok {
			return p
		}
	case "Glob":
		if p, ok := input["pattern"].(string); ok {
			return p
		}
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			if len(cmd) > 100 {
				return cmd[:100] + "..."
			}
			return cmd
		}
	default:
		data, err := json.Marshal(input)
		if err != nil {
			return ""
		}
		s := string(data)
		if len(s) > 100 {
			return s[:100] + "..."
		}
		return s
	}
	return ""
}

// isNavigationTool returns true if the tool name is a navigation/exploration tool.
func isNavigationTool(name string) bool {
	switch name {
	case "Read", "Grep", "Glob", "Bash":
		return true
	}
	return false
}

// isEditTool returns true if the tool name is a file-editing tool.
func isEditTool(name string) bool {
	switch name {
	case "Edit", "Write":
		return true
	}
	return false
}

// extractInariCommand extracts the inari subcommand from a Bash command string.
// For example:
//   - "inari sketch PaymentService" -> "inari sketch"
//   - "cd foo && inari refs bar" -> "inari refs"
//   - "echo hello" -> ""
func extractInariCommand(bashCommand string) string {
	idx := strings.Index(bashCommand, "inari ")
	if idx < 0 {
		return ""
	}

	afterInari := bashCommand[idx+len("inari "):]
	fields := strings.Fields(afterInari)
	if len(fields) == 0 {
		return ""
	}

	subcommand := fields[0]
	if strings.HasPrefix(subcommand, "-") {
		return ""
	}

	return "inari " + subcommand
}

// accumulateUsage extracts token usage fields from a JSON object and adds them
// to the running totals.
func accumulateUsage(value map[string]interface{}, inputTokens, outputTokens, cacheCreation, cacheRead *uint64) {
	usage, ok := value["usage"].(map[string]interface{})
	if !ok {
		return
	}
	*inputTokens += getUint64Field(usage, "input_tokens")
	*outputTokens += getUint64Field(usage, "output_tokens")
	*cacheCreation += getUint64Field(usage, "cache_creation_input_tokens")
	*cacheRead += getUint64Field(usage, "cache_read_input_tokens")
}

// getStringField safely extracts a string field from a JSON object.
func getStringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getUint64Field safely extracts a numeric field from a JSON object as uint64.
func getUint64Field(m map[string]interface{}, key string) uint64 {
	if v, ok := m[key].(float64); ok {
		return uint64(v)
	}
	return 0
}

// containsString checks if a slice contains a specific string.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// FileExistsPublic is a public wrapper for file existence checks.
func FileExistsPublic(path string) bool {
	return fileExists(path)
}

// DirExistsPublic is a public wrapper for directory existence checks.
func DirExistsPublic(path string) bool {
	return dirExists(path)
}

// fileExists checks if a path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// dirExists checks if a path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// copyDirRecursive copies a directory and all its contents recursively.
func copyDirRecursive(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to compute relative path: %w", err)
		}

		target := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		parentDir := filepath.Dir(target)
		if err := os.MkdirAll(parentDir, 0o755); err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
