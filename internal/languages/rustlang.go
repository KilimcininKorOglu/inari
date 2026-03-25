// Rust-specific metadata extraction and language plugin.
//
// Extracts visibility modifiers (pub, pub(crate), pub(super), private),
// Rust-specific modifiers (async, const, unsafe), attributes, return type,
// and parameters from Rust AST nodes.
package languages

import (
	"encoding/json"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/rust"

	"github.com/KilimcininKorOglu/inari/queries"
)

// RustPlugin implements LanguagePlugin for Rust (.rs) files.
type RustPlugin struct{}

// RustMetadata holds structured metadata for a Rust symbol.
type RustMetadata struct {
	Visibility string              `json:"visibility"`
	IsAsync    bool                `json:"is_async"`
	IsConst    bool                `json:"is_const"`
	IsUnsafe   bool                `json:"is_unsafe"`
	Attributes []string            `json:"attributes"`
	ReturnType *string             `json:"return_type"`
	Parameters []RustParameterInfo `json:"parameters"`
}

// RustParameterInfo holds information about a single Rust function/method parameter.
type RustParameterInfo struct {
	Name           string  `json:"name"`
	TypeAnnotation *string `json:"type"`
	IsMutable      bool    `json:"is_mutable"`
}

// Language returns Rust.
func (p *RustPlugin) Language() SupportedLanguage {
	return Rust
}

// Extensions returns ["rs"].
func (p *RustPlugin) Extensions() []string {
	return []string{"rs"}
}

// TSLanguage returns the tree-sitter Rust grammar.
func (p *RustPlugin) TSLanguage() *sitter.Language {
	return rust.GetLanguage()
}

// SymbolQuerySource returns the Rust symbols.scm query text.
func (p *RustPlugin) SymbolQuerySource() string {
	return queries.RustSymbolsQuery
}

// EdgeQuerySource returns the Rust edges.scm query text.
func (p *RustPlugin) EdgeQuerySource() string {
	return queries.RustEdgesQuery
}

// InferSymbolKind maps a tree-sitter node kind to an Inari symbol kind.
func (p *RustPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "function_item":
		return "function"
	case "struct_item":
		return "struct"
	case "enum_item":
		return "enum"
	case "trait_item":
		return "interface"
	case "type_item":
		return "type"
	case "const_item", "static_item":
		return "const"
	default:
		return "function"
	}
}

// ScopeNodeTypes returns node types that constitute a scope boundary.
func (p *RustPlugin) ScopeNodeTypes() []string {
	return []string{"function_item", "impl_item", "trait_item", "mod_item"}
}

// ClassBodyNodeTypes returns an empty slice.
// Rust impl blocks contain a declaration_list body, but impl_item is not
// stored as a symbol. The standard findParentClass would generate a
// parent_id referencing a non-existent symbol, causing FK constraint errors.
func (p *RustPlugin) ClassBodyNodeTypes() []string {
	return []string{}
}

// ClassDeclNodeTypes returns an empty slice.
// Avoids FK constraint errors since Rust does not use class declarations.
func (p *RustPlugin) ClassDeclNodeTypes() []string {
	return []string{}
}

// ExtractMetadata extracts Rust-specific metadata from a symbol node as a JSON string.
func (p *RustPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	return extractRustMetadata(node, source, kind)
}

