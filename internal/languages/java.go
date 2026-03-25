// Java-specific metadata extraction and language plugin.
//
// Extracts access modifiers (public, private, protected, package-private),
// Java-specific modifiers (static, abstract, final, synchronized, native),
// return type, and parameters from Java AST nodes.
// Supports Java 8-21 features including records, sealed classes, and annotations.
package languages

import (
	"encoding/json"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"

	"github.com/KilimcininKorOglu/inari/queries"
)

// JavaPlugin implements LanguagePlugin for Java (.java) files.
type JavaPlugin struct{}

// JavaMetadata holds structured metadata for a Java symbol.
type JavaMetadata struct {
	Access         string              `json:"access"`
	IsStatic       bool                `json:"isStatic"`
	IsAbstract     bool                `json:"isAbstract"`
	IsFinal        bool                `json:"isFinal"`
	IsSynchronized bool                `json:"isSynchronized"`
	IsNative       bool                `json:"isNative"`
	IsDefault      bool                `json:"isDefault"`
	ReturnType     *string             `json:"returnType,omitempty"`
	Parameters     []JavaParameterInfo `json:"parameters"`
}

// JavaParameterInfo holds information about a single Java method/constructor parameter.
type JavaParameterInfo struct {
	Name           string  `json:"name"`
	TypeAnnotation *string `json:"type"`
	IsFinal        bool    `json:"isFinal"`
}

// Language returns Java.
func (p *JavaPlugin) Language() SupportedLanguage {
	return Java
}

// Extensions returns ["java"].
func (p *JavaPlugin) Extensions() []string {
	return []string{"java"}
}

// TSLanguage returns the tree-sitter Java grammar.
func (p *JavaPlugin) TSLanguage() *sitter.Language {
	return java.GetLanguage()
}

// SymbolQuerySource returns the embedded Java symbols.scm query.
func (p *JavaPlugin) SymbolQuerySource() string {
	return queries.JavaSymbolsQuery
}

// EdgeQuerySource returns the embedded Java edges.scm query.
func (p *JavaPlugin) EdgeQuerySource() string {
	return queries.JavaEdgesQuery
}

// InferSymbolKind maps Java tree-sitter node types to Inari symbol kinds.
func (p *JavaPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "class_declaration":
		return "class"
	case "interface_declaration":
		return "interface"
	case "enum_declaration":
		return "enum"
	case "record_declaration":
		return "class"
	case "annotation_type_declaration":
		return "interface"
	case "method_declaration":
		return "method"
	case "constructor_declaration":
		return "method"
	case "field_declaration":
		return "property"
	default:
		return "function"
	}
}

// ScopeNodeTypes returns node types that define scope boundaries in Java.
func (p *JavaPlugin) ScopeNodeTypes() []string {
	return []string{
		"class_declaration",
		"interface_declaration",
		"enum_declaration",
		"record_declaration",
		"annotation_type_declaration",
		"method_declaration",
		"constructor_declaration",
	}
}

// ClassBodyNodeTypes returns node types for class/interface body containers.
func (p *JavaPlugin) ClassBodyNodeTypes() []string {
	return []string{
		"class_body",
		"interface_body",
		"enum_body",
		"annotation_type_body",
	}
}

// ClassDeclNodeTypes returns node types for class-like declarations.
func (p *JavaPlugin) ClassDeclNodeTypes() []string {
	return []string{
		"class_declaration",
		"interface_declaration",
		"enum_declaration",
		"record_declaration",
		"annotation_type_declaration",
	}
}

// ExtractMetadata extracts Java-specific metadata from a symbol node as a JSON string.
func (p *JavaPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := JavaMetadata{
		Parameters: []JavaParameterInfo{},
	}

	// Walk direct children to find modifiers.
	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "modifiers" {
			extractJavaModifiers(child, source, &meta)
		}
	}

	// Default access if none was set (Java package-private).
	if meta.Access == "" {
		meta.Access = "package-private"
	}

	// Extract return type from the "type" field (method_declaration).
	if kind == "method" && node.Type() == "method_declaration" {
		if typeNode := node.ChildByFieldName("type"); typeNode != nil {
			text := strings.TrimSpace(typeNode.Content(source))
			if text != "" {
				meta.ReturnType = &text
			}
		}
	}

	// Extract parameters for methods and constructors.
	if kind == "method" {
		if paramsNode := node.ChildByFieldName("parameters"); paramsNode != nil {
			meta.Parameters = extractJavaParameters(paramsNode, source)
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "{}", fmt.Errorf("failed to marshal Java metadata: %w", err)
	}
	return string(data), nil
}

