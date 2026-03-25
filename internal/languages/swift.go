// Swift-specific metadata extraction and language plugin.
//
// Extracts classes, structs, enums, protocols, functions, methods, and
// properties from Swift AST nodes. Swift's tree-sitter grammar uses
// class_declaration for class, struct, and enum (distinguished by first
// child keyword). Modifiers are real AST nodes under a modifiers parent.
//
// InferSymbolKind returns "class" for all class_declaration nodes since
// it only receives the node type string, not the node itself. The actual
// keyword (class/struct/enum) is stored in metadata.
package languages

import (
	"encoding/json"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/swift"

	"github.com/KilimcininKorOglu/inari/queries"
)

// SwiftPlugin implements LanguagePlugin for Swift (.swift) files.
type SwiftPlugin struct{}

// SwiftMetadata holds structured metadata for a Swift symbol.
type SwiftMetadata struct {
	Access     string               `json:"access"`
	IsStatic   bool                 `json:"isStatic"`
	IsLet      bool                 `json:"isLet"`
	Keyword    string               `json:"keyword,omitempty"`
	ReturnType *string              `json:"returnType,omitempty"`
	Parameters []SwiftParameterInfo `json:"parameters"`
}

// SwiftParameterInfo holds information about a single Swift function parameter.
type SwiftParameterInfo struct {
	Name           string  `json:"name"`
	TypeAnnotation *string `json:"type,omitempty"`
}

// Language returns Swift.
func (p *SwiftPlugin) Language() SupportedLanguage {
	return Swift
}

// Extensions returns ["swift"].
func (p *SwiftPlugin) Extensions() []string {
	return []string{"swift"}
}

// TSLanguage returns the tree-sitter Swift grammar.
func (p *SwiftPlugin) TSLanguage() *sitter.Language {
	return swift.GetLanguage()
}

// SymbolQuerySource returns the embedded Swift symbols.scm query.
func (p *SwiftPlugin) SymbolQuerySource() string {
	return queries.SwiftSymbolsQuery
}

// EdgeQuerySource returns the embedded Swift edges.scm query.
func (p *SwiftPlugin) EdgeQuerySource() string {
	return queries.SwiftEdgesQuery
}

// InferSymbolKind maps Swift tree-sitter node types to Inari symbol kinds.
// Note: class_declaration is used for class, struct, and enum — all return "class"
// since InferSymbolKind only receives the node type string. The actual keyword
// is stored in metadata.
func (p *SwiftPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "class_declaration":
		return "class"
	case "protocol_declaration":
		return "interface"
	case "function_declaration":
		return "function"
	case "protocol_function_declaration":
		return "method"
	case "property_declaration":
		return "property"
	default:
		return "function"
	}
}

// ScopeNodeTypes returns node types that define scope boundaries in Swift.
func (p *SwiftPlugin) ScopeNodeTypes() []string {
	return []string{
		"class_declaration",
		"protocol_declaration",
		"function_declaration",
		"init_declaration",
		"protocol_function_declaration",
	}
}

// ClassBodyNodeTypes returns node types for class body containers in Swift.
func (p *SwiftPlugin) ClassBodyNodeTypes() []string {
	return []string{"class_body", "protocol_body", "enum_class_body"}
}

// ClassDeclNodeTypes returns node types for class-like declarations in Swift.
func (p *SwiftPlugin) ClassDeclNodeTypes() []string {
	return []string{"class_declaration", "protocol_declaration"}
}

