// C++-specific metadata extraction and language plugin.
//
// Extends C support with classes, namespaces, inheritance, member calls (->),
// scope resolution (::), new expressions, and access specifiers.
// Uses the tree-sitter C++ grammar which shares many node types with C.
package languages

import (
	"encoding/json"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/cpp"

	"github.com/KilimcininKorOglu/inari/queries"
)

// CppPlugin implements LanguagePlugin for C++ (.cpp, .cc, .cxx, .hpp, .hxx) files.
type CppPlugin struct{}

// CppMetadata holds structured metadata for a C++ symbol.
type CppMetadata struct {
	Access     string             `json:"access"`
	IsStatic   bool               `json:"isStatic"`
	IsVirtual  bool               `json:"isVirtual"`
	ReturnType *string            `json:"returnType,omitempty"`
	Parameters []CppParameterInfo `json:"parameters"`
}

// CppParameterInfo holds information about a single C++ function parameter.
type CppParameterInfo struct {
	Name string  `json:"name"`
	Type *string `json:"type,omitempty"`
}

// Language returns Cpp.
func (p *CppPlugin) Language() SupportedLanguage {
	return Cpp
}

// Extensions returns C++ file extensions.
func (p *CppPlugin) Extensions() []string {
	return []string{"cpp", "cc", "cxx", "hpp", "hxx", "h"}
}

// TSLanguage returns the tree-sitter C++ grammar.
func (p *CppPlugin) TSLanguage() *sitter.Language {
	return cpp.GetLanguage()
}

// SymbolQuerySource returns the embedded C++ symbols.scm query.
func (p *CppPlugin) SymbolQuerySource() string {
	return queries.CppSymbolsQuery
}

// EdgeQuerySource returns the embedded C++ edges.scm query.
func (p *CppPlugin) EdgeQuerySource() string {
	return queries.CppEdgesQuery
}

// InferSymbolKind maps C++ tree-sitter node types to Inari symbol kinds.
func (p *CppPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "class_specifier":
		return "class"
	case "struct_specifier":
		return "struct"
	case "enum_specifier":
		return "enum"
	case "namespace_definition":
		return "module"
	case "function_definition":
		return "function"
	case "type_definition":
		return "type"
	default:
		return "function"
	}
}

// ScopeNodeTypes returns node types that define scope boundaries in C++.
func (p *CppPlugin) ScopeNodeTypes() []string {
	return []string{
		"function_definition",
		"class_specifier",
		"struct_specifier",
		"namespace_definition",
	}
}

// ClassBodyNodeTypes returns node types for class body containers.
func (p *CppPlugin) ClassBodyNodeTypes() []string {
	return []string{"field_declaration_list", "declaration_list"}
}

// ClassDeclNodeTypes returns node types for class-like declarations.
func (p *CppPlugin) ClassDeclNodeTypes() []string {
	return []string{"class_specifier", "struct_specifier"}
}

// ExtractMetadata extracts C++-specific metadata from a symbol node as a JSON string.
func (p *CppPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := CppMetadata{
		Access:     "public",
		Parameters: []CppParameterInfo{},
	}

	childCount := int(node.ChildCount())

	// Detect static and virtual storage class.
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "storage_class_specifier":
			if strings.Contains(child.Content(source), "static") {
				meta.IsStatic = true
			}
		case "virtual_function_specifier", "virtual":
			meta.IsVirtual = true
		}
	}

	// Detect access level from preceding access_specifier in parent.
	parent := node.Parent()
	if parent != nil && parent.Type() == "field_declaration_list" {
		currentAccess := "private" // C++ class default is private.
		parentChildCount := int(parent.ChildCount())
		for i := 0; i < parentChildCount; i++ {
			sibling := parent.Child(i)
			if sibling == nil {
				continue
			}
			if sibling.Type() == "access_specifier" {
				text := sibling.Content(source)
				if strings.Contains(text, "public") {
					currentAccess = "public"
				} else if strings.Contains(text, "protected") {
					currentAccess = "protected"
				} else if strings.Contains(text, "private") {
					currentAccess = "private"
				}
			}
			if sibling == node {
				meta.Access = currentAccess
				break
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
			if nodeType == "primitive_type" || nodeType == "type_identifier" || nodeType == "sized_type_specifier" || nodeType == "template_type" {
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
					meta.Parameters = extractCppParameters(paramsNode, source)
				}
				break
			}
			if child != nil && child.Type() == "pointer_declarator" {
				innerCount := int(child.ChildCount())
				for j := 0; j < innerCount; j++ {
					inner := child.Child(j)
					if inner != nil && inner.Type() == "function_declarator" {
						if paramsNode := inner.ChildByFieldName("parameters"); paramsNode != nil {
							meta.Parameters = extractCppParameters(paramsNode, source)
						}
						break
					}
				}
			}
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "{}", fmt.Errorf("failed to marshal C++ metadata: %w", err)
	}
	return string(data), nil
}

