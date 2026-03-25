// Parser provides tree-sitter parsing and symbol/edge extraction.
//
// Uses tree-sitter queries stored in queries/<language>/ to extract
// symbol definitions and relationships from source code.
//
// Language-specific logic is provided by LanguagePlugin implementations
// in internal/languages/. Adding a new language requires only implementing
// the interface and registering it via CodeParser.RegisterPlugin.
package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/KilimcininKorOglu/inari/internal/languages"
)

// pluginEntry holds a registered language plugin with its compiled queries.
type pluginEntry struct {
	// plugin is the language plugin implementation.
	plugin languages.LanguagePlugin
	// symbolQuery is the compiled query for extracting symbol definitions.
	symbolQuery *sitter.Query
	// edgeQuery is the compiled query for extracting edges (calls, imports, etc.).
	edgeQuery *sitter.Query
}

// CodeParser uses tree-sitter to extract symbols and edges from source code.
type CodeParser struct {
	parser  *sitter.Parser
	plugins []pluginEntry
}

// NewCodeParser creates a new parser ready for plugin registration.
// After creation, call RegisterPlugin to add language support.
func NewCodeParser() (*CodeParser, error) {
	parser := sitter.NewParser()
	return &CodeParser{
		parser:  parser,
		plugins: make([]pluginEntry, 0),
	}, nil
}

// RegisterPlugin compiles the symbol and edge queries for the given plugin
// and registers it with the parser. Returns an error if query compilation fails.
func (cp *CodeParser) RegisterPlugin(plugin languages.LanguagePlugin) error {
	tsLang := plugin.TSLanguage()
	langName := plugin.Language().String()

	symbolQuery, err := sitter.NewQuery([]byte(plugin.SymbolQuerySource()), tsLang)
	if err != nil {
		return fmt.Errorf("failed to compile %s symbol query: %w", langName, err)
	}

	edgeQuery, err := sitter.NewQuery([]byte(plugin.EdgeQuerySource()), tsLang)
	if err != nil {
		return fmt.Errorf("failed to compile %s edge query: %w", langName, err)
	}

	cp.plugins = append(cp.plugins, pluginEntry{
		plugin:      plugin,
		symbolQuery: symbolQuery,
		edgeQuery:   edgeQuery,
	})

	return nil
}

// findPlugin returns the plugin entry for the given language, or nil if not found.
func (cp *CodeParser) findPlugin(lang languages.SupportedLanguage) *pluginEntry {
	for i := range cp.plugins {
		if cp.plugins[i].plugin.Language() == lang {
			return &cp.plugins[i]
		}
	}
	return nil
}

// findPluginByExtension returns the plugin entry matching the given file extension,
// or nil if no plugin handles that extension.
func (cp *CodeParser) findPluginByExtension(ext string) *pluginEntry {
	for i := range cp.plugins {
		for _, pluginExt := range cp.plugins[i].plugin.Extensions() {
			if pluginExt == ext {
				return &cp.plugins[i]
			}
		}
	}
	return nil
}

// DetectLanguage detects the programming language of a file based on its extension.
func (cp *CodeParser) DetectLanguage(path string) (languages.SupportedLanguage, error) {
	ext := filepath.Ext(path)
	if ext == "" {
		return 0, fmt.Errorf("no file extension: %s", path)
	}
	// Remove the leading dot from the extension.
	ext = ext[1:]

	switch ext {
	case "ts", "tsx":
		return languages.TypeScript, nil
	case "cs":
		return languages.CSharp, nil
	case "py":
		return languages.Python, nil
	case "go":
		return languages.Go, nil
	case "java":
		return languages.Java, nil
	case "kt":
		return languages.Kotlin, nil
	case "rs":
		return languages.Rust, nil
	case "rb":
		return languages.Ruby, nil
	default:
		return 0, fmt.Errorf("unsupported file extension: .%s", ext)
	}
}

// IsSupported checks if a file extension is supported for parsing (has a loaded grammar).
func (cp *CodeParser) IsSupported(path string) bool {
	ext := filepath.Ext(path)
	if ext == "" {
		return false
	}
	// Remove the leading dot.
	ext = ext[1:]
	return cp.findPluginByExtension(ext) != nil
}

// ExtensionsFor returns the file extensions supported by the given language.
// Returns nil if the language plugin is not registered.
func (cp *CodeParser) ExtensionsFor(lang languages.SupportedLanguage) []string {
	entry := cp.findPlugin(lang)
	if entry == nil {
		return nil
	}
	return entry.plugin.Extensions()
}

