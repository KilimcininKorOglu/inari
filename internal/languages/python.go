// Python-specific metadata extraction and language plugin.
//
// Extracts access level (public, private, name_mangled from naming conventions),
// async, decorator-based modifiers (staticmethod, classmethod, abstractmethod, property),
// return type annotations, and parameters from Python AST nodes.
//
// Python docstrings are NOT comment nodes — they are the first expression_statement
// child of the function/class body containing a string node.
package languages

import (
	"encoding/json"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"

	"github.com/KilimcininKorOglu/inari/queries"
)

// PythonPlugin implements LanguagePlugin for Python (.py) files.
type PythonPlugin struct{}

// PythonMetadata holds structured metadata for a Python symbol.
type PythonMetadata struct {
	Access      string              `json:"access"`
	IsAsync     bool                `json:"is_async"`
	IsStatic    bool                `json:"is_static"`
	IsClassmethod bool             `json:"is_classmethod"`
	IsAbstract  bool                `json:"is_abstract"`
	IsProperty  bool                `json:"is_property"`
	Decorators  []string            `json:"decorators"`
	ReturnType  *string             `json:"return_type"`
	Parameters  []PythonParameterInfo `json:"parameters"`
}

// PythonParameterInfo holds information about a single Python function/method parameter.
type PythonParameterInfo struct {
	Name           string  `json:"name"`
	TypeAnnotation *string `json:"type"`
	HasDefault     bool    `json:"has_default"`
}

// Language returns Python.
func (p *PythonPlugin) Language() SupportedLanguage {
	return Python
}

// Extensions returns ["py"].
func (p *PythonPlugin) Extensions() []string {
	return []string{"py"}
}

// TSLanguage returns the tree-sitter Python grammar.
func (p *PythonPlugin) TSLanguage() *sitter.Language {
	return python.GetLanguage()
}

// SymbolQuerySource returns the Python symbols.scm query text.
func (p *PythonPlugin) SymbolQuerySource() string {
	return queries.PythonSymbolsQuery
}

// EdgeQuerySource returns the Python edges.scm query text.
func (p *PythonPlugin) EdgeQuerySource() string {
	return queries.PythonEdgesQuery
}

// InferSymbolKind maps a tree-sitter node kind to an Inari symbol kind.
//
// Python uses function_definition for both top-level functions and class methods.
// We map to "function" here. Note: parser.go only sets parent_id for
// kind == "method" || kind == "property", so Python methods won't have
// parent_id set automatically. This is a known limitation of the current
// LanguagePlugin trait contract.
func (p *PythonPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "function_definition":
		return "function"
	case "class_definition":
		return "class"
	default:
		return "function"
	}
}

// ScopeNodeTypes returns node types that constitute a scope boundary.
func (p *PythonPlugin) ScopeNodeTypes() []string {
	return []string{
		"function_definition",
		"class_definition",
		"decorated_definition",
		"module",
	}
}

// ClassBodyNodeTypes returns node types for class body nodes.
func (p *PythonPlugin) ClassBodyNodeTypes() []string {
	return []string{"block"}
}

// ClassDeclNodeTypes returns node types for class declaration nodes.
func (p *PythonPlugin) ClassDeclNodeTypes() []string {
	return []string{"class_definition"}
}

// ExtractMetadata extracts Python-specific metadata from a symbol node as a JSON string.
func (p *PythonPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	return extractPythonMetadata(node, source, kind)
}

