// C-specific metadata extraction and language plugin.
//
// Extracts function definitions, struct/enum declarations, typedef
// definitions, #include imports, and function calls from C source files.
// C is procedural — no classes, methods, or modules.
package languages

import (
	"encoding/json"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"

	"github.com/KilimcininKorOglu/inari/queries"
)

// CLangPlugin implements LanguagePlugin for C (.c, .h) files.
type CLangPlugin struct{}

// CLangMetadata holds structured metadata for a C symbol.
type CLangMetadata struct {
	IsStatic   bool                 `json:"isStatic"`
	ReturnType *string              `json:"returnType,omitempty"`
	Parameters []CLangParameterInfo `json:"parameters"`
}

// CLangParameterInfo holds information about a single C function parameter.
type CLangParameterInfo struct {
	Name string  `json:"name"`
	Type *string `json:"type,omitempty"`
}

// Language returns CLang.
func (p *CLangPlugin) Language() SupportedLanguage {
	return CLang
}

// Extensions returns ["c", "h"].
func (p *CLangPlugin) Extensions() []string {
	return []string{"c", "h"}
}

// TSLanguage returns the tree-sitter C grammar.
func (p *CLangPlugin) TSLanguage() *sitter.Language {
	return c.GetLanguage()
}

// SymbolQuerySource returns the embedded C symbols.scm query.
func (p *CLangPlugin) SymbolQuerySource() string {
	return queries.CSymbolsQuery
}

// EdgeQuerySource returns the embedded C edges.scm query.
func (p *CLangPlugin) EdgeQuerySource() string {
	return queries.CEdgesQuery
}

// InferSymbolKind maps C tree-sitter node types to Inari symbol kinds.
func (p *CLangPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "function_definition":
		return "function"
	case "struct_specifier":
		return "struct"
	case "enum_specifier":
		return "enum"
	case "type_definition":
		return "type"
	default:
		return "function"
	}
}

// ScopeNodeTypes returns node types that define scope boundaries in C.
func (p *CLangPlugin) ScopeNodeTypes() []string {
	return []string{"function_definition"}
}

// ClassBodyNodeTypes returns node types for struct body containers.
func (p *CLangPlugin) ClassBodyNodeTypes() []string {
	return []string{"field_declaration_list"}
}

// ClassDeclNodeTypes returns node types for struct-like declarations.
func (p *CLangPlugin) ClassDeclNodeTypes() []string {
	return []string{"struct_specifier"}
}

// ExtractMetadata extracts C-specific metadata from a symbol node as a JSON string.
func (p *CLangPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := CLangMetadata{
		Parameters: []CLangParameterInfo{},
	}

	// Detect static storage class.
	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "storage_class_specifier" {
			text := child.Content(source)
			if strings.Contains(text, "static") {
				meta.IsStatic = true
			}
		}
	}

	// Extract return type for functions.
	if kind == "function" {
		for i := 0; i < childCount; i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			nodeType := child.Type()
			if nodeType == "primitive_type" || nodeType == "type_identifier" || nodeType == "sized_type_specifier" {
				text := strings.TrimSpace(child.Content(source))
				if text != "" {
					meta.ReturnType = &text
				}
				break
			}
		}

		// Extract parameters.
		for i := 0; i < childCount; i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "function_declarator" {
				if paramsNode := child.ChildByFieldName("parameters"); paramsNode != nil {
					meta.Parameters = extractCParameters(paramsNode, source)
				}
				break
			}
			// Handle pointer_declarator > function_declarator.
			if child != nil && child.Type() == "pointer_declarator" {
				innerCount := int(child.ChildCount())
				for j := 0; j < innerCount; j++ {
					inner := child.Child(j)
					if inner != nil && inner.Type() == "function_declarator" {
						if paramsNode := inner.ChildByFieldName("parameters"); paramsNode != nil {
							meta.Parameters = extractCParameters(paramsNode, source)
						}
						break
					}
				}
			}
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "{}", fmt.Errorf("failed to marshal C metadata: %w", err)
	}
	return string(data), nil
}

// ExtractEdge extracts edges from a single C tree-sitter pattern match.
//
// Pattern indices map to the order of patterns in queries/c/edges.scm:
// 0 = #include (import), 1 = function call
func (p *CLangPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
	var edges []PluginEdge
	fromFunction, _ := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// 0: #include "file.h" or #include <file.h>
	case 0:
		if source, ok := captures["source"]; ok {
			// Strip quotes and angle brackets.
			name := strings.Trim(source.Text, "\"<>")
			if name != "" {
				edges = append(edges, NewEdge(
					fmt.Sprintf("%s::__module__::function", filePath),
					name,
					"imports",
					filePath,
					source.Line,
				))
			}
		}

	// 1: foo(args) — function call
	case 1:
		if callee, ok := captures["callee"]; ok {
			if isCBuiltin(callee.Text) {
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

// ExtractDocstring extracts C doc comments (/* */ or //) preceding definitions.
func (p *CLangPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	return DefaultExtractDocstring(node, source)
}

// isCBuiltin returns true for C standard library functions that should not
// generate edges.
func isCBuiltin(name string) bool {
	switch name {
	case "printf", "fprintf", "sprintf", "snprintf",
		"scanf", "fscanf", "sscanf",
		"malloc", "calloc", "realloc", "free",
		"memcpy", "memmove", "memset", "memcmp",
		"strcmp", "strncmp", "strcpy", "strncpy", "strlen", "strcat",
		"sizeof", "exit", "abort", "assert",
		"fopen", "fclose", "fread", "fwrite", "fgets", "fputs",
		"puts", "getchar", "putchar":
		return true
	default:
		return false
	}
}

// extractCParameters extracts parameter info from a parameter_list node.
func extractCParameters(paramsNode *sitter.Node, source []byte) []CLangParameterInfo {
	var params []CLangParameterInfo

	childCount := int(paramsNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramsNode.Child(i)
		if child == nil || child.Type() != "parameter_declaration" {
			continue
		}

		info := CLangParameterInfo{}

		// Extract type from the first type-like child.
		innerCount := int(child.ChildCount())
		for j := 0; j < innerCount; j++ {
			inner := child.Child(j)
			if inner == nil {
				continue
			}
			nodeType := inner.Type()
			if nodeType == "primitive_type" || nodeType == "type_identifier" || nodeType == "sized_type_specifier" {
				text := strings.TrimSpace(inner.Content(source))
				if text != "" {
					info.Type = &text
				}
			}
			if nodeType == "identifier" {
				info.Name = inner.Content(source)
			}
			// Handle pointer declarator: const char* name
			if nodeType == "pointer_declarator" {
				if nameNode := inner.ChildByFieldName("declarator"); nameNode != nil && nameNode.Type() == "identifier" {
					info.Name = nameNode.Content(source)
				}
			}
		}

		if info.Name != "" {
			params = append(params, info)
		}
	}

	return params
}
