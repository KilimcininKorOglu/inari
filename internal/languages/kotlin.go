// Kotlin-specific metadata extraction and language plugin.
//
// Extracts visibility modifiers (public, private, protected, internal),
// Kotlin-specific modifiers (open, abstract, final, data, sealed, inner,
// inline, suspend, override, lateinit), val/var distinction, return type,
// and parameters from Kotlin AST nodes.
package languages

import (
	"encoding/json"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"

	"github.com/KilimcininKorOglu/inari/queries"
)

// KotlinPlugin implements LanguagePlugin for Kotlin (.kt) files.
type KotlinPlugin struct{}

// KotlinMetadata holds structured metadata for a Kotlin symbol.
type KotlinMetadata struct {
	Access     string                `json:"access"`
	IsOpen     bool                  `json:"isOpen"`
	IsAbstract bool                  `json:"isAbstract"`
	IsFinal    bool                  `json:"isFinal"`
	IsData     bool                  `json:"isData"`
	IsSealed   bool                  `json:"isSealed"`
	IsInner    bool                  `json:"isInner"`
	IsInline   bool                  `json:"isInline"`
	IsSuspend  bool                  `json:"isSuspend"`
	IsOverride bool                  `json:"isOverride"`
	IsLateinit bool                  `json:"isLateinit"`
	ValOrVar   string                `json:"valOrVar,omitempty"`
	ReturnType *string               `json:"returnType,omitempty"`
	Parameters []KotlinParameterInfo `json:"parameters"`
}

// KotlinParameterInfo holds information about a single Kotlin function parameter.
type KotlinParameterInfo struct {
	Name           string  `json:"name"`
	TypeAnnotation *string `json:"type"`
}

// Language returns Kotlin.
func (p *KotlinPlugin) Language() SupportedLanguage {
	return Kotlin
}

// Extensions returns ["kt"].
func (p *KotlinPlugin) Extensions() []string {
	return []string{"kt"}
}

// TSLanguage returns the tree-sitter Kotlin grammar.
func (p *KotlinPlugin) TSLanguage() *sitter.Language {
	return kotlin.GetLanguage()
}

// SymbolQuerySource returns the embedded Kotlin symbols.scm query.
func (p *KotlinPlugin) SymbolQuerySource() string {
	return queries.KotlinSymbolsQuery
}

// EdgeQuerySource returns the embedded Kotlin edges.scm query.
func (p *KotlinPlugin) EdgeQuerySource() string {
	return queries.KotlinEdgesQuery
}

// InferSymbolKind maps Kotlin tree-sitter node types to Inari symbol kinds.
func (p *KotlinPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "class_declaration":
		return "class"
	case "interface_declaration":
		return "interface"
	case "object_declaration":
		return "class"
	case "companion_object":
		return "class"
	case "function_declaration":
		return "function"
	case "property_declaration":
		return "property"
	default:
		return "function"
	}
}

// ScopeNodeTypes returns node types that define scope boundaries in Kotlin.
func (p *KotlinPlugin) ScopeNodeTypes() []string {
	return []string{
		"class_declaration",
		"interface_declaration",
		"object_declaration",
		"companion_object",
		"function_declaration",
	}
}

// ClassBodyNodeTypes returns node types for class body containers.
func (p *KotlinPlugin) ClassBodyNodeTypes() []string {
	return []string{"class_body"}
}

// ClassDeclNodeTypes returns node types for class-like declarations.
func (p *KotlinPlugin) ClassDeclNodeTypes() []string {
	return []string{
		"class_declaration",
		"interface_declaration",
		"object_declaration",
		"companion_object",
	}
}

// ExtractMetadata extracts Kotlin-specific metadata from a symbol node as a JSON string.
func (p *KotlinPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := KotlinMetadata{
		Parameters: []KotlinParameterInfo{},
	}

	// Walk direct children to find modifiers.
	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "modifiers" {
			extractKotlinModifiers(child, source, &meta)
		}
	}

	// Default access: Kotlin defaults to public.
	if meta.Access == "" {
		meta.Access = "public"
	}

	// Detect val/var for properties.
	if kind == "property" {
		for i := 0; i < childCount; i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			text := child.Content(source)
			if text == "val" || text == "var" {
				meta.ValOrVar = text
				break
			}
		}
	}

	// Extract return type for functions.
	if kind == "function" || kind == "method" {
		if typeNode := node.ChildByFieldName("type"); typeNode != nil {
			text := strings.TrimSpace(typeNode.Content(source))
			if text != "" {
				meta.ReturnType = &text
			}
		}
		// Extract parameters.
		if paramsNode := node.ChildByFieldName("function_value_parameters"); paramsNode != nil {
			meta.Parameters = extractKotlinParameters(paramsNode, source)
		}
		// Try alternative parameter node name.
		if len(meta.Parameters) == 0 {
			for i := 0; i < childCount; i++ {
				child := node.Child(i)
				if child != nil && child.Type() == "function_value_parameters" {
					meta.Parameters = extractKotlinParameters(child, source)
					break
				}
			}
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "{}", fmt.Errorf("failed to marshal Kotlin metadata: %w", err)
	}
	return string(data), nil
}

