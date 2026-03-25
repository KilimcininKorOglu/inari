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

// Kotlin queries

//go:embed kotlin/symbols.scm
var KotlinSymbolsQuery string

//go:embed kotlin/edges.scm
var KotlinEdgesQuery string

// Ruby queries

//go:embed ruby/symbols.scm
var RubySymbolsQuery string

//go:embed ruby/edges.scm
var RubyEdgesQuery string

// PHP queries

//go:embed php/symbols.scm
var PhpSymbolsQuery string

//go:embed php/edges.scm
var PhpEdgesQuery string

// Lua queries

//go:embed lua/symbols.scm
var LuaSymbolsQuery string

//go:embed lua/edges.scm
var LuaEdgesQuery string

// Swift queries

//go:embed swift/symbols.scm
var SwiftSymbolsQuery string

//go:embed swift/edges.scm
var SwiftEdgesQuery string

// Bash queries

//go:embed bash/symbols.scm
var BashSymbolsQuery string

//go:embed bash/edges.scm
var BashEdgesQuery string

// C queries

//go:embed c/symbols.scm
var CSymbolsQuery string

//go:embed c/edges.scm
var CEdgesQuery string
