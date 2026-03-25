// PHP-specific metadata extraction and language plugin.
//
// Extracts visibility modifiers (public, private, protected), static, abstract,
// final, readonly modifiers, return type annotations, and parameters from PHP
// AST nodes. PHP modifiers are actual AST nodes (visibility_modifier, etc.),
// unlike Ruby where visibility is a method call.
//
// PHP has two distinct "use" constructs in tree-sitter:
//   - namespace_use_declaration: import at file level (use App\Foo;)
//   - use_declaration: trait inclusion inside a class body (use SomeTrait;)
package languages

import (
	"encoding/json"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/php"

	"github.com/KilimcininKorOglu/inari/queries"
)

// PhpPlugin implements LanguagePlugin for PHP (.php) files.
type PhpPlugin struct{}

// PhpMetadata holds structured metadata for a PHP symbol.
type PhpMetadata struct {
	Access     string             `json:"access"`
	IsStatic   bool               `json:"isStatic"`
	IsAbstract bool               `json:"isAbstract"`
	IsFinal    bool               `json:"isFinal"`
	IsReadonly bool               `json:"isReadonly"`
	ReturnType *string            `json:"returnType,omitempty"`
	Parameters []PhpParameterInfo `json:"parameters"`
}

// PhpParameterInfo holds information about a single PHP function/method parameter.
type PhpParameterInfo struct {
	Name           string  `json:"name"`
	TypeAnnotation *string `json:"type,omitempty"`
	HasDefault     bool    `json:"hasDefault"`
	IsVariadic     bool    `json:"isVariadic"`
}

// Language returns Php.
func (p *PhpPlugin) Language() SupportedLanguage {
	return Php
}

// Extensions returns ["php"].
func (p *PhpPlugin) Extensions() []string {
	return []string{"php"}
}

// TSLanguage returns the tree-sitter PHP grammar.
func (p *PhpPlugin) TSLanguage() *sitter.Language {
	return php.GetLanguage()
}

// SymbolQuerySource returns the embedded PHP symbols.scm query.
func (p *PhpPlugin) SymbolQuerySource() string {
	return queries.PhpSymbolsQuery
}

// EdgeQuerySource returns the embedded PHP edges.scm query.
func (p *PhpPlugin) EdgeQuerySource() string {
	return queries.PhpEdgesQuery
}

// InferSymbolKind maps PHP tree-sitter node types to Inari symbol kinds.
func (p *PhpPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "namespace_definition":
		return "module"
	case "class_declaration":
		return "class"
	case "interface_declaration":
		return "interface"
	case "trait_declaration":
		return "interface"
	case "enum_declaration":
		return "enum"
	case "function_definition":
		return "function"
	case "method_declaration":
		return "method"
	case "property_declaration":
		return "property"
	case "const_declaration":
		return "const"
	default:
		return "function"
	}
}

// ScopeNodeTypes returns node types that define scope boundaries in PHP.
func (p *PhpPlugin) ScopeNodeTypes() []string {
	return []string{
		"namespace_definition",
		"class_declaration",
		"interface_declaration",
		"trait_declaration",
		"enum_declaration",
		"function_definition",
		"method_declaration",
	}
}

// ClassBodyNodeTypes returns node types for class body containers in PHP.
func (p *PhpPlugin) ClassBodyNodeTypes() []string {
	return []string{"declaration_list"}
}

// ClassDeclNodeTypes returns node types for class-like declarations in PHP.
func (p *PhpPlugin) ClassDeclNodeTypes() []string {
	return []string{
		"class_declaration",
		"interface_declaration",
		"trait_declaration",
		"enum_declaration",
	}
}