// ExtractMetadata extracts Swift-specific metadata from a symbol node as a JSON string.
func (p *SwiftPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := SwiftMetadata{
		Access:     "internal",
		Parameters: []SwiftParameterInfo{},
	}

	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "modifiers":
			extractSwiftModifiers(child, source, &meta)
		case "class":
			meta.Keyword = "class"
		case "struct":
			meta.Keyword = "struct"
		case "enum":
			meta.Keyword = "enum"
		case "value_binding_pattern":
			// Check let vs var for properties.
			innerCount := int(child.ChildCount())
			for j := 0; j < innerCount; j++ {
				inner := child.Child(j)
				if inner != nil && inner.Type() == "let" {
					meta.IsLet = true
				}
			}
		case "parameter":
			info := extractSwiftParameter(child, source)
			if info.Name != "" {
				meta.Parameters = append(meta.Parameters, info)
			}
		}
	}

	// Extract return type (-> Type).
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "->" {
			// Next sibling is the return type.
			if i+1 < childCount {
				next := node.Child(i + 1)
				if next != nil {
					text := strings.TrimSpace(next.Content(source))
					if text != "" {
						meta.ReturnType = &text
					}
				}
			}
			break
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "{}", fmt.Errorf("failed to marshal Swift metadata: %w", err)
	}
	return string(data), nil
}

// ExtractEdge extracts edges from a single Swift tree-sitter pattern match.
//
// Pattern indices map to the order of patterns in queries/swift/edges.scm:
// 0 = import, 1 = method/static call, 2 = direct/constructor call, 3 = inheritance
func (p *SwiftPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
	var edges []PluginEdge
	fromFunction, fromClass := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// 0: import Foundation
	case 0:
		if importedName, ok := captures["imported_name"]; ok {
			edges = append(edges, NewEdge(
				fmt.Sprintf("%s::__module__::function", filePath),
				importedName.Text,
				"imports",
				filePath,
				importedName.Line,
			))
		}

	// 1: service.processPayment() or PaymentService.createDefault()
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

	// 2: PaymentService(...) or directFunc()
	case 2:
		if callee, ok := captures["callee"]; ok {
			if isSwiftBuiltinCall(callee.Text) {
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

	// 3: : BaseService, PaymentInterface — inheritance
	case 3:
		if parentType, ok := captures["parent_type"]; ok {
			edges = append(edges, NewEdge(
				fromClass,
				parentType.Text,
				"implements",
				filePath,
				parentType.Line,
			))
		}
	}

	return edges
}

// ExtractDocstring extracts Swift doc comments (/// or /** */) preceding declarations.
func (p *SwiftPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	return DefaultExtractDocstring(node, source)
}

// isSwiftBuiltinCall returns true for Swift built-in functions/types that
// should not generate edges.
func isSwiftBuiltinCall(name string) bool {
	switch name {
	case "print", "debugPrint", "fatalError", "precondition",
		"preconditionFailure", "assert", "assertionFailure",
		"type", "dump", "abs", "min", "max",
		"stride", "zip", "map", "filter", "reduce",
		"Array", "Dictionary", "Set", "String", "Int", "Double", "Bool", "Float",
		"Optional", "Result":
		return true
	default:
		return false
	}
}

// extractSwiftModifiers walks the modifiers node and populates metadata fields.
func extractSwiftModifiers(modifiersNode *sitter.Node, source []byte, meta *SwiftMetadata) {
	childCount := int(modifiersNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := modifiersNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "visibility_modifier":
			text := child.Content(source)
			switch text {
			case "public":
				meta.Access = "public"
			case "private":
				meta.Access = "private"
			case "internal":
				meta.Access = "internal"
			case "fileprivate":
				meta.Access = "fileprivate"
			case "open":
				meta.Access = "open"
			}
		case "property_modifier":
			text := child.Content(source)
			if text == "static" || text == "class" {
				meta.IsStatic = true
			}
		}
	}
}

// extractSwiftParameter extracts parameter info from a parameter node.
func extractSwiftParameter(paramNode *sitter.Node, source []byte) SwiftParameterInfo {
	info := SwiftParameterInfo{}

	// First simple_identifier is the parameter name.
	childCount := int(paramNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramNode.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "simple_identifier" && info.Name == "" {
			info.Name = child.Content(source)
		}
		if child.Type() == "user_type" || child.Type() == "type_identifier" {
			text := strings.TrimSpace(child.Content(source))
			if text != "" {
				info.TypeAnnotation = &text
			}
		}
	}

	return info
}
