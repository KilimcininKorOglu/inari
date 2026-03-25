// Protocol Buffers language plugin for Inari.
//
// Indexes message, enum, service, and rpc definitions from .proto files.
// Field and enum value symbols are captured as property and const kinds.
// Edge detection covers imports, field type references, and rpc type references.
package languages

import (
	"encoding/json"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/protobuf"

	"github.com/KilimcininKorOglu/inari/queries"
)

// ProtobufPlugin implements LanguagePlugin for Protocol Buffers (.proto) files.
type ProtobufPlugin struct{}

// ProtobufMetadata holds structured metadata for a protobuf symbol.
type ProtobufMetadata struct {
	FieldNumber *int    `json:"fieldNumber,omitempty"`
	FieldType   string  `json:"fieldType,omitempty"`
	IsRepeated  bool    `json:"isRepeated,omitempty"`
	IsStream    bool    `json:"isStream,omitempty"`
	RequestType string  `json:"requestType,omitempty"`
	ReturnType  *string `json:"returnType,omitempty"`
}

func (p *ProtobufPlugin) Language() SupportedLanguage { return Protobuf }

func (p *ProtobufPlugin) Extensions() []string { return []string{"proto"} }

func (p *ProtobufPlugin) TSLanguage() *sitter.Language { return protobuf.GetLanguage() }

func (p *ProtobufPlugin) SymbolQuerySource() string { return queries.ProtobufSymbolsQuery }

func (p *ProtobufPlugin) EdgeQuerySource() string { return queries.ProtobufEdgesQuery }

func (p *ProtobufPlugin) InferSymbolKind(nodeKind string) string {
	switch nodeKind {
	case "message":
		return "class"
	case "enum":
		return "enum"
	case "service":
		return "interface"
	case "rpc":
		return "method"
	case "enum_field":
		return "const"
	case "field", "map_field":
		return "property"
	default:
		return "function"
	}
}

func (p *ProtobufPlugin) ScopeNodeTypes() []string {
	return []string{"message", "service", "enum"}
}

func (p *ProtobufPlugin) ClassBodyNodeTypes() []string {
	return []string{"message_body"}
}

func (p *ProtobufPlugin) ClassDeclNodeTypes() []string {
	return []string{"message", "service"}
}

func (p *ProtobufPlugin) ExtractMetadata(node *sitter.Node, source []byte, kind string) (string, error) {
	meta := ProtobufMetadata{}

	switch node.Type() {
	case "field":
		// Extract field type and number.
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			switch child.Type() {
			case "type":
				meta.FieldType = child.Content(source)
			case "field_number":
				num := parseFieldNumber(child, source)
				meta.FieldNumber = &num
			}
			// Check for "repeated" keyword.
			if !child.IsNamed() && child.Content(source) == "repeated" {
				meta.IsRepeated = true
			}
		}

	case "map_field":
		meta.FieldType = "map"
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "field_number" {
				num := parseFieldNumber(child, source)
				meta.FieldNumber = &num
			}
		}

	case "rpc":
		// Extract request and response types.
		msgTypes := collectNamedChildrenOfType(node, "message_or_enum_type", source)
		if len(msgTypes) >= 1 {
			meta.RequestType = msgTypes[0]
		}
		if len(msgTypes) >= 2 {
			rt := msgTypes[1]
			meta.ReturnType = &rt
		}
		// Check for stream keyword.
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if !child.IsNamed() && child.Content(source) == "stream" {
				meta.IsStream = true
				break
			}
		}

	case "enum_field":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "int_lit" {
				num := parseIntLit(child, source)
				meta.FieldNumber = &num
				break
			}
		}
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return "{}", nil
	}
	return string(data), nil
}

func (p *ProtobufPlugin) ExtractEdge(
	patternIndex uint32,
	captures map[string]CaptureData,
	filePath string,
	enclosingScopeID string,
) []PluginEdge {
	var edges []PluginEdge
	fromFunction, _ := ResolveEdgeScope(enclosingScopeID, filePath)

	switch patternIndex {
	// 0: import "file.proto"
	case 0:
		if source, ok := captures["source"]; ok {
			importPath := strings.Trim(source.Text, "\"")
			if importPath != "" {
				edges = append(edges, NewEdge(
					fmt.Sprintf("%s::__module__::function", filePath),
					importPath,
					"imports",
					filePath,
					source.Line,
				))
			}
		}

	// 1, 2, 3, 4: type references (field, rpc, oneof, map)
	case 1, 2, 3, 4:
		if typeRef, ok := captures["type_ref"]; ok {
			// For qualified types like google.protobuf.Timestamp,
			// extract the last identifier.
			typeName := typeRef.Text
			if parts := strings.Split(typeName, "."); len(parts) > 1 {
				typeName = parts[len(parts)-1]
			}
			edges = append(edges, NewEdge(
				fromFunction,
				typeName,
				"references_type",
				filePath,
				typeRef.Line,
			))
		}
	}

	return edges
}

func (p *ProtobufPlugin) ExtractDocstring(_ *sitter.Node, _ []byte) string {
	return ""
}

// parseFieldNumber extracts the integer value from a field_number node.
func parseFieldNumber(node *sitter.Node, source []byte) int {
	text := node.Content(source)
	var n int
	fmt.Sscanf(text, "%d", &n)
	return n
}

// parseIntLit extracts the integer value from an int_lit node.
func parseIntLit(node *sitter.Node, source []byte) int {
	text := node.Content(source)
	var n int
	fmt.Sscanf(text, "%d", &n)
	return n
}

// collectNamedChildrenOfType collects text content of all named children
// with the given type from a node.
func collectNamedChildrenOfType(node *sitter.Node, nodeType string, source []byte) []string {
	var results []string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.IsNamed() && child.Type() == nodeType {
			results = append(results, child.Content(source))
		}
	}
	return results
}
