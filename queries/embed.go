// Package queries embeds all tree-sitter .scm query files for supported languages.
//
// Each language has two query files:
//   - symbols.scm: extracts symbol definitions (functions, classes, methods, etc.)
//   - edges.scm: extracts relationships (calls, imports, extends, implements, etc.)
//
// These are the same query files used by the Rust implementation and must be
// kept in sync.
package queries

import _ "embed"

// TypeScript queries

//go:embed typescript/symbols.scm
var TypeScriptSymbolsQuery string

//go:embed typescript/edges.scm
var TypeScriptEdgesQuery string

// C# queries

//go:embed csharp/symbols.scm
var CSharpSymbolsQuery string

//go:embed csharp/edges.scm
var CSharpEdgesQuery string

// Python queries

//go:embed python/symbols.scm
var PythonSymbolsQuery string

//go:embed python/edges.scm
var PythonEdgesQuery string

// Rust queries

//go:embed rust/symbols.scm
var RustSymbolsQuery string

//go:embed rust/edges.scm
var RustEdgesQuery string

// Go queries

//go:embed go/symbols.scm
var GoSymbolsQuery string

//go:embed go/edges.scm
var GoEdgesQuery string

// Java queries

//go:embed java/symbols.scm
var JavaSymbolsQuery string

//go:embed java/edges.scm
var JavaEdgesQuery string