// ExtractMetadata extracts PHP-specific metadata from a symbol node as a JSON string.
func (p *PhpPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := PhpMetadata{
		Access:     "public",
		Parameters: []PhpParameterInfo{},
	}

	// Walk direct children to find modifier nodes.
	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "visibility_modifier":
			text := child.Content(source)
			if text == "private" || text == "protected" || text == "public" {
				meta.Access = text
			}
		case "static_modifier":
			meta.IsStatic = true
		case "abstract_modifier":
			meta.IsAbstract = true
		case "final_modifier":
			meta.IsFinal = true
		case "readonly_modifier":
			meta.IsReadonly = true
		}
	}

	// Extract return type for functions/methods.
	if kind == "function" || kind == "method" {
		if returnType := node.ChildByFieldName("return_type"); returnType != nil {
			text := strings.TrimSpace(returnType.Content(source))
			if text != "" {
				meta.ReturnType = &text
			}
		}
		// Extract parameters.
		if paramsNode := node.ChildByFieldName("parameters"); paramsNode != nil {
			meta.Parameters = extractPhpParameters(paramsNode, source)
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "{}", fmt.Errorf("failed to marshal PHP metadata: %w", err)
	}
	return string(data), nil
}

// ExtractEdge extracts edges from a single PHP tree-sitter pattern match.
//
// Pattern indices map to the order of patterns in queries/php/edges.scm:
// 0 = namespace use (import), 1 = member call, 2 = scoped/static call,
// 3 = new (instantiate), 4 = extends, 5 = implements, 6 = trait use, 7 = direct call
func (p *PhpPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
	var edges []PluginEdge
	fromFunction, fromClass := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// 0: use Foo\Bar; (namespace import)
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

	// 1: $obj->method() (member call)
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

	// 2: ClassName::staticMethod() (scoped/static call)
	case 2:
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

	// 3: new ClassName() (object creation)
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

	// 4: extends ParentClass
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

	// 5: implements InterfaceName
	case 5:
		if interfaceName, ok := captures["interface_name"]; ok {
			edges = append(edges, NewEdge(
				fromClass,
				interfaceName.Text,
				"implements",
				filePath,
				interfaceName.Line,
			))
		}

	// 6: use TraitName; (trait use inside class body)
	case 6:
		if traitName, ok := captures["trait_name"]; ok {
			edges = append(edges, NewEdge(
				fromClass,
				traitName.Text,
				"includes",
				filePath,
				traitName.Line,
			))
		}

	// 7: direct function call (e.g. array_map(...), myFunction())
	case 7:
		if callee, ok := captures["callee"]; ok {
			if isPhpBuiltinCall(callee.Text) {
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

// ExtractDocstring extracts PHP doc comments (/** ... */) preceding declarations.
func (p *PhpPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	return DefaultExtractDocstring(node, source)
}

// isPhpBuiltinCall returns true for PHP built-in constructs that should not
// generate edges.
func isPhpBuiltinCall(name string) bool {
	switch name {
	case "echo", "print", "var_dump", "print_r",
		"isset", "unset", "empty",
		"die", "exit",
		"array", "list",
		"require", "require_once", "include", "include_once",
		"is_null", "is_array", "is_string", "is_int",
		"count", "strlen", "sizeof",
		"throw":
		return true
	default:
		return false
	}
}

// extractPhpParameters extracts parameter info from a formal_parameters node.
func extractPhpParameters(paramsNode *sitter.Node, source []byte) []PhpParameterInfo {
	var params []PhpParameterInfo

	childCount := int(paramsNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}
		if child.Type() != "simple_parameter" && child.Type() != "variadic_parameter" && child.Type() != "property_promotion_parameter" {
			continue
		}

		info := PhpParameterInfo{}

		if child.Type() == "variadic_parameter" {
			info.IsVariadic = true
		}

		// Extract parameter name (variable_name with $ prefix).
		if nameNode := child.ChildByFieldName("name"); nameNode != nil {
			name := nameNode.Content(source)
			name = strings.TrimPrefix(name, "$")
			if name != "" {
				info.Name = name
			}
		}

		// Extract type hint.
		if typeNode := child.ChildByFieldName("type"); typeNode != nil {
			text := strings.TrimSpace(typeNode.Content(source))
			if text != "" {
				info.TypeAnnotation = &text
			}
		}

		// Check for default value.
		if defaultNode := child.ChildByFieldName("default_value"); defaultNode != nil {
			info.HasDefault = true
		}

		if info.Name != "" {
			params = append(params, info)
		}
	}

	return params
}
