// TypeScript-specific metadata extraction and language plugin.
//
// Extracts access modifiers, async, static, return type, and parameters
// from TypeScript AST nodes. TypeScript defaults to public access.
package languages

import (
	"encoding/json"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/KilimcininKorOglu/inari/queries"
)

// TypeScriptPlugin implements LanguagePlugin for TypeScript (.ts, .tsx) files.
type TypeScriptPlugin struct{}

// tsSymbolMetadata holds structured metadata for a TypeScript symbol.
type tsSymbolMetadata struct {
	Access     string            `json:"access"`
	IsAsync    bool              `json:"is_async"`
	IsStatic   bool              `json:"is_static"`
	IsAbstract bool              `json:"is_abstract"`
	IsReadonly bool              `json:"is_readonly"`
	ReturnType *string           `json:"return_type"`
	Parameters []tsParameterInfo `json:"parameters"`
}

// tsParameterInfo holds information about a single function/method parameter.
type tsParameterInfo struct {
	Name           string  `json:"name"`
	TypeAnnotation *string `json:"type"`
	Optional       bool    `json:"optional"`
}

// Language returns TypeScript.
func (p *TypeScriptPlugin) Language() SupportedLanguage {
	return TypeScript
}

// Extensions returns ["ts", "tsx"].
func (p *TypeScriptPlugin) Extensions() []string {
	return []string{"ts", "tsx"}
}

// TSLanguage returns the tree-sitter TypeScript grammar.
func (p *TypeScriptPlugin) TSLanguage() *sitter.Language {
	return typescript.GetLanguage()
}

// SymbolQuerySource returns the TypeScript symbols.scm query text.
func (p *TypeScriptPlugin) SymbolQuerySource() string {
	return queries.TypeScriptSymbolsQuery
}

// EdgeQuerySource returns the TypeScript edges.scm query text.
func (p *TypeScriptPlugin) EdgeQuerySource() string {
	return queries.TypeScriptEdgesQuery
}

// InferSymbolKind maps a tree-sitter node kind to an Inari symbol kind.
func (p *TypeScriptPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "function_declaration":
		return "function"
	case "class_declaration":
		return "class"
	case "method_definition":
		return "method"
	case "interface_declaration":
		return "interface"
	case "enum_declaration":
		return "enum"
	case "type_alias_declaration":
		return "type"
	case "public_field_definition":
		return "property"
	case "lexical_declaration", "arrow_function", "function_expression":
		return "function"
	default:
		return "function"
	}
}

// ScopeNodeTypes returns node types that constitute a scope boundary.
func (p *TypeScriptPlugin) ScopeNodeTypes() []string {
	return []string{
		"function_declaration",
		"method_definition",
		"arrow_function",
		"function_expression",
		"class_declaration",
		"interface_declaration",
	}
}

// ClassBodyNodeTypes returns node types for class body nodes.
func (p *TypeScriptPlugin) ClassBodyNodeTypes() []string {
	return []string{"class_body"}
}

// ClassDeclNodeTypes returns node types for class declaration nodes.
func (p *TypeScriptPlugin) ClassDeclNodeTypes() []string {
	return []string{"class_declaration"}
}

// ExtractMetadata extracts TypeScript-specific metadata from a symbol node as a JSON string.
func (p *TypeScriptPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	return extractTSMetadata(node, source, kind)
}

// ExtractEdge extracts edges from a single TypeScript query pattern match.
//
// Pattern indices map to the order of patterns in queries/typescript/edges.scm:
// 0 = import, 1 = direct call, 2 = member call, 3 = chained member call,
// 4 = new expression, 5 = extends, 6 = implements, 7 = type reference
func (p *TypeScriptPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
	var edges []PluginEdge
	fromFunction, fromClass := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// Import statement — always module-level
	case 0:
		importedName, hasImport := captures["imported_name"]
		sourceCap, hasSource := captures["source"]
		if hasImport && hasSource {
			sourceClean := strings.Trim(sourceCap.Text, "'\"")
			line := importedName.Line
			edges = append(edges, PluginEdge{
				FromID:   fmt.Sprintf("%s::__module__::function", filePath),
				ToID:     fmt.Sprintf("%s::%s", sourceClean, importedName.Text),
				Kind:     "imports",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Direct call expression
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

	// Member call expression / chained member access call
	case 2, 3:
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

	// New expression (instantiation)
	case 4:
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

	// Extends clause
	case 5:
		if baseClass, ok := captures["base_class"]; ok {
			line := baseClass.Line
			edges = append(edges, PluginEdge{
				FromID:   fromClass,
				ToID:     baseClass.Text,
				Kind:     "extends",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Implements clause
	case 6:
		if ifaceName, ok := captures["interface_name"]; ok {
			line := ifaceName.Line
			edges = append(edges, PluginEdge{
				FromID:   fromClass,
				ToID:     ifaceName.Text,
				Kind:     "implements",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Type reference
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
	}

	return edges
}

// ExtractDocstring uses the default docstring extraction (previous sibling comment).
func (p *TypeScriptPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	return DefaultExtractDocstring(node, source)
}

// extractTSMetadata extracts metadata from a TypeScript AST node.
// Returns a JSON string suitable for the metadata column.
func extractTSMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := tsSymbolMetadata{
		Access:     "public",
		Parameters: []tsParameterInfo{},
	}

	// Walk direct children to find modifiers
	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "async":
			meta.IsAsync = true
		case "static":
			meta.IsStatic = true
		case "abstract":
			meta.IsAbstract = true
		case "readonly":
			meta.IsReadonly = true
		case "accessibility_modifier":
			text := child.Content(source)
			if text != "" {
				meta.Access = text
			}
		}
	}

	// Extract return type from type_annotation field
	if returnTypeNode := node.ChildByFieldName("return_type"); returnTypeNode != nil {
		text := returnTypeNode.Content(source)
		if text != "" {
			// Strip the leading `: ` from type annotations
			clean := strings.TrimSpace(strings.TrimPrefix(text, ":"))
			meta.ReturnType = &clean
		}
	}

	// Extract parameters
	if kind == "function" || kind == "method" {
		if paramsNode := node.ChildByFieldName("parameters"); paramsNode != nil {
			meta.Parameters = extractTSParameters(paramsNode, source)
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("failed to serialize TypeScript metadata: %w", err)
	}
	return string(data), nil
}

// extractTSParameters extracts parameter info from a formal_parameters node.
func extractTSParameters(paramsNode *sitter.Node, source []byte) []tsParameterInfo {
	var params []tsParameterInfo

	childCount := int(paramsNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		childKind := child.Type()
		if childKind != "required_parameter" && childKind != "optional_parameter" {
			continue
		}

		optional := childKind == "optional_parameter"

		var name string
		if patternNode := child.ChildByFieldName("pattern"); patternNode != nil {
			name = patternNode.Content(source)
		}

		var typeAnnotation *string
		if typeNode := child.ChildByFieldName("type"); typeNode != nil {
			text := typeNode.Content(source)
			if text != "" {
				clean := strings.TrimSpace(strings.TrimPrefix(text, ":"))
				typeAnnotation = &clean
			}
		}

		if name != "" {
			params = append(params, tsParameterInfo{
				Name:           name,
				TypeAnnotation: typeAnnotation,
				Optional:       optional,
			})
		}
	}

	return params
}
