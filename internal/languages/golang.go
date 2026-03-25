// Go-specific metadata extraction and language plugin.
//
// Extracts exported/unexported status (capitalization convention), method
// receivers, parameters, and return types from Go AST nodes.
package languages

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"

	"github.com/KilimcininKorOglu/inari/queries"
)

// GoPlugin implements LanguagePlugin for Go (.go) files.
type GoPlugin struct{}

// GoMetadata holds structured metadata for a Go symbol.
type GoMetadata struct {
	Exported   bool              `json:"exported"`
	IsMethod   bool              `json:"is_method"`
	Receiver   *string           `json:"receiver,omitempty"`
	Parameters []GoParameterInfo `json:"parameters"`
	ReturnType *string           `json:"return_type,omitempty"`
}

// GoParameterInfo holds information about a single Go function/method parameter.
type GoParameterInfo struct {
	Name           string  `json:"name"`
	TypeAnnotation *string `json:"type"`
}

// Language returns the Go language identifier.
func (p *GoPlugin) Language() SupportedLanguage {
	return Go
}

// Extensions returns the file extensions handled by this plugin.
func (p *GoPlugin) Extensions() []string {
	return []string{"go"}
}

// TSLanguage returns the tree-sitter Go grammar.
func (p *GoPlugin) TSLanguage() *sitter.Language {
	return golang.GetLanguage()
}

// SymbolQuerySource returns the embedded Go symbols.scm query.
func (p *GoPlugin) SymbolQuerySource() string {
	return queries.GoSymbolsQuery
}

// EdgeQuerySource returns the embedded Go edges.scm query.
func (p *GoPlugin) EdgeQuerySource() string {
	return queries.GoEdgesQuery
}

// InferSymbolKind maps Go tree-sitter node types to Inari symbol kinds.
func (p *GoPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "function_declaration":
		return "function"
	case "method_declaration":
		return "method"
	case "type_declaration", "type_spec":
		return "type"
	case "const_declaration", "const_spec":
		return "const"
	default:
		return "function"
	}
}

// ScopeNodeTypes returns node types that define scope boundaries in Go.
func (p *GoPlugin) ScopeNodeTypes() []string {
	return []string{"function_declaration", "method_declaration", "func_literal"}
}

// ClassBodyNodeTypes returns node types for struct/interface body containers.
func (p *GoPlugin) ClassBodyNodeTypes() []string {
	return []string{"field_declaration_list"}
}

// ClassDeclNodeTypes returns node types for type declarations.
func (p *GoPlugin) ClassDeclNodeTypes() []string {
	return []string{"type_declaration"}
}

// ExtractMetadata extracts Go-specific metadata from an AST node.
func (p *GoPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := GoMetadata{
		Parameters: []GoParameterInfo{},
	}

	name := ""
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		name = nameNode.Content(source)
	}

	// Exported = first letter is uppercase.
	if name != "" {
		r, _ := utf8.DecodeRuneInString(name)
		meta.Exported = unicode.IsUpper(r)
	}

	switch node.Type() {
	case "function_declaration":
		meta.Parameters = extractGoParameters(node, source)
		meta.ReturnType = extractGoReturnType(node, source)

	case "method_declaration":
		meta.IsMethod = true
		meta.Parameters = extractGoParameters(node, source)
		meta.ReturnType = extractGoReturnType(node, source)

		// Extract receiver type.
		recvNode := node.ChildByFieldName("receiver")
		if recvNode != nil {
			recvText := strings.TrimSpace(recvNode.Content(source))
			recvText = strings.TrimPrefix(recvText, "(")
			recvText = strings.TrimSuffix(recvText, ")")
			// Extract type from "varName *Type" or "*Type"
			parts := strings.Fields(recvText)
			if len(parts) > 0 {
				recvType := parts[len(parts)-1]
				meta.Receiver = &recvType
			}
		}

	case "type_declaration", "type_spec":
		// Check if it's a struct or interface for refined kind.
		typeNode := node.ChildByFieldName("type")
		if typeNode == nil {
			// type_declaration wraps type_spec; look inside.
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child != nil && child.Type() == "type_spec" {
					typeNode = child.ChildByFieldName("type")
					if nameNode == nil {
						nameNode = child.ChildByFieldName("name")
						if nameNode != nil {
							name = nameNode.Content(source)
							r, _ := utf8.DecodeRuneInString(name)
							meta.Exported = unicode.IsUpper(r)
						}
					}
					break
				}
			}
		}
		if typeNode != nil {
			switch typeNode.Type() {
			case "struct_type":
				// kind will be refined by the caller
			case "interface_type":
				// kind will be refined by the caller
			}
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "{}", fmt.Errorf("failed to marshal Go metadata: %w", err)
	}
	return string(data), nil
}

