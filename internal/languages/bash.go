// Bash-specific metadata extraction and language plugin.
//
// Extracts function definitions and command calls from Bash/Shell scripts.
// Bash has no classes, interfaces, or modules — only functions.
// `source` and `.` commands are treated as imports.
package languages

import (
	"encoding/json"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"

	"github.com/KilimcininKorOglu/inari/queries"
)

// BashPlugin implements LanguagePlugin for Bash/Shell (.sh, .bash) files.
type BashPlugin struct{}

// BashMetadata holds structured metadata for a Bash symbol.
type BashMetadata struct{}

// Language returns Bash.
func (p *BashPlugin) Language() SupportedLanguage {
	return Bash
}

// Extensions returns ["sh", "bash"].
func (p *BashPlugin) Extensions() []string {
	return []string{"sh", "bash"}
}

// TSLanguage returns the tree-sitter Bash grammar.
func (p *BashPlugin) TSLanguage() *sitter.Language {
	return bash.GetLanguage()
}

// SymbolQuerySource returns the embedded Bash symbols.scm query.
func (p *BashPlugin) SymbolQuerySource() string {
	return queries.BashSymbolsQuery
}

// EdgeQuerySource returns the embedded Bash edges.scm query.
func (p *BashPlugin) EdgeQuerySource() string {
	return queries.BashEdgesQuery
}

// InferSymbolKind maps Bash tree-sitter node types to Inari symbol kinds.
func (p *BashPlugin) InferSymbolKind(nodeKind string) string {
	return "function"
}

// ScopeNodeTypes returns node types that define scope boundaries in Bash.
func (p *BashPlugin) ScopeNodeTypes() []string {
	return []string{"function_definition", "subshell", "compound_statement"}
}

// ClassBodyNodeTypes returns empty — Bash has no classes.
func (p *BashPlugin) ClassBodyNodeTypes() []string {
	return []string{}
}

// ClassDeclNodeTypes returns empty — Bash has no classes.
func (p *BashPlugin) ClassDeclNodeTypes() []string {
	return []string{}
}

// ExtractMetadata extracts Bash-specific metadata (minimal — no modifiers in Bash).
func (p *BashPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	data, err := json.Marshal(BashMetadata{})
	if err != nil {
		return "{}", fmt.Errorf("failed to marshal Bash metadata: %w", err)
	}
	return string(data), nil
}

// ExtractEdge extracts edges from a single Bash tree-sitter pattern match.
//
// Pattern indices map to the order of patterns in queries/bash/edges.scm:
// 0 = source/. import, 1 = direct command call
func (p *BashPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
	var edges []PluginEdge
	fromFunction, _ := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// 0: source ./file.sh or . ./file.sh — import
	case 0:
		cmd, hasCmd := captures["_cmd"]
		source, hasSource := captures["source"]
		if hasCmd && hasSource && (cmd.Text == "source" || cmd.Text == ".") {
			edges = append(edges, NewEdge(
				fmt.Sprintf("%s::__module__::function", filePath),
				source.Text,
				"imports",
				filePath,
				source.Line,
			))
		}

	// 1: Direct command call (function call)
	case 1:
		if callee, ok := captures["callee"]; ok {
			if isShellBuiltin(callee.Text) {
				break
			}
			edges = append(edges, NewEdge(
				fromFunction,
				callee.Text,
				"calls",
				filePath,
				callee.Line,
			))
		}
	}

	return edges
}

// ExtractDocstring extracts Bash comments (# lines) preceding definitions.
func (p *BashPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	return DefaultExtractDocstring(node, source)
}

// isShellBuiltin returns true for shell built-in commands that should not
// generate edges.
func isShellBuiltin(name string) bool {
	switch name {
	case "echo", "printf", "cd", "pwd", "exit", "return",
		"export", "unset", "local", "declare", "readonly",
		"shift", "read", "test", "true", "false",
		"set", "eval", "exec", "trap", "wait",
		"kill", "bg", "fg", "jobs",
		"pushd", "popd", "dirs",
		"source", ".", "break", "continue",
		"alias", "unalias", "type", "which",
		"getopts", "let", "sleep":
		return true
	default:
		return false
	}
}
