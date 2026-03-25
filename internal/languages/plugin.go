// Package languages defines the LanguagePlugin interface and shared types
// used by language-specific extraction logic.
//
// Each language module provides metadata extraction functions that
// understand language-specific modifiers, access levels, and conventions.
//
// The LanguagePlugin interface allows adding new language support without
// modifying parser.go — implement the interface and register the plugin.
package languages

import (
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
)

// SupportedLanguage represents a programming language that Inari can parse.
type SupportedLanguage int

const (
	// TypeScript represents .ts and .tsx files.
	TypeScript SupportedLanguage = iota
	// CSharp represents .cs files.
	CSharp
	// Python represents .py files.
	Python
	// Go represents .go files.
	Go
	// Java represents .java files.
	Java
	// Kotlin represents .kt files.
	Kotlin
	// Rust represents .rs files.
	Rust
	// Ruby represents .rb files.
	Ruby
	// Php represents .php files.
	Php
	// Lua represents .lua files.
	Lua
	// Swift represents .swift files.
	Swift
	// Bash represents .sh and .bash files.
	Bash
)

// AsStr returns the language name as a lowercase identifier string.
func (l SupportedLanguage) AsStr() string {
	switch l {
	case TypeScript:
		return "typescript"
	case CSharp:
		return "csharp"
	case Python:
		return "python"
	case Go:
		return "go"
	case Java:
		return "java"
	case Kotlin:
		return "kotlin"
	case Rust:
		return "rust"
	case Ruby:
		return "ruby"
	case Php:
		return "php"
	case Lua:
		return "lua"
	case Swift:
		return "swift"
	case Bash:
		return "bash"
	default:
		return "unknown"
	}
}

// String returns the display name of the language.
func (l SupportedLanguage) String() string {
	switch l {
	case TypeScript:
		return "TypeScript"
	case CSharp:
		return "C#"
	case Python:
		return "Python"
	case Go:
		return "Go"
	case Java:
		return "Java"
	case Kotlin:
		return "Kotlin"
	case Rust:
		return "Rust"
	case Ruby:
		return "Ruby"
	case Php:
		return "PHP"
	case Lua:
		return "Lua"
	case Swift:
		return "Swift"
	case Bash:
		return "Bash"
	default:
		return fmt.Sprintf("SupportedLanguage(%d)", int(l))
	}
}

// CaptureData holds the text and line number extracted from a tree-sitter query capture.
type CaptureData struct {
	// Text is the captured source text.
	Text string
	// Line is the 1-based line number where the capture starts.
	Line uint32
}

// RawEdge holds raw edge data returned by language plugins before scope resolution.
type RawEdge struct {
	// Kind is the edge kind (e.g. "calls", "imports", "extends").
	Kind string
	// Target is the target symbol identifier.
	Target string
	// Line is the 1-based source line number.
	Line uint32
}

// PluginEdge represents an edge extracted by a language plugin.
// Defined here to avoid circular imports between languages and core packages.
// The parser converts these to core.Edge after extraction.
type PluginEdge struct {
	// FromID is the source symbol ID.
	FromID string
	// ToID is the target symbol ID (may reference external symbols not in the index).
	ToID string
	// Kind is the edge kind: calls, imports, extends, implements, instantiates, references, references_type.
	Kind string
	// FilePath is the file where this edge was observed.
	FilePath string
	// Line is the 1-based line number where the edge was observed (nil for file-level edges).
	Line *uint32
}

// ResolveEdgeScope resolves the from_id for function-level and class-level edges.
// If enclosingScopeID is empty, falls back to a synthetic __module__ ID.
// This is shared boilerplate used by all language plugins' ExtractEdge methods.
func ResolveEdgeScope(enclosingScopeID, filePath string) (fromFunction, fromClass string) {
	fromFunction = enclosingScopeID
	if fromFunction == "" {
		fromFunction = fmt.Sprintf("%s::__module__::function", filePath)
	}
	fromClass = enclosingScopeID
	if fromClass == "" {
		fromClass = fmt.Sprintf("%s::__module__::class", filePath)
	}
	return
}

// NewEdge is a convenience constructor for PluginEdge with a line number.
func NewEdge(fromID, toID, kind, filePath string, line uint32) PluginEdge {
	return PluginEdge{FromID: fromID, ToID: toID, Kind: kind, FilePath: filePath, Line: &line}
}

// LanguagePlugin defines the interface that each language plugin must implement.
//
// Adding a new language means implementing this interface and registering
// the plugin via CodeParser.RegisterPlugin — parser.go never changes.
type LanguagePlugin interface {
	// Language returns which language this plugin handles.
	Language() SupportedLanguage

	// Extensions returns file extensions this language matches (e.g., ["ts", "tsx"]).
	Extensions() []string

	// TSLanguage returns the tree-sitter Language grammar.
	TSLanguage() *sitter.Language

	// SymbolQuerySource returns the source text of the symbols.scm query.
	SymbolQuerySource() string

	// EdgeQuerySource returns the source text of the edges.scm query.
	EdgeQuerySource() string

	// InferSymbolKind maps a tree-sitter node kind to an Inari symbol kind.
	//
	// For example, "function_declaration" maps to "function",
	// "class_declaration" maps to "class".
	InferSymbolKind(nodeKind string) string

	// ScopeNodeTypes returns node types that constitute a scope boundary.
	ScopeNodeTypes() []string

	// ClassBodyNodeTypes returns node types for class body nodes (used in findParentClass).
	ClassBodyNodeTypes() []string

	// ClassDeclNodeTypes returns node types for class declaration nodes (used in findParentClass).
	ClassDeclNodeTypes() []string

	// ExtractMetadata extracts language-specific metadata from a symbol node as a JSON string.
	ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error)

	// ExtractEdge extracts edges from a single query pattern match.
	//
	// captures maps capture names to CaptureData (text, line) pairs.
	ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge

	// ExtractDocstring extracts a docstring from the definition node.
	//
	// Language plugins can override this for languages where docstrings
	// are string literals inside the body (e.g., Python).
	ExtractDocstring(node *sitter.Node, source []byte) string
}

// DefaultExtractDocstring provides the default docstring extraction logic.
// It looks for a comment node as the previous sibling of the given node.
// Works for TypeScript, C#, Rust, and similar languages where doc comments
// precede the definition.
func DefaultExtractDocstring(node *sitter.Node, source []byte) string {
	prev := node.PrevSibling()
	if prev == nil {
		return ""
	}
	if prev.Type() == "comment" {
		text := prev.Content(source)
		if text != "" {
			return trimString(text)
		}
	}
	return ""
}

// trimString trims leading and trailing whitespace from a string.
func trimString(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
