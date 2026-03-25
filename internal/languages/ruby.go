// Ruby-specific metadata extraction and language plugin.
//
// Extracts access level (public, private, protected), singleton methods
// (class-level methods via def self.xxx), attr_accessor/attr_reader/attr_writer
// as property symbols, and method parameters from Ruby AST nodes.
//
// Ruby visibility modifiers (private, protected, public) are method calls in
// tree-sitter, not AST modifiers. Only the inline pattern (private def foo)
// is detected; the block pattern (private followed by methods) is not.
package languages

import (
	"encoding/json"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/ruby"

	"github.com/KilimcininKorOglu/inari/queries"
)

// RubyPlugin implements LanguagePlugin for Ruby (.rb) files.
type RubyPlugin struct{}

// RubyMetadata holds structured metadata for a Ruby symbol.
type RubyMetadata struct {
	Access     string              `json:"access"`
	IsStatic   bool                `json:"isStatic"`
	AttrType   string              `json:"attrType,omitempty"`
	Parameters []RubyParameterInfo `json:"parameters"`
}

// RubyParameterInfo holds information about a single Ruby method parameter.
type RubyParameterInfo struct {
	Name       string `json:"name"`
	HasDefault bool   `json:"hasDefault"`
}

// Language returns Ruby.
func (p *RubyPlugin) Language() SupportedLanguage {
	return Ruby
}

// Extensions returns ["rb"].
func (p *RubyPlugin) Extensions() []string {
	return []string{"rb"}
}

// TSLanguage returns the tree-sitter Ruby grammar.
func (p *RubyPlugin) TSLanguage() *sitter.Language {
	return ruby.GetLanguage()
}

// SymbolQuerySource returns the embedded Ruby symbols.scm query.
func (p *RubyPlugin) SymbolQuerySource() string {
	return queries.RubySymbolsQuery
}

// EdgeQuerySource returns the embedded Ruby edges.scm query.
func (p *RubyPlugin) EdgeQuerySource() string {
	return queries.RubyEdgesQuery
}

// InferSymbolKind maps Ruby tree-sitter node types to Inari symbol kinds.
func (p *RubyPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "class":
		return "class"
	case "module":
		return "module"
	case "method":
		return "method"
	case "singleton_method":
		return "method"
	case "call":
		// attr_accessor/attr_reader/attr_writer captured as call nodes.
		return "property"
	default:
		return "function"
	}
}

// ScopeNodeTypes returns node types that define scope boundaries in Ruby.
func (p *RubyPlugin) ScopeNodeTypes() []string {
	return []string{
		"class",
		"module",
		"method",
		"singleton_method",
		"do_block",
		"block",
	}
}

// ClassBodyNodeTypes returns node types for class body containers in Ruby.
func (p *RubyPlugin) ClassBodyNodeTypes() []string {
	return []string{"body_statement"}
}

// ClassDeclNodeTypes returns node types for class-like declarations in Ruby.
func (p *RubyPlugin) ClassDeclNodeTypes() []string {
	return []string{"class", "module"}
}

