// C#-specific metadata extraction and language plugin.
//
// Extracts access modifiers (public, private, protected, internal),
// C#-specific modifiers (async, static, abstract, virtual, override, sealed, readonly),
// return type, and parameters from C# AST nodes.
package languages

import (
	"encoding/json"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/csharp"

	"github.com/KilimcininKorOglu/inari/queries"
)

// CSharpPlugin implements LanguagePlugin for C# (.cs) files.
type CSharpPlugin struct{}

// csMetadata holds structured metadata for a C# symbol.
type csMetadata struct {
	Access     string            `json:"access"`
	IsAsync    bool              `json:"is_async"`
	IsStatic   bool              `json:"is_static"`
	IsAbstract bool              `json:"is_abstract"`
	IsVirtual  bool              `json:"is_virtual"`
	IsOverride bool              `json:"is_override"`
	IsSealed   bool              `json:"is_sealed"`
	IsReadonly bool              `json:"is_readonly"`
	IsPartial  bool              `json:"is_partial"`
	ReturnType *string           `json:"return_type"`
	Parameters []csParameterInfo `json:"parameters"`
}

// csParameterInfo holds information about a single C# method/constructor parameter.
type csParameterInfo struct {
	Name           string  `json:"name"`
	TypeAnnotation *string `json:"type"`
	Optional       bool    `json:"optional"`
}

// Language returns CSharp.
func (p *CSharpPlugin) Language() SupportedLanguage {
	return CSharp
}

// Extensions returns ["cs"].
func (p *CSharpPlugin) Extensions() []string {
	return []string{"cs"}
}

// TSLanguage returns the tree-sitter C# grammar.
func (p *CSharpPlugin) TSLanguage() *sitter.Language {
	return csharp.GetLanguage()
}

// SymbolQuerySource returns the C# symbols.scm query text.
func (p *CSharpPlugin) SymbolQuerySource() string {
	return queries.CSharpSymbolsQuery
}

// EdgeQuerySource returns the C# edges.scm query text.
func (p *CSharpPlugin) EdgeQuerySource() string {
	return queries.CSharpEdgesQuery
}

// InferSymbolKind maps a tree-sitter node kind to an Inari symbol kind.
func (p *CSharpPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "class_declaration":
		return "class"
	case "method_declaration":
		return "method"
	case "constructor_declaration":
		return "method"
	case "property_declaration":
		return "property"
	case "interface_declaration":
		return "interface"
	case "enum_declaration":
		return "enum"
	case "struct_declaration":
		return "struct"
	case "record_declaration":
		return "class"
	case "delegate_declaration":
		return "type"
	default:
		return "function"
	}
}

// ScopeNodeTypes returns node types that constitute a scope boundary.
func (p *CSharpPlugin) ScopeNodeTypes() []string {
	return []string{
		"method_declaration",
		"constructor_declaration",
		"class_declaration",
		"struct_declaration",
		"interface_declaration",
		"record_declaration",
	}
}

// ClassBodyNodeTypes returns node types for class body nodes.
func (p *CSharpPlugin) ClassBodyNodeTypes() []string {
	return []string{"declaration_list"}
}

// ClassDeclNodeTypes returns node types for class declaration nodes.
func (p *CSharpPlugin) ClassDeclNodeTypes() []string {
	return []string{
		"class_declaration",
		"struct_declaration",
		"interface_declaration",
		"record_declaration",
	}
}

// ExtractMetadata extracts C#-specific metadata from a symbol node as a JSON string.
func (p *CSharpPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	return extractCSMetadata(node, source, kind)
}