// ExtractEdge extracts edges from a single tree-sitter pattern match.
func (p *GoPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
	var edges []PluginEdge
	fromFunction, _ := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// 0: Import
	case 0:
		if importedName, ok := captures["imported_name"]; ok {
			// Strip quotes from import path.
			importPath := strings.Trim(importedName.Text, "\"")
			line := importedName.Line
			edges = append(edges, PluginEdge{
				FromID:   fmt.Sprintf("%s::__module__::function", filePath),
				ToID:     importPath,
				Kind:     "imports",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// 1: Function call
	case 1:
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

	// 2: Method / qualified call
	case 2:
		object, hasObject := captures["object"]
		method, hasMethod := captures["method"]
		if hasObject && hasMethod {
			line := object.Line
			edges = append(edges, PluginEdge{
				FromID:   fromFunction,
				ToID:     fmt.Sprintf("%s.%s", object.Text, method.Text),
				Kind:     "calls",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// 3: Composite literal (struct instantiation)
	case 3:
		if className, ok := captures["class_name"]; ok {
			line := className.Line
			edges = append(edges, PluginEdge{
				FromID:   fromFunction,
				ToID:     className.Text,
				Kind:     "instantiates",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// 4: Type reference in field
	case 4:
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

// ExtractDocstring uses the default docstring extraction.
// Go's tree-sitter grammar uses "comment" node type, matching the default.
func (p *GoPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	return DefaultExtractDocstring(node, source)
}

// extractGoParameters extracts parameter information from a Go function/method node.
func extractGoParameters(node *sitter.Node, source []byte) []GoParameterInfo {
	var params []GoParameterInfo

	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode == nil {
		return params
	}

	childCount := int(paramsNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "parameter_declaration":
			nameNode := child.ChildByFieldName("name")
			typeNode := child.ChildByFieldName("type")

			name := ""
			if nameNode != nil {
				name = nameNode.Content(source)
			}

			var typeStr *string
			if typeNode != nil {
				t := typeNode.Content(source)
				typeStr = &t
			}

			if name != "" {
				params = append(params, GoParameterInfo{
					Name:           name,
					TypeAnnotation: typeStr,
				})
			}

		case "variadic_parameter_declaration":
			nameNode := child.ChildByFieldName("name")
			typeNode := child.ChildByFieldName("type")

			name := ""
			if nameNode != nil {
				name = nameNode.Content(source)
			}

			var typeStr *string
			if typeNode != nil {
				t := "..." + typeNode.Content(source)
				typeStr = &t
			}

			if name != "" {
				params = append(params, GoParameterInfo{
					Name:           name,
					TypeAnnotation: typeStr,
				})
			}
		}
	}

	return params
}

// extractGoReturnType extracts the return type from a Go function/method node.
func extractGoReturnType(node *sitter.Node, source []byte) *string {
	resultNode := node.ChildByFieldName("result")
	if resultNode == nil {
		return nil
	}
	text := strings.TrimSpace(resultNode.Content(source))
	if text == "" {
		return nil
	}
	return &text
}
