// SQL language plugin for Inari.
//
// Indexes CREATE TABLE, CREATE VIEW, CREATE FUNCTION definitions and their
// column definitions. Edge detection covers REFERENCES (FK), FROM/JOIN table
// references, and function invocations.
package languages

import (
	"encoding/json"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/sql"

	"github.com/KilimcininKorOglu/inari/queries"
)

// SQLPlugin implements LanguagePlugin for SQL (.sql) files.
type SQLPlugin struct{}

// SQLMetadata holds structured metadata for a SQL symbol.
type SQLMetadata struct {
	ColumnType string  `json:"columnType,omitempty"`
	NotNull    bool    `json:"notNull,omitempty"`
	PrimaryKey bool    `json:"primaryKey,omitempty"`
	HasDefault bool    `json:"hasDefault,omitempty"`
	Language   string  `json:"language,omitempty"`
	ReturnType *string `json:"returnType,omitempty"`
}

func (p *SQLPlugin) Language() SupportedLanguage { return SQL }

func (p *SQLPlugin) Extensions() []string { return []string{"sql"} }

func (p *SQLPlugin) TSLanguage() *sitter.Language { return sql.GetLanguage() }

func (p *SQLPlugin) SymbolQuerySource() string { return queries.SQLSymbolsQuery }

func (p *SQLPlugin) EdgeQuerySource() string { return queries.SQLEdgesQuery }

func (p *SQLPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "create_table":
		return "class"
	case "create_view":
		return "class"
	case "create_function":
		return "function"
	case "column_definition":
		return "property"
	default:
		return "function"
	}
}

func (p *SQLPlugin) ScopeNodeTypes() []string {
	return []string{"create_table", "create_view", "create_function"}
}

func (p *SQLPlugin) ClassBodyNodeTypes() []string {
	return []string{"column_definitions"}
}

func (p *SQLPlugin) ClassDeclNodeTypes() []string {
	return []string{"create_table", "create_view"}
}

func (p *SQLPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := SQLMetadata{}

	switch node.Type() {
	case "column_definition":
		// Extract column type.
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.IsNamed() {
				switch child.Type() {
				case "keyword_not":
					// Next sibling should be keyword_null.
					meta.NotNull = true
				case "keyword_primary":
					meta.PrimaryKey = true
				case "keyword_default":
					meta.HasDefault = true
				default:
					// Type nodes: int, varchar, decimal, timestamp, etc.
					if meta.ColumnType == "" && child.Type() != "identifier" &&
						child.Type() != "keyword_not" && child.Type() != "keyword_null" &&
						child.Type() != "keyword_primary" && child.Type() != "keyword_key" &&
						child.Type() != "keyword_default" && child.Type() != "keyword_references" &&
						child.Type() != "object_reference" && child.Type() != "literal" {
						meta.ColumnType = child.Content(source)
					}
				}
			}
		}

	case "create_function":
		// Extract function language.
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "function_language" {
				for j := 0; j < int(child.ChildCount()); j++ {
					langChild := child.Child(j)
					if langChild.Type() == "identifier" {
						meta.Language = langChild.Content(source)
					}
				}
			}
			// Extract return type.
			if child.Type() == "keyword_returns" {
				// Next named sibling is the return type.
				for k := i + 1; k < int(node.ChildCount()); k++ {
					next := node.Child(k)
					if next.IsNamed() && next.Type() != "function_arguments" &&
						next.Type() != "function_body" && next.Type() != "function_language" {
						rt := next.Content(source)
						meta.ReturnType = &rt
						break
					}
				}
			}
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "{}", nil
	}
	return string(data), nil
}

func (p *SQLPlugin) ExtractEdge(
	patternIndex uint32,
	captures map[string]CaptureData,
	filePath string,
	enclosingScopeID string,
) []PluginEdge {
	var edges []PluginEdge
	fromFunction, _ := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// 0: REFERENCES table — foreign key
	case 0:
		if ref, ok := captures["ref_table"]; ok {
			edges = append(edges, NewEdge(
				fromFunction,
				ref.Text,
				"references_type",
				filePath,
				ref.Line,
			))
		}

	// 1: FROM table
	case 1:
		if ref, ok := captures["ref_table"]; ok {
			edges = append(edges, NewEdge(
				fromFunction,
				ref.Text,
				"references_type",
				filePath,
				ref.Line,
			))
		}

	// 2: JOIN table
	case 2:
		if ref, ok := captures["ref_table"]; ok {
			edges = append(edges, NewEdge(
				fromFunction,
				ref.Text,
				"references_type",
				filePath,
				ref.Line,
			))
		}

	// 3: Function invocation
	case 3:
		if fn, ok := captures["func_name"]; ok {
			name := fn.Text
			// Skip SQL built-in aggregate functions.
			upper := strings.ToUpper(name)
			if upper == "SUM" || upper == "COUNT" || upper == "AVG" ||
				upper == "MIN" || upper == "MAX" || upper == "COALESCE" ||
				upper == "NULLIF" || upper == "CAST" {
				break
			}
			edges = append(edges, NewEdge(
				fromFunction,
				name,
				"calls",
				filePath,
				fn.Line,
			))
		}
	}

	return edges
}

func (p *SQLPlugin) ExtractDocstring(_ *sitter.Node, _ []byte) string {
	return ""
}

// Compile-time check that SQLPlugin implements LanguagePlugin.
var _ LanguagePlugin = (*SQLPlugin)(nil)
