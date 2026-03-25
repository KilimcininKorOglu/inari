// Lua-specific metadata extraction and language plugin.
//
// Extracts global and local function definitions, table-based OOP methods
// (function Foo:bar() and function Foo.bar()), require() imports, colon
// method calls, and dot function calls.
//
// Lua has no class system — OOP is done via tables and metatables.
// All function definitions are mapped to "function" kind (like Python).
// The Munif Tanjim tree-sitter grammar is used, which has different node
// types than the standard tree-sitter-lua (e.g. function_statement instead
// of function_declaration).
package languages

import (
	"encoding/json"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/lua"

	"github.com/KilimcininKorOglu/inari/queries"
)

// LuaPlugin implements LanguagePlugin for Lua (.lua) files.
type LuaPlugin struct{}

// LuaMetadata holds structured metadata for a Lua symbol.
type LuaMetadata struct {
	IsLocal    bool               `json:"isLocal"`
	Parameters []LuaParameterInfo `json:"parameters"`
}

// LuaParameterInfo holds information about a single Lua function parameter.
type LuaParameterInfo struct {
	Name      string `json:"name"`
	IsVarargs bool   `json:"isVarargs"`
}

// Language returns Lua.
func (p *LuaPlugin) Language() SupportedLanguage {
	return Lua
}

// Extensions returns ["lua"].
func (p *LuaPlugin) Extensions() []string {
	return []string{"lua"}
}

// TSLanguage returns the tree-sitter Lua grammar.
func (p *LuaPlugin) TSLanguage() *sitter.Language {
	return lua.GetLanguage()
}

// SymbolQuerySource returns the embedded Lua symbols.scm query.
func (p *LuaPlugin) SymbolQuerySource() string {
	return queries.LuaSymbolsQuery
}

// EdgeQuerySource returns the embedded Lua edges.scm query.
func (p *LuaPlugin) EdgeQuerySource() string {
	return queries.LuaEdgesQuery
}

// InferSymbolKind maps Lua tree-sitter node types to Inari symbol kinds.
// Lua has no classes — all definitions are "function" kind.
func (p *LuaPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "function_statement":
		return "function"
	default:
		return "function"
	}
}

// ScopeNodeTypes returns node types that define scope boundaries in Lua.
func (p *LuaPlugin) ScopeNodeTypes() []string {
	return []string{
		"function_statement",
		"function_body",
		"do_statement",
		"for_statement",
		"while_statement",
		"if_statement",
	}
}

// ClassBodyNodeTypes returns empty — Lua has no classes.
func (p *LuaPlugin) ClassBodyNodeTypes() []string {
	return []string{}
}

// ClassDeclNodeTypes returns empty — Lua has no classes.
func (p *LuaPlugin) ClassDeclNodeTypes() []string {
	return []string{}
}

// ExtractMetadata extracts Lua-specific metadata from a symbol node as a JSON string.
func (p *LuaPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := LuaMetadata{
		Parameters: []LuaParameterInfo{},
	}

	// Detect local functions: function_statement with a "local" child.
	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "local" {
			meta.IsLocal = true
			break
		}
	}

	// Extract parameters from parameter_list.
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "parameter_list" {
			meta.Parameters = extractLuaParameters(child, source)
			break
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "{}", fmt.Errorf("failed to marshal Lua metadata: %w", err)
	}
	return string(data), nil
}

// ExtractEdge extracts edges from a single Lua tree-sitter pattern match.
//
// Pattern indices map to the order of patterns in queries/lua/edges.scm:
// 0 = require (import), 1 = colon method call, 2 = dot call, 3 = direct call
func (p *LuaPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
	var edges []PluginEdge
	fromFunction, _ := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// 0: require("module") — import
	case 0:
		callee, hasCallee := captures["callee"]
		source, hasSource := captures["source"]
		if hasCallee && hasSource && callee.Text == "require" {
			// Strip surrounding quotes from the string node text.
			modName := strings.Trim(source.Text, "\"'")
			if modName != "" {
				edges = append(edges, NewEdge(
					fmt.Sprintf("%s::__module__::function", filePath),
					modName,
					"imports",
					filePath,
					source.Line,
				))
			}
		}

	// 1: obj:method() — colon method call
	case 1:
		receiver, hasReceiver := captures["receiver"]
		callee, hasCallee := captures["callee"]
		if hasReceiver && hasCallee {
			edges = append(edges, NewEdge(
				fromFunction,
				fmt.Sprintf("%s.%s", receiver.Text, callee.Text),
				"calls",
				filePath,
				receiver.Line,
			))
		}

	// 2: module.func() — dot call
	case 2:
		receiver, hasReceiver := captures["receiver"]
		callee, hasCallee := captures["callee"]
		if hasReceiver && hasCallee {
			edges = append(edges, NewEdge(
				fromFunction,
				fmt.Sprintf("%s.%s", receiver.Text, callee.Text),
				"calls",
				filePath,
				receiver.Line,
			))
		}

	// 3: func() — direct call
	case 3:
		if callee, ok := captures["callee"]; ok {
			if isLuaBuiltinCall(callee.Text) {
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

// ExtractDocstring extracts Lua doc comments (-- lines) preceding definitions.
func (p *LuaPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	return DefaultExtractDocstring(node, source)
}

// isLuaBuiltinCall returns true for Lua built-in functions that should not
// generate edges.
func isLuaBuiltinCall(name string) bool {
	switch name {
	case "print", "error", "assert", "type",
		"tostring", "tonumber", "rawget", "rawset", "rawlen",
		"pairs", "ipairs", "next", "select",
		"pcall", "xpcall",
		"setmetatable", "getmetatable",
		"unpack", "require",
		"collectgarbage", "dofile", "loadfile", "load":
		return true
	default:
		return false
	}
}

// extractLuaParameters extracts parameter info from a parameter_list node.
func extractLuaParameters(paramsNode *sitter.Node, source []byte) []LuaParameterInfo {
	var params []LuaParameterInfo

	childCount := int(paramsNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier":
			name := child.Content(source)
			if name != "" && name != "self" {
				params = append(params, LuaParameterInfo{Name: name})
			}
		case "ellipsis":
			params = append(params, LuaParameterInfo{Name: "...", IsVarargs: true})
		}
	}

	return params
}