// ExtractSymbols extracts symbol definitions from a source file.
func (cp *CodeParser) ExtractSymbols(filePath string, source []byte, lang languages.SupportedLanguage) ([]Symbol, error) {
	entry := cp.findPlugin(lang)
	if entry == nil {
		return nil, fmt.Errorf("language %q not loaded", lang.String())
	}

	tsLang := entry.plugin.TSLanguage()
	cp.parser.SetLanguage(tsLang)

	tree, err := cp.parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil, fmt.Errorf("parse failed for %s: %w", filePath, err)
	}
	if tree == nil {
		return nil, fmt.Errorf("parse failed for %s: tree is nil", filePath)
	}
	defer tree.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(entry.symbolQuery, tree.RootNode())

	var symbols []Symbol

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		var nameText string
		var defNode *sitter.Node
		var hasName, hasDef bool

		for _, capture := range match.Captures {
			captureName := entry.symbolQuery.CaptureNameForId(capture.Index)
			text := parserNodeText(capture.Node, source)

			switch captureName {
			case "name":
				nameText = text
				// Strip leading ':' from Ruby symbol literals (e.g. :name from attr_accessor).
				if len(nameText) > 1 && nameText[0] == ':' {
					nameText = nameText[1:]
				}
				hasName = true
			case "definition":
				defNode = capture.Node
				hasDef = true
			case "params":
				// Captured for potential future use; not stored directly.
			case "return_type":
				// Captured for potential future use; not stored directly.
			}
		}

		if !hasName || !hasDef {
			continue
		}

		kind := entry.plugin.InferSymbolKind(defNode.Type())
		line := defNode.StartPoint().Row + 1 // Convert 0-based to 1-based.
		id := fmt.Sprintf("%s::%s::%s::%d", filePath, nameText, kind, line)

		// Extract metadata using language-specific logic.
		metadata, err := entry.plugin.ExtractMetadata(defNode, source, kind)
		if err != nil {
			return nil, fmt.Errorf("failed to extract metadata for %s: %w", id, err)
		}

		// Extract signature — first line of the definition up to `{` or end of line.
		signature := extractSignature(defNode, source)

		// Extract docstring — delegates to language plugin.
		docstring := entry.plugin.ExtractDocstring(defNode, source)

		// Determine parent_id for methods inside classes.
		var parentID *string
		if kind == "method" || kind == "property" {
			parentID = findParentClass(defNode, source, filePath, entry.plugin)
		}

		lineStart := defNode.StartPoint().Row + 1
		lineEnd := defNode.EndPoint().Row + 1

		var sigPtr *string
		if signature != "" {
			sigPtr = &signature
		}

		var docPtr *string
		if docstring != "" {
			docPtr = &docstring
		}

		symbols = append(symbols, Symbol{
			ID:        id,
			Name:      nameText,
			Kind:      kind,
			FilePath:  filePath,
			LineStart: lineStart,
			LineEnd:   lineEnd,
			Signature: sigPtr,
			Docstring: docPtr,
			ParentID:  parentID,
			Language:  lang.AsStr(),
			Metadata:  metadata,
		})
	}

	return symbols, nil
}

// ExtractEdges extracts edges (relationships) from a source file.
func (cp *CodeParser) ExtractEdges(filePath string, source []byte, lang languages.SupportedLanguage) ([]Edge, error) {
	entry := cp.findPlugin(lang)
	if entry == nil {
		return nil, fmt.Errorf("language %q not loaded", lang.String())
	}

	tsLang := entry.plugin.TSLanguage()
	cp.parser.SetLanguage(tsLang)

	tree, err := cp.parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil, fmt.Errorf("parse failed for %s: %w", filePath, err)
	}
	if tree == nil {
		return nil, fmt.Errorf("parse failed for %s: tree is nil", filePath)
	}
	defer tree.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(entry.edgeQuery, tree.RootNode())

	var edges []Edge

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		pattern := uint32(match.PatternIndex)
		capturesMap := make(map[string]languages.CaptureData)
		var representativeNode *sitter.Node

		for _, capture := range match.Captures {
			captureName := entry.edgeQuery.CaptureNameForId(capture.Index)
			text := parserNodeText(capture.Node, source)
			line := capture.Node.StartPoint().Row + 1 // 1-based.
			if representativeNode == nil {
				representativeNode = capture.Node
			}
			capturesMap[captureName] = languages.CaptureData{
				Text: text,
				Line: line,
			}
		}

		// Resolve the enclosing scope for this match.
		enclosingScopeID := ""
		if representativeNode != nil {
			if scopeID := findEnclosingScope(representativeNode, source, filePath, entry.plugin); scopeID != "" {
				enclosingScopeID = scopeID
			}
		}

		extracted := entry.plugin.ExtractEdge(pattern, capturesMap, filePath, enclosingScopeID)
		for _, pe := range extracted {
			edges = append(edges, Edge{
				FromID:   pe.FromID,
				ToID:     pe.ToID,
				Kind:     pe.Kind,
				FilePath: pe.FilePath,
				Line:     pe.Line,
			})
		}
	}

	return edges, nil
}

