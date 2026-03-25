// Shared helper functions for Inari CLI commands.
package commands

import "strings"

// sourceExtensions lists file extensions for supported (and planned) languages.
// Keep in sync with internal/languages plugin registrations.
var sourceExtensions = []string{
	".ts", ".tsx", ".js", ".jsx", // TypeScript / JavaScript
	".cs",   // C#
	".rs",   // Rust
	".py",   // Python
	".go",   // Go
	".java", // Java
	".kt",   // Kotlin
}

// LooksLikeFilePath returns true if the input looks like a file path
// rather than a symbol name.
//
// Checks for path separators and common source file extensions.
// Used by commands like sketch, refs, and deps to branch between
// symbol-based and file-based queries.
func LooksLikeFilePath(input string) bool {
	if strings.Contains(input, "/") || strings.Contains(input, "\\") {
		return true
	}
	for _, ext := range sourceExtensions {
		if strings.HasSuffix(input, ext) {
			return true
		}
	}
	return false
}