// ExtractEdge extracts edges from a single Java tree-sitter pattern match.
//
// Pattern indices map to the order of patterns in queries/java/edges.scm:
// 0 = import, 1 = method call with receiver, 2 = method call without receiver,
// 3 = object creation, 4 = extends, 5 = implements
func (p *JavaPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
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

	// 1: Method call with receiver (e.g. service.process())
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

	// 2: Method call without receiver (e.g. process())
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

	// 3: Object creation (new ClassName())
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

	// 4: Extends (class Foo extends Bar)
	case 4:
		if parentClass, ok := captures["parent_class"]; ok {
			line := parentClass.Line
			edges = append(edges, PluginEdge{
				FromID:   fromClass,
				ToID:     parentClass.Text,
				Kind:     "extends",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// 5: Implements (class Foo implements IBar)
	case 5:
		if interfaceName, ok := captures["interface_name"]; ok {
			line := interfaceName.Line
			edges = append(edges, PluginEdge{
				FromID:   fromClass,
				ToID:     interfaceName.Text,
				Kind:     "implements",
				FilePath: filePath,
				Line:     &line,
			})
		}
	}

	return edges
}

// ExtractDocstring uses the default docstring extraction.
// Java's Javadoc (/** ... */) is a block_comment node preceding the definition.
func (p *JavaPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	prev := node.PrevSibling()
	if prev == nil {
		return ""
	}
	// Java tree-sitter uses "block_comment" for Javadoc and "line_comment" for //.
	if prev.Type() == "block_comment" || prev.Type() == "line_comment" {
		text := prev.Content(source)
		if text != "" {
			return trimString(text)
		}
	}
	return ""
}

// extractJavaModifiers walks the modifiers node and populates metadata fields.
func extractJavaModifiers(modifiersNode *sitter.Node, source []byte, meta *JavaMetadata) {
	childCount := int(modifiersNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := modifiersNode.Child(i)
		if child == nil {
			continue
		}
		text := child.Content(source)
		switch text {
		case "public":
			meta.Access = "public"
		case "private":
			meta.Access = "private"
		case "protected":
			meta.Access = "protected"
		case "static":
			meta.IsStatic = true
		case "abstract":
			meta.IsAbstract = true
		case "final":
			meta.IsFinal = true
		case "synchronized":
			meta.IsSynchronized = true
		case "native":
			meta.IsNative = true
		case "default":
			meta.IsDefault = true
		}
	}
}

// extractJavaParameters extracts parameter info from a formal_parameters node.
func extractJavaParameters(paramsNode *sitter.Node, source []byte) []JavaParameterInfo {
	var params []JavaParameterInfo

	childCount := int(paramsNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "formal_parameter", "spread_parameter":
			param := extractSingleJavaParam(child, source)
			if param.Name != "" {
				params = append(params, param)
			}
		}
	}

	return params
}

// extractSingleJavaParam extracts info from a single formal_parameter or spread_parameter node.
func extractSingleJavaParam(paramNode *sitter.Node, source []byte) JavaParameterInfo {
	info := JavaParameterInfo{}

	// Check for final modifier.
	childCount := int(paramNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramNode.Child(i)
		if child != nil && child.Type() == "modifiers" {
			modText := child.Content(source)
			if strings.Contains(modText, "final") {
				info.IsFinal = true
			}
		}
	}

	if nameNode := paramNode.ChildByFieldName("name"); nameNode != nil {
		info.Name = nameNode.Content(source)
	}

	if typeNode := paramNode.ChildByFieldName("type"); typeNode != nil {
		text := strings.TrimSpace(typeNode.Content(source))
		if text != "" {
			if paramNode.Type() == "spread_parameter" {
				text = text + "..."
			}
			info.TypeAnnotation = &text
		}
	}

	return info
}
