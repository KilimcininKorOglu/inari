// Package errors defines typed error values for Inari operations.
//
// Use these sentinel errors for typed matching (errors.Is). Use fmt.Errorf
// with %w wrapping for application-level error propagation.
package errors

import "fmt"

// IndexNotFound indicates the .inari/ directory or graph.db was not found.
type IndexNotFound struct{}

func (e *IndexNotFound) Error() string {
	return "No index found. Run 'inari init' to initialise, then 'inari index' to build the index."
}

// SymbolNotFound indicates a symbol name was not found in the index.
type SymbolNotFound struct {
	Name string
}

func (e *SymbolNotFound) Error() string {
	return fmt.Sprintf("Symbol '%s' not found in index. Run 'inari index' if this is a new file.", e.Name)
}

// ParseError indicates tree-sitter failed to parse a source file.
type ParseError struct {
	FilePath string
	Reason   string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("Failed to parse %s: %s", e.FilePath, e.Reason)
}

// StorageError indicates a SQLite operation failed.
type StorageError struct {
	Message string
}

func (e *StorageError) Error() string {
	return fmt.Sprintf("Storage error: %s", e.Message)
}

// ConfigError indicates a configuration file is missing or malformed.
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("Configuration error: %s", e.Message)
}

// UnsupportedLanguage indicates a language is not yet supported by Inari.
type UnsupportedLanguage struct {
	Language string
}

func (e *UnsupportedLanguage) Error() string {
	return fmt.Sprintf("Unsupported language: %s", e.Language)
}