// extractSignature extracts the signature from a definition node.
// It returns the first line of the definition up to `{` or end of the line.
func extractSignature(node *sitter.Node, source []byte) string {
	start := node.StartByte()
	end := node.EndByte()
	if start >= end || int(end) > len(source) {
		return ""
	}
	text := string(source[start:end])

	// Take up to the first `{` or newline, whichever comes first.
	var sig string
	bracePos := strings.Index(text, "{")
	nlPos := strings.Index(text, "\n")

	if bracePos >= 0 && (nlPos < 0 || bracePos < nlPos) {
		sig = strings.TrimSpace(text[:bracePos])
	} else if nlPos >= 0 {
		sig = strings.TrimSpace(text[:nlPos])
	} else {
		sig = strings.TrimSpace(text)
	}

	if sig == "" {
		return ""
	}
	return sig
}

// findEnclosingScope walks up the AST from node to find the nearest enclosing
// scope (function, method, class). Returns the symbol ID of that scope,
// or an empty string if at module level.
func findEnclosingScope(node *sitter.Node, source []byte, filePath string, plugin languages.LanguagePlugin) string {
	current := node.Parent()
	scopeTypes := plugin.ScopeNodeTypes()

	for current != nil {
		nodeType := current.Type()
		if containsString(scopeTypes, nodeType) {
			// For arrow functions / function expressions assigned to variables,
			// walk up to the variable_declarator to get a meaningful name.
			if nodeType == "arrow_function" || nodeType == "function_expression" {
				grandparent := current.Parent()
				if grandparent != nil && grandparent.Type() == "variable_declarator" {
					nameNode := grandparent.ChildByFieldName("name")
					if nameNode != nil {
						name := parserNodeText(nameNode, source)
						if name != "" {
							line := grandparent.StartPoint().Row + 1
							return fmt.Sprintf("%s::%s::function::%d", filePath, name, line)
						}
					}
				}
				// If we can't get a name from variable_declarator, keep walking up.
				current = current.Parent()
				continue
			}

			// Named scope — get its name and build the ID.
			nameNode := current.ChildByFieldName("name")
			if nameNode != nil {
				name := parserNodeText(nameNode, source)
				if name != "" {
					kind := plugin.InferSymbolKind(current.Type())
					line := current.StartPoint().Row + 1
					return fmt.Sprintf("%s::%s::%s::%d", filePath, name, kind, line)
				}
			}
		}
		current = current.Parent()
	}

	return "" // Module level — no enclosing scope.
}

// findParentClass finds the parent class for a method or property node.
// Returns the symbol ID of the parent class, or nil if not inside a class.
func findParentClass(node *sitter.Node, source []byte, filePath string, plugin languages.LanguagePlugin) *string {
	classBodyNodes := plugin.ClassBodyNodeTypes()
	classDeclNodes := plugin.ClassDeclNodeTypes()

	current := node.Parent()
	for current != nil {
		if containsString(classBodyNodes, current.Type()) {
			classNode := current.Parent()
			if classNode != nil && containsString(classDeclNodes, classNode.Type()) {
				nameNode := classNode.ChildByFieldName("name")
				if nameNode != nil {
					className := parserNodeText(nameNode, source)
					if className != "" {
						kind := plugin.InferSymbolKind(classNode.Type())
						classLine := classNode.StartPoint().Row + 1
						id := fmt.Sprintf("%s::%s::%s::%d", filePath, className, kind, classLine)
						return &id
					}
				}
			}
		}
		current = current.Parent()
	}
	return nil
}

// parserNodeText safely extracts the text content of a tree-sitter node.
// Named with "parser" prefix to avoid collision with nodeText in graph.go.
func parserNodeText(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}
	return node.Content(source)
}