// ExtractMetadata extracts Ruby-specific metadata from a symbol node as a JSON string.
func (p *RubyPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := RubyMetadata{
		Access:     "public",
		Parameters: []RubyParameterInfo{},
	}

	// Detect singleton_method (class-level method, e.g. def self.from_config).
	if node.Type() == "singleton_method" {
		meta.IsStatic = true
	}

	// Detect attr type from call node (attr_accessor/attr_reader/attr_writer).
	if node.Type() == "call" {
		if methodNode := node.ChildByFieldName("method"); methodNode != nil {
			methodName := methodNode.Content(source)
			switch methodName {
			case "attr_accessor":
				meta.AttrType = "accessor"
			case "attr_reader":
				meta.AttrType = "reader"
			case "attr_writer":
				meta.AttrType = "writer"
			}
		}
	}

	// Detect private/protected visibility via inline pattern (private def foo).
	// In Ruby's tree-sitter, `private def foo` wraps the method inside a call
	// node whose method is "private"/"protected".
	parent := node.Parent()
	if parent != nil && parent.Type() == "call" {
		if methodNode := parent.ChildByFieldName("method"); methodNode != nil {
			text := methodNode.Content(source)
			if text == "private" {
				meta.Access = "private"
			} else if text == "protected" {
				meta.Access = "protected"
			}
		}
	}

	// Extract parameters for methods.
	if kind == "method" {
		if paramsNode := node.ChildByFieldName("parameters"); paramsNode != nil {
			meta.Parameters = extractRubyParameters(paramsNode, source)
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "{}", fmt.Errorf("failed to marshal Ruby metadata: %w", err)
	}
	return string(data), nil
}

// ExtractEdge extracts edges from a single Ruby tree-sitter pattern match.
//
// Pattern indices map to the order of patterns in queries/ruby/edges.scm:
// 0 = require/require_relative, 1 = class inheritance, 2 = include/extend/prepend,
// 3 = method call with receiver, 4 = direct call, 5 = super call
func (p *RubyPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
	var edges []PluginEdge
	fromFunction, fromClass := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// 0: require / require_relative
	case 0:
		if source, ok := captures["source"]; ok {
			edges = append(edges, NewEdge(
				fmt.Sprintf("%s::__module__::function", filePath),
				source.Text,
				"imports",
				filePath,
				source.Line,
			))
		}

	// 1: Class inheritance (class Foo < Bar)
	case 1:
		if parentClass, ok := captures["parent_class"]; ok {
			edges = append(edges, NewEdge(
				fromClass,
				parentClass.Text,
				"extends",
				filePath,
				parentClass.Line,
			))
		}

	// 2: include/extend/prepend mixin
	case 2:
		mixinMethod, hasMethod := captures["_mixin_method"]
		mixinTarget, hasTarget := captures["mixin_target"]
		if hasTarget {
			kind := "includes"
			if hasMethod && mixinMethod.Text == "extend" {
				kind = "extends"
			}
			edges = append(edges, NewEdge(
				fromClass,
				mixinTarget.Text,
				kind,
				filePath,
				mixinTarget.Line,
			))
		}

	// 3: Method call with receiver (e.g. logger.info("hello"))
	case 3:
		receiver, hasReceiver := captures["receiver"]
		callee, hasCallee := captures["callee"]
		if hasReceiver && hasCallee {
			// Skip visibility/attr/require calls that happen to have a receiver.
			edges = append(edges, NewEdge(
				fromFunction,
				fmt.Sprintf("%s.%s", receiver.Text, callee.Text),
				"calls",
				filePath,
				receiver.Line,
			))
		}

	// 4: Direct call without receiver
	case 4:
		if callee, ok := captures["callee"]; ok {
			// Skip Ruby built-in calls that are not meaningful edges.
			name := callee.Text
			if isRubyBuiltinCall(name) {
				break
			}
			edges = append(edges, NewEdge(
				fromFunction,
				name,
				"calls",
				filePath,
				callee.Line,
			))
		}

	// 5: super call
	case 5:
		if superKw, ok := captures["_super_kw"]; ok {
			edges = append(edges, NewEdge(
				fromFunction,
				"super",
				"calls",
				filePath,
				superKw.Line,
			))
		}
	}

	return edges
}

// ExtractDocstring extracts Ruby doc comments (# lines) preceding definitions.
func (p *RubyPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	return DefaultExtractDocstring(node, source)
}

// isRubyBuiltinCall returns true for Ruby built-in method calls that should not
// generate edges (require, include, attr macros, visibility, IO, etc.).
func isRubyBuiltinCall(name string) bool {
	switch name {
	case "require", "require_relative",
		"include", "extend", "prepend",
		"attr_accessor", "attr_reader", "attr_writer",
		"private", "protected", "public",
		"raise", "puts", "print", "p", "pp",
		"freeze", "dup", "clone":
		return true
	default:
		return false
	}
}

// extractRubyParameters extracts parameter info from a method_parameters node.
func extractRubyParameters(paramsNode *sitter.Node, source []byte) []RubyParameterInfo {
	var params []RubyParameterInfo

	childCount := int(paramsNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		// Regular parameter: bare name
		case "identifier":
			name := child.Content(source)
			if name != "" {
				params = append(params, RubyParameterInfo{Name: name})
			}

		// Optional parameter: name = value
		case "optional_parameter":
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name := nameNode.Content(source)
				if name != "" {
					params = append(params, RubyParameterInfo{Name: name, HasDefault: true})
				}
			}

		// Keyword parameter: name:
		case "keyword_parameter":
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name := nameNode.Content(source)
				if name != "" {
					params = append(params, RubyParameterInfo{Name: name})
				}
			}

		// Splat parameter: *args
		case "splat_parameter":
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name := nameNode.Content(source)
				if name != "" {
					params = append(params, RubyParameterInfo{Name: "*" + name})
				}
			}

		// Hash splat parameter: **kwargs
		case "hash_splat_parameter":
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name := nameNode.Content(source)
				if name != "" {
					params = append(params, RubyParameterInfo{Name: "**" + name})
				}
			}

		// Block parameter: &block
		case "block_parameter":
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name := nameNode.Content(source)
				if name != "" {
					params = append(params, RubyParameterInfo{Name: "&" + name})
				}
			}
		}
	}

	return params
}
