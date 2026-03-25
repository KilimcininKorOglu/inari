// Text construction for symbol search indexing.
//
// Builds rich text representations of symbols for full-text search.
// The text combines symbol kind, name, signature, docstring, and parent
// context, ordered by importance. This is what gets indexed in the FTS5
// table and searched against user queries.
package core

import (
	"strings"
	"unicode"
)

// BuildEmbeddingText builds the searchable text representation of a symbol.
//
// Combines kind, name, signature, docstring, and parent context into a
// single string optimised for full-text search. More important fields
// appear first so that FTS5's BM25 ranking naturally weights them higher.
//
// Parts are joined by " | ".
func BuildEmbeddingText(symbol *Symbol, callers []string, callees []string, importance float64) string {
	var parts []string

	// Kind and name: "method processPayment"
	parts = append(parts, symbol.Kind+" "+symbol.Name)

	// Split camelCase/PascalCase name into separate words for better matching.
	// e.g. "processPayment" -> "process Payment" so searching "payment" works.
	splitName := SplitCamelCase(symbol.Name)
	if splitName != symbol.Name {
		parts = append(parts, splitName)
	}

	// Split snake_case names for better matching.
	snakeSplit := SplitSnakeCase(symbol.Name)
	if snakeSplit != symbol.Name {
		parts = append(parts, snakeSplit)
	}

	// Signature if available.
	if symbol.Signature != nil && *symbol.Signature != "" {
		parts = append(parts, *symbol.Signature)
	}

	// Docstring if available.
	if symbol.Docstring != nil && *symbol.Docstring != "" {
		parts = append(parts, *symbol.Docstring)
	}

	// Parent context from parent_id (extract class name from ID format "file::ClassName::class").
	if symbol.ParentID != nil && *symbol.ParentID != "" {
		parentName := extractNameFromID(*symbol.ParentID)
		if parentName != "" {
			parts = append(parts, "in "+parentName)
			// Also split parent name for search.
			splitParent := SplitCamelCase(parentName)
			if splitParent != parentName {
				parts = append(parts, "in "+splitParent)
			}
		}
	}

	// Add file path segments for contextual search.
	// "src/payments/services/PaymentService.ts" -> "payments services"
	pathSegments := strings.Split(symbol.FilePath, "/")
	var filteredSegments []string
	for _, seg := range pathSegments {
		if seg != "src" && !strings.Contains(seg, ".") {
			filteredSegments = append(filteredSegments, seg)
		}
	}
	if len(filteredSegments) > 0 {
		parts = append(parts, "path "+strings.Join(filteredSegments, " "))
	}

	// Add caller names for relationship-based search.
	if len(callers) > 0 {
		parts = append(parts, "called-by "+strings.Join(callers, " "))
	}

	// Add callee names for relationship-based search.
	if len(callees) > 0 {
		parts = append(parts, "calls "+strings.Join(callees, " "))
	}

	// Add importance tier for search boosting.
	if importance > 0.7 {
		parts = append(parts, "importance high core")
	} else if importance > 0.3 {
		parts = append(parts, "importance medium")
	}

	return strings.Join(parts, " | ")
}

// SplitCamelCase splits a camelCase or PascalCase identifier into space-separated words.
//
// Examples:
//   - "processPayment" -> "process Payment"
//   - "PaymentService" -> "Payment Service"
//   - "getHTTPResponse" -> "get HTTP Response"
//   - "login" -> "login" (no change)
func SplitCamelCase(name string) string {
	runes := []rune(name)
	var result strings.Builder
	result.Grow(len(name) + 4)

	for i, ch := range runes {
		if i > 0 && unicode.IsUpper(ch) {
			prevLower := unicode.IsLower(runes[i-1])
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if prevLower || (nextLower && unicode.IsUpper(runes[i-1])) {
				result.WriteRune(' ')
			}
		}
		result.WriteRune(ch)
	}

	return result.String()
}

// SplitSnakeCase splits a snake_case identifier into space-separated words.
//
// Examples:
//   - "payment_retry_worker" -> "payment retry worker"
//   - "API_KEY" -> "API KEY"
//   - "login" -> "login" (no change)
func SplitSnakeCase(name string) string {
	return strings.ReplaceAll(name, "_", " ")
}

// extractNameFromID extracts the symbol name portion from a symbol ID.
//
// Symbol IDs follow the format "file_path::name::kind". This extracts
// the name part. Returns empty string if the ID doesn't match the expected format.
func extractNameFromID(id string) string {
	parts := strings.Split(id, "::")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return ""
}