// ExtractEdge extracts edges from a single C# query pattern match.
//
// Pattern indices map to the order of patterns in queries/csharp/edges.scm:
// 0 = using (identifier), 1 = using (qualified), 2 = member call,
// 3 = direct call, 4 = new expression, 5 = base list (identifier),
// 6 = base list (qualified)
func (p *CSharpPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
	var edges []PluginEdge
	fromFunction, fromClass := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// Using directive with identifier — always module-level
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

	// Using directive with qualified name — always module-level
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

	// Member access call (e.g. _logger.Info(...))
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

	// Direct call (e.g. DoSomething(...))
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

	// Object creation (new ...)
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

	// Base list with identifier (implements/extends)
	case 5:
		if baseType, ok := captures["base_type"]; ok {
			line := baseType.Line
			edges = append(edges, PluginEdge{
				FromID:   fromClass,
				ToID:     baseType.Text,
				Kind:     "implements",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Base list with qualified name
	case 6:
		if baseType, ok := captures["base_type"]; ok {
			line := baseType.Line
			edges = append(edges, PluginEdge{
				FromID:   fromClass,
				ToID:     baseType.Text,
				Kind:     "implements",
				FilePath: filePath,
				Line:     &line,
			})
		}
	}

	return edges
}

// ExtractDocstring uses the default docstring extraction (previous sibling comment).
func (p *CSharpPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	return DefaultExtractDocstring(node, source)
}

// extractCSMetadata extracts metadata from a C# AST node.
// Returns a JSON string suitable for the metadata column.
func extractCSMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := csMetadata{
		Parameters: []csParameterInfo{},
	}

	// Walk direct children to find modifiers
	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "modifier" {
			text := child.Content(source)
			switch text {
			case "public":
				meta.Access = "public"
			case "private":
				meta.Access = "private"
			case "protected":
				// Could be "protected internal" — check if access already set
				if meta.Access == "internal" {
					meta.Access = "protected internal"
				} else {
					meta.Access = "protected"
				}
			case "internal":
				if meta.Access == "protected" {
					meta.Access = "protected internal"
				} else {
					meta.Access = "internal"
				}
			case "async":
				meta.IsAsync = true
			case "static":
				meta.IsStatic = true
			case "abstract":
				meta.IsAbstract = true
			case "virtual":
				meta.IsVirtual = true
			case "override":
				meta.IsOverride = true
			case "sealed":
				meta.IsSealed = true
			case "readonly":
				meta.IsReadonly = true
			case "partial":
				meta.IsPartial = true
			}
		}
	}

	// Default access if none was set
	if meta.Access == "" {
		switch kind {
		case "class", "interface", "struct", "enum":
			meta.Access = "internal"
		default:
			meta.Access = "private"
		}
	}

	// Extract return type from the "returns" field (method_declaration)
	if returnsNode := node.ChildByFieldName("returns"); returnsNode != nil {
		text := returnsNode.Content(source)
		if text != "" {
			clean := strings.TrimSpace(text)
			meta.ReturnType = &clean
		}
	}

	// For properties, the type is in the "type" field
	if kind == "property" {
		if typeNode := node.ChildByFieldName("type"); typeNode != nil {
			text := typeNode.Content(source)
			if text != "" {
				clean := strings.TrimSpace(text)
				meta.ReturnType = &clean
			}
		}
	}

	// Extract parameters
	if kind == "function" || kind == "method" || kind == "constructor" {
		if paramsNode := node.ChildByFieldName("parameters"); paramsNode != nil {
			meta.Parameters = extractCSParameters(paramsNode, source)
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("failed to serialize C# metadata: %w", err)
	}
	return string(data), nil
}

// extractCSParameters extracts parameter info from a parameter_list node.
func extractCSParameters(paramsNode *sitter.Node, source []byte) []csParameterInfo {
	var params []csParameterInfo

	childCount := int(paramsNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}
		if child.Type() != "parameter" {
			continue
		}

		var name string
		if nameNode := child.ChildByFieldName("name"); nameNode != nil {
			name = nameNode.Content(source)
		}

		var typeAnnotation *string
		if typeNode := child.ChildByFieldName("type"); typeNode != nil {
			text := typeNode.Content(source)
			if text != "" {
				clean := strings.TrimSpace(text)
				typeAnnotation = &clean
			}
		}

		// Check if parameter has a default value (equals_value_clause child)
		hasDefault := false
		paramChildCount := int(child.ChildCount())
		for j := 0; j < paramChildCount; j++ {
			paramChild := child.Child(j)
			if paramChild != nil && paramChild.Type() == "equals_value_clause" {
				hasDefault = true
				break
			}
		}

		if name != "" {
			params = append(params, csParameterInfo{
				Name:           name,
				TypeAnnotation: typeAnnotation,
				Optional:       hasDefault,
			})
		}
	}

	return params
}

// PartialClassSymbol carries the minimum fields needed by MergePartialClasses.
// The caller maps core.Symbol slices to/from this type to avoid a circular
// import between the languages and core packages.
type PartialClassSymbol struct {
	// Unique identifier: "{file_path}::{name}::{kind}".
	ID string
	// The symbol name.
	Name string
	// The kind of symbol (function, class, method, etc.).
	Kind string
	// Parent symbol ID (e.g. class ID for a method).
	ParentID *string
	// JSON blob with modifiers — checked for "is_partial":true.
	Metadata string
}

// MergePartialClasses groups partial C# classes by name, keeps the first as
// primary, and re-parents methods from secondary definitions to the primary
// class symbol. Returns the IDs of symbols that should be removed (secondary
// class definitions). The caller is responsible for converting between
// core.Symbol and PartialClassSymbol to avoid circular imports.
func MergePartialClasses(symbols []PartialClassSymbol) []string {
	var removals []string

	// Group class symbols by name
	classGroups := make(map[string][]int)
	for i, symbol := range symbols {
		if symbol.Kind == "class" {
			// Only merge if the class is marked partial
			if strings.Contains(symbol.Metadata, "\"is_partial\":true") {
				classGroups[symbol.Name] = append(classGroups[symbol.Name], i)
			}
		}
	}

	// For each class with multiple definitions, merge them
	for _, indices := range classGroups {
		if len(indices) <= 1 {
			continue
		}

		primaryID := symbols[indices[0]].ID

		// Collect secondary class IDs
		secondaryIDs := make(map[string]bool)
		for _, idx := range indices[1:] {
			secondaryIDs[symbols[idx].ID] = true
		}

		// Re-parent methods from secondary classes to primary
		for i := range symbols {
			if symbols[i].ParentID != nil && secondaryIDs[*symbols[i].ParentID] {
				pid := primaryID
				symbols[i].ParentID = &pid
			}
		}

		// Mark secondary class symbols for removal
		for id := range secondaryIDs {
			removals = append(removals, id)
		}
	}

	return removals
}