// ExtractEdge extracts edges from a single Python query pattern match.
//
// Pattern indices map to the order of patterns in queries/python/edges.scm:
// 0 = import statement, 1 = from-import statement, 2 = direct call,
// 3 = attribute/method call, 4 = class inheritance
func (p *PythonPlugin) ExtractEdge(patternIndex uint32, captures map[string]CaptureData, filePath string, enclosingScopeID string) []PluginEdge {
	var edges []PluginEdge
	fromFunction, fromClass := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// Import statement (e.g. `import os`) — always module-level
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

	// From-import statement (e.g. `from os.path import join`)
	case 1:
		importedName, hasImport := captures["imported_name"]
		sourceMod, hasSource := captures["source"]
		if hasImport && hasSource {
			line := importedName.Line
			edges = append(edges, PluginEdge{
				FromID:   fmt.Sprintf("%s::__module__::function", filePath),
				ToID:     fmt.Sprintf("%s::%s", sourceMod.Text, importedName.Text),
				Kind:     "imports",
				FilePath: filePath,
				Line:     &line,
			})
		}

	// Direct function call (e.g. `foo()`)
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

	// Attribute/method call (e.g. `self.foo()`, `obj.bar()`)
	case 3:
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

	// Class inheritance (e.g. `class Foo(Bar):`)
	case 4:
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
	}

	return edges
}

// ExtractDocstring extracts a Python docstring from the first statement in
// a function/class body.
//
// Python docstrings are the first expression_statement child of the body block
// containing a string node. They are NOT comment nodes. This overrides the
// default previous-sibling-comment behavior.
func (p *PythonPlugin) ExtractDocstring(node *sitter.Node, source []byte) string {
	// The node is the inner function_definition or class_definition.
	// Find the body block.
	body := node.ChildByFieldName("body")
	if body == nil {
		return ""
	}

	// Check the first child — docstrings must be the very first statement.
	if body.ChildCount() == 0 {
		return ""
	}

	firstChild := body.Child(0)
	if firstChild == nil || firstChild.Type() != "expression_statement" {
		return ""
	}

	// Look for a string node inside the expression_statement.
	innerCount := int(firstChild.ChildCount())
	for i := 0; i < innerCount; i++ {
		inner := firstChild.Child(i)
		if inner == nil {
			continue
		}
		if inner.Type() == "string" {
			text := inner.Content(source)
			if text != "" {
				// Strip triple-quote delimiters (""" or ''')
				cleaned := text
				cleaned = strings.TrimPrefix(cleaned, "\"\"\"")
				cleaned = strings.TrimPrefix(cleaned, "'''")
				cleaned = strings.TrimSuffix(cleaned, "\"\"\"")
				cleaned = strings.TrimSuffix(cleaned, "'''")
				cleaned = strings.TrimSpace(cleaned)
				if cleaned != "" {
					return cleaned
				}
			}
		}
	}

	return ""
}

// extractPythonMetadata extracts metadata from a Python AST node.
// Returns a JSON string suitable for the metadata column.
func extractPythonMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := PythonMetadata{
		Access:     "public",
		Decorators: []string{},
		Parameters: []PythonParameterInfo{},
	}

	// Check if the parent is a decorated_definition and extract decorators from it.
	parent := node.Parent()
	if parent != nil && parent.Type() == "decorated_definition" {
		parentChildCount := int(parent.ChildCount())
		for i := 0; i < parentChildCount; i++ {
			child := parent.Child(i)
			if child == nil {
				continue
			}
			if child.Type() == "decorator" {
				text := child.Content(source)
				if text != "" {
					// Strip leading `@` and any arguments (e.g., `@decorator(args)` -> `decorator`)
					decName := strings.TrimPrefix(text, "@")
					if parenIdx := strings.Index(decName, "("); parenIdx >= 0 {
						decName = decName[:parenIdx]
					}
					decName = strings.TrimSpace(decName)
					if decName != "" {
						switch decName {
						case "staticmethod":
							meta.IsStatic = true
						case "classmethod":
							meta.IsClassmethod = true
						case "abstractmethod", "abc.abstractmethod":
							meta.IsAbstract = true
						case "property":
							meta.IsProperty = true
						}
						meta.Decorators = append(meta.Decorators, decName)
					}
				}
			}
		}
	}

	// Check for async keyword (async def)
	if node.Type() == "function_definition" {
		childCount := int(node.ChildCount())
		for i := 0; i < childCount; i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			if child.Type() == "async" {
				meta.IsAsync = true
				break
			}
		}
	}

	// Infer access from the symbol name.
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		name := nameNode.Content(source)
		if name != "" {
			meta.Access = inferPythonAccess(name)
		}
	}

	// Extract return type annotation.
	if returnTypeNode := node.ChildByFieldName("return_type"); returnTypeNode != nil {
		text := returnTypeNode.Content(source)
		if text != "" {
			// Strip leading `-> ` from return type annotations.
			clean := strings.TrimSpace(strings.TrimPrefix(text, "->"))
			if clean != "" {
				meta.ReturnType = &clean
			}
		}
	}

	// Extract parameters.
	if kind == "function" || kind == "method" || kind == "class" {
		if paramsNode := node.ChildByFieldName("parameters"); paramsNode != nil {
			meta.Parameters = extractPythonParameters(paramsNode, source)
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("failed to serialize Python metadata: %w", err)
	}
	return string(data), nil
}