// ExtractEdge extracts edges from a single C++ tree-sitter pattern match.
//
// Pattern indices map to the order of patterns in queries/cpp/edges.scm:
// 0 = #include, 1 = member call (->), 2 = scoped call (::), 3 = new,
// 4 = inheritance, 5 = direct call
func (p *CppPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
	var edges []PluginEdge
	fromFunction, fromClass := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// 0: #include
	case 0:
		if source, ok := captures["source"]; ok {
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

	// 1: service->method() or obj.method() — member call
	case 1:
		if callee, ok := captures["callee"]; ok {
			edges = append(edges, NewEdge(
				fromFunction,
				callee.Text,
				"calls",
				filePath,
				callee.Line,
			))
		}

	// 2: Class::staticMethod() — scope resolution call
	case 2:
		if callee, ok := captures["callee"]; ok {
			edges = append(edges, NewEdge(
				fromFunction,
				callee.Text,
				"calls",
				filePath,
				callee.Line,
			))
		}

	// 3: new ClassName() — instantiation
	case 3:
		if className, ok := captures["class_name"]; ok {
			edges = append(edges, NewEdge(
				fromFunction,
				className.Text,
				"instantiates",
				filePath,
				className.Line,
			))
		}

	// 4: : public BaseClass — inheritance
	case 4:
		if parentClass, ok := captures["parent_class"]; ok {
			edges = append(edges, NewEdge(
				fromClass,
				parentClass.Text,
				"extends",
				filePath,
				parentClass.Line,
			))
		}

	// 5: direct function call
	case 5:
		if callee, ok := captures["callee"]; ok {
			if isCppBuiltin(callee.Text) {
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

// ExtractDocstring extracts C++ doc comments (/* */ or //) preceding definitions.
func (p *CppPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	return DefaultExtractDocstring(node, source)
}

// isCppBuiltin returns true for C++ standard library functions that should not
// generate edges.
func isCppBuiltin(name string) bool {
	switch name {
	case "printf", "fprintf", "sprintf", "snprintf",
		"scanf", "fscanf", "sscanf",
		"malloc", "calloc", "realloc", "free",
		"memcpy", "memmove", "memset", "memcmp",
		"strcmp", "strncmp", "strcpy", "strncpy", "strlen",
		"sizeof", "exit", "abort", "assert",
		"cout", "cerr", "endl",
		"make_shared", "make_unique",
		"static_cast", "dynamic_cast", "reinterpret_cast", "const_cast",
		"move", "forward", "swap":
		return true
	default:
		return false
	}
}

// extractCppParameters extracts parameter info from a parameter_list node.
func extractCppParameters(paramsNode *sitter.Node, source []byte) []CppParameterInfo {
	var params []CppParameterInfo

	childCount := int(paramsNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}
		if child.Type() != "parameter_declaration" && child.Type() != "optional_parameter_declaration" {
			continue
		}

		info := CppParameterInfo{}

		innerCount := int(child.ChildCount())
		for j := 0; j < innerCount; j++ {
			inner := child.Child(j)
			if inner == nil {
				continue
			}
			nodeType := inner.Type()
			if nodeType == "primitive_type" || nodeType == "type_identifier" || nodeType == "sized_type_specifier" || nodeType == "template_type" || nodeType == "qualified_identifier" {
				text := strings.TrimSpace(inner.Content(source))
				if text != "" {
					info.Type = &text
				}
			}
			if nodeType == "identifier" {
				info.Name = inner.Content(source)
			}
			if nodeType == "reference_declarator" || nodeType == "pointer_declarator" {
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