// ExtractEdge extracts edges from a single Rust query pattern match.
//
// Pattern indices map to the order of patterns in queries/rust/edges.scm:
// 0 = use scoped, 1 = use aliased, 2 = direct call, 3 = scoped call,
// 4 = method call, 5 = macro invocation, 6 = scoped macro,
// 7 = field type ref, 8 = param type ref, 9 = return type ref
func (p *RustPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
	var edges []PluginEdge
	fromFunction, _ := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// Use declaration — scoped identifier (e.g. use std::io)
	case 0:
		if importedName, ok := captures["imported_name"]; ok {
			line := importedName.Line
			edges = append(edges, PluginEdge{
				FromID:   fmt.Sprintf("%s::__module__::function", filePath),
				ToID:     importedName.Text,
				Kind:     "imports",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Use declaration — aliased (use ... as ...)
	case 1:
		if importedName, ok := captures["imported_name"]; ok {
			line := importedName.Line
			edges = append(edges, PluginEdge{
				FromID:   fmt.Sprintf("%s::__module__::function", filePath),
				ToID:     importedName.Text,
				Kind:     "imports",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Direct call expression (e.g. process_payment(...))
	case 2:
		if callee, ok := captures["callee"]; ok {
			line := callee.Line
			edges = append(edges, PluginEdge{
				FromID:   fromFunction,
				ToID:     callee.Text,
				Kind:     "calls",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Scoped call expression (e.g. PaymentService::new(...))
	case 3:
		if callee, ok := captures["callee"]; ok {
			line := callee.Line
			edges = append(edges, PluginEdge{
				FromID:   fromFunction,
				ToID:     callee.Text,
				Kind:     "calls",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Method call expression (e.g. self.client.charge(...))
	case 4:
		if method, ok := captures["method"]; ok {
			line := method.Line
			edges = append(edges, PluginEdge{
				FromID:   fromFunction,
				ToID:     method.Text,
				Kind:     "calls",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Macro invocation (e.g. println!(...))
	case 5:
		if macroName, ok := captures["macro_name"]; ok {
			line := macroName.Line
			edges = append(edges, PluginEdge{
				FromID:   fromFunction,
				ToID:     fmt.Sprintf("%s!", macroName.Text),
				Kind:     "calls",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Scoped macro invocation (e.g. std::println!(...))
	case 6:
		if macroName, ok := captures["macro_name"]; ok {
			line := macroName.Line
			edges = append(edges, PluginEdge{
				FromID:   fromFunction,
				ToID:     fmt.Sprintf("%s!", macroName.Text),
				Kind:     "calls",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Field type reference
	case 7:
		if typeRef, ok := captures["type_ref"]; ok {
			line := typeRef.Line
			edges = append(edges, PluginEdge{
				FromID:   fromFunction,
				ToID:     typeRef.Text,
				Kind:     "references_type",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Parameter type reference
	case 8:
		if typeRef, ok := captures["type_ref"]; ok {
			line := typeRef.Line
			edges = append(edges, PluginEdge{
				FromID:   fromFunction,
				ToID:     typeRef.Text,
				Kind:     "references_type",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Return type reference
	case 9:
		if typeRef, ok := captures["type_ref"]; ok {
			line := typeRef.Line
			edges = append(edges, PluginEdge{
				FromID:   fromFunction,
				ToID:     typeRef.Text,
				Kind:     "references_type",
				FilePath: filePath,
				Line:     &line,
			})
		}
	}

	return edges
}

// ExtractDocstring extracts doc comments from Rust AST nodes.
// Rust's tree-sitter grammar uses "line_comment" and "block_comment" node types
// instead of the generic "comment" that DefaultExtractDocstring checks.
func (p *RustPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	prev := node.PrevSibling()
	if prev == nil {
		return ""
	}
	nodeType := prev.Type()
	if nodeType == "line_comment" || nodeType == "block_comment" || nodeType == "comment" {
		text := prev.Content(source)
		if text != "" {
			return trimDocComment(text)
		}
	}
	return ""
}

// trimDocComment strips Rust doc comment prefixes (///, //!, /**, etc.) and trims whitespace.
func trimDocComment(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "///") {
		s = strings.TrimPrefix(s, "///")
	} else if strings.HasPrefix(s, "//!") {
		s = strings.TrimPrefix(s, "//!")
	} else if strings.HasPrefix(s, "/**") && strings.HasSuffix(s, "*/") {
		s = strings.TrimPrefix(s, "/**")
		s = strings.TrimSuffix(s, "*/")
	} else if strings.HasPrefix(s, "//") {
		s = strings.TrimPrefix(s, "//")
	}
	return strings.TrimSpace(s)
}

// extractRustMetadata extracts metadata from a Rust AST node.
// Returns a JSON string suitable for the metadata column.
func extractRustMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := RustMetadata{
		Visibility: "private",
		Attributes: []string{},
		Parameters: []RustParameterInfo{},
	}

	// Walk direct children to find modifiers and attributes.
	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "visibility_modifier":
			text := child.Content(source)
			if text != "" {
				trimmed := strings.TrimSpace(text)
				switch {
				case trimmed == "pub":
					meta.Visibility = "pub"
				case strings.HasPrefix(trimmed, "pub(crate)"):
					meta.Visibility = "pub(crate)"
				case strings.HasPrefix(trimmed, "pub(super)"):
					meta.Visibility = "pub(super)"
				case strings.HasPrefix(trimmed, "pub"):
					meta.Visibility = trimmed
				default:
					meta.Visibility = "private"
				}
			}
		case "attribute_item":
			text := child.Content(source)
			if text != "" {
				meta.Attributes = append(meta.Attributes, strings.TrimSpace(text))
			}
		}
	}

	// Check for async, const, unsafe keywords in function items.
	if kind == "function" || kind == "method" {
		for i := 0; i < childCount; i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			text := child.Content(source)
			switch text {
			case "async":
				meta.IsAsync = true
			case "const":
				meta.IsConst = true
			case "unsafe":
				meta.IsUnsafe = true
			}
		}

		// Extract return type.
		if returnNode := node.ChildByFieldName("return_type"); returnNode != nil {
			text := returnNode.Content(source)
			if text != "" {
				// Strip the leading `-> ` from return types.
				clean := strings.TrimSpace(strings.TrimPrefix(text, "->"))
				if clean != "" {
					meta.ReturnType = &clean
				}
			}
		}

		// Extract parameters.
		if paramsNode := node.ChildByFieldName("parameters"); paramsNode != nil {
			meta.Parameters = extractRustParameters(paramsNode, source)
		}
	}

	// For const/static items, mark is_const.
	if kind == "const" {
		meta.IsConst = true
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("failed to serialize Rust metadata: %w", err)
	}
	return string(data), nil
}

// extractRustParameters extracts parameter info from a parameters node.
// Handles parameter (with mut check) and self_parameter nodes.
func extractRustParameters(paramsNode *sitter.Node, source []byte) []RustParameterInfo {
	var params []RustParameterInfo

	childCount := int(paramsNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "parameter":
			var name string
			var typeAnnotation *string
			isMutable := false

			// Extract pattern (name) and type.
			if patternNode := child.ChildByFieldName("pattern"); patternNode != nil {
				text := patternNode.Content(source)
				if text != "" {
					trimmed := strings.TrimSpace(text)
					if strings.HasPrefix(trimmed, "mut ") {
						name = strings.TrimPrefix(trimmed, "mut ")
						isMutable = true
					} else {
						name = trimmed
					}
				}
			}

			if typeNode := child.ChildByFieldName("type"); typeNode != nil {
				text := typeNode.Content(source)
				if text != "" {
					clean := strings.TrimSpace(text)
					typeAnnotation = &clean
				}
			}

			if name != "" {
				params = append(params, RustParameterInfo{
					Name:           name,
					TypeAnnotation: typeAnnotation,
					IsMutable:      isMutable,
				})
			}

		case "self_parameter":
			text := child.Content(source)
			if text != "" {
				trimmed := strings.TrimSpace(text)
				isMutable := strings.Contains(trimmed, "mut")
				params = append(params, RustParameterInfo{
					Name:      "self",
					IsMutable: isMutable,
				})
			}
		}
	}

	return params
}