// inferPythonAccess infers Python access level from naming conventions.
//
// Name starts with __ and does NOT end with __ -> "name_mangled"
// Name starts with _ -> "private"
// Otherwise -> "public"
func inferPythonAccess(name string) string {
	if strings.HasPrefix(name, "__") && !strings.HasSuffix(name, "__") {
		return "name_mangled"
	}
	if strings.HasPrefix(name, "_") {
		return "private"
	}
	return "public"
}

// extractPythonParameters extracts parameter info from a parameters node.
// Handles identifier, typed_parameter, default_parameter, and typed_default_parameter.
// Skips "self" and "cls" parameters.
func extractPythonParameters(paramsNode *sitter.Node, source []byte) []PythonParameterInfo {
	var params []PythonParameterInfo

	childCount := int(paramsNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		// Regular parameter: bare `name`
		case "identifier":
			name := child.Content(source)
			// Skip self and cls
			if name == "self" || name == "cls" {
				continue
			}
			if name != "" {
				params = append(params, PythonParameterInfo{
					Name:       name,
					HasDefault: false,
				})
			}

		// Typed parameter: `name: type`
		case "typed_parameter":
			var name string
			// Try the "name" field first.
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			} else {
				// Sometimes the name is the first identifier child.
				innerCount := int(child.ChildCount())
				for j := 0; j < innerCount; j++ {
					inner := child.Child(j)
					if inner != nil && inner.Type() == "identifier" {
						name = inner.Content(source)
						break
					}
				}
			}

			// Skip self and cls
			if name == "self" || name == "cls" {
				continue
			}

			var typeAnnotation *string
			if typeNode := child.ChildByFieldName("type"); typeNode != nil {
				text := typeNode.Content(source)
				if text != "" {
					clean := strings.TrimSpace(text)
					typeAnnotation = &clean
				}
			}

			if name != "" {
				params = append(params, PythonParameterInfo{
					Name:           name,
					TypeAnnotation: typeAnnotation,
					HasDefault:     false,
				})
			}

		// Default parameter: `name = value`
		case "default_parameter":
			var name string
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}

			// Skip self and cls
			if name == "self" || name == "cls" {
				continue
			}

			if name != "" {
				params = append(params, PythonParameterInfo{
					Name:       name,
					HasDefault: true,
				})
			}

		// Typed default parameter: `name: type = value`
		case "typed_default_parameter":
			var name string
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name = nameNode.Content(source)
			}

			// Skip self and cls
			if name == "self" || name == "cls" {
				continue
			}

			var typeAnnotation *string
			if typeNode := child.ChildByFieldName("type"); typeNode != nil {
				text := typeNode.Content(source)
				if text != "" {
					clean := strings.TrimSpace(text)
					typeAnnotation = &clean
				}
			}

			if name != "" {
				params = append(params, PythonParameterInfo{
					Name:           name,
					TypeAnnotation: typeAnnotation,
					HasDefault:     true,
				})
			}
		}
	}

	return params
}