// ExtractEdge extracts edges from a single Kotlin tree-sitter pattern match.
//
// Pattern indices map to the order of patterns in queries/kotlin/edges.scm:
// 0 = import, 1 = method call with receiver, 2 = direct/constructor call,
// 3 = delegation specifier (extends/implements)
func (p *KotlinPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
	var edges []PluginEdge
	fromFunction, fromClass := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// 0: Import
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

	// 1: Method call with receiver (e.g. logger.info("hello"))
	case 1:
		receiver, hasReceiver := captures["receiver"]
		callee, hasCallee := captures["callee"]
		if hasReceiver && hasCallee {
			line := receiver.Line
			edges = append(edges, PluginEdge{
				FromID:   fromFunction,
				ToID:     fmt.Sprintf("%s.%s", receiver.Text, callee.Text),
				Kind:     "calls",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// 2: Direct call or constructor call (e.g. process() or Logger("test"))
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

	// 3: Delegation specifier (extends/implements — Kotlin uses : for both)
	case 3:
		if parentType, ok := captures["parent_type"]; ok {
			line := parentType.Line
			edges = append(edges, PluginEdge{
				FromID:   fromClass,
				ToID:     parentType.Text,
				Kind:     "implements",
				FilePath: filePath,
				Line:     &line,
			})
		}
	}

	return edges
}

// ExtractDocstring extracts Kotlin doc comments (/** ... */) preceding declarations.
func (p *KotlinPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	prev := node.PrevSibling()
	if prev == nil {
		return ""
	}
	if prev.Type() == "multiline_comment" || prev.Type() == "comment" {
		text := prev.Content(source)
		if text != "" {
			return trimString(text)
		}
	}
	return ""
}

// extractKotlinModifiers walks the modifiers node and populates metadata fields.
func extractKotlinModifiers(modifiersNode *sitter.Node, source []byte, meta *KotlinMetadata) {
	childCount := int(modifiersNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := modifiersNode.Child(i)
		if child == nil {
			continue
		}

		nodeType := child.Type()
		text := child.Content(source)

		switch nodeType {
		case "visibility_modifier":
			switch text {
			case "public":
				meta.Access = "public"
			case "private":
				meta.Access = "private"
			case "protected":
				meta.Access = "protected"
			case "internal":
				meta.Access = "internal"
			}
		case "class_modifier":
			switch text {
			case "data":
				meta.IsData = true
			case "sealed":
				meta.IsSealed = true
			case "inner":
				meta.IsInner = true
			case "abstract":
				meta.IsAbstract = true
			case "open":
				meta.IsOpen = true
			case "final":
				meta.IsFinal = true
			case "enum":
				// Enum is a class modifier in Kotlin
			}
		case "member_modifier":
			switch text {
			case "override":
				meta.IsOverride = true
			case "lateinit":
				meta.IsLateinit = true
			case "abstract":
				meta.IsAbstract = true
			case "open":
				meta.IsOpen = true
			case "final":
				meta.IsFinal = true
			}
		case "function_modifier":
			switch text {
			case "suspend":
				meta.IsSuspend = true
			case "inline":
				meta.IsInline = true
			}
		case "inheritance_modifier":
			switch text {
			case "abstract":
				meta.IsAbstract = true
			case "open":
				meta.IsOpen = true
			case "final":
				meta.IsFinal = true
			}
		}
	}
}

// extractKotlinParameters extracts parameter info from a function_value_parameters node.
func extractKotlinParameters(paramsNode *sitter.Node, source []byte) []KotlinParameterInfo {
	var params []KotlinParameterInfo

	childCount := int(paramsNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}
		if child.Type() != "parameter" && child.Type() != "function_value_parameter" {
			continue
		}

		info := KotlinParameterInfo{}

		// Try field name "name" first, then walk children.
		if nameNode := child.ChildByFieldName("name"); nameNode != nil {
			info.Name = nameNode.Content(source)
		} else {
			// Walk children to find simple_identifier.
			paramChildCount := int(child.ChildCount())
			for j := 0; j < paramChildCount; j++ {
				paramChild := child.Child(j)
				if paramChild != nil && paramChild.Type() == "simple_identifier" {
					info.Name = paramChild.Content(source)
					break
				}
			}
		}

		// Extract type.
		if typeNode := child.ChildByFieldName("type"); typeNode != nil {
			text := strings.TrimSpace(typeNode.Content(source))
			if text != "" {
				info.TypeAnnotation = &text
			}
		} else {
			// Walk children for user_type.
			paramChildCount := int(child.ChildCount())
			for j := 0; j < paramChildCount; j++ {
				paramChild := child.Child(j)
				if paramChild != nil && (paramChild.Type() == "user_type" || paramChild.Type() == "nullable_type") {
					text := strings.TrimSpace(paramChild.Content(source))
					if text != "" {
						info.TypeAnnotation = &text
					}
					break
				}
			}
		}

		if info.Name != "" {
			params = append(params, info)
		}
	}

	return params
}
