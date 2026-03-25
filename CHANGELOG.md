# Changelog

All notable changes to this project will be documented in this file.

## [1.3.4] - 2026-03-25

### Added
- C++ language support with classes, namespaces, inheritance, member calls (->), scope resolution (::), and new expressions

## [1.3.3] - 2026-03-25

### Added
- C language support with functions, structs, enums, typedefs, and #include imports

## [1.3.2] - 2026-03-25

### Added
- Bash language support with function definitions, source/. imports, and command call detection

## [1.3.1] - 2026-03-25

### Added
- Swift language support with protocol as interface, class/struct/enum declarations, navigation expressions, and inheritance specifiers

## [1.3.0] - 2026-03-25

### Added
- Ruby language support with module kind, mixin edges (include/extend/prepend), attr macros as properties, and singleton methods
- PHP language support with namespace as module, trait as interface, static calls (::), visibility/abstract/final/readonly modifiers
- Lua language support with table-based OOP, require imports, colon-syntax and dot-syntax method calls

## [1.2.0] - 2026-03-25

### Added
- Java language support with full Java 8-21 coverage (classes, interfaces, enums, records, annotations, extends/implements)
- Kotlin language support with data/sealed/inner class modifiers, suspend/inline functions, and delegation specifiers

## [1.1.1] - 2026-03-25

### Added
- Self-update command (`inari update`) with in-place binary replacement from GitHub Releases
- Background update check with 24h cooldown and `--no-update-check` flag
- PowerShell install script (`install.ps1`) for Windows
- CLI parameter update rule enforced across all documentation

### Changed
- Install scripts served from custom domain (inari.hermestech.uk)
- README updated with dual install commands (bash + PowerShell)
- Landing page shows both bash and PowerShell install options

### Fixed
- Install script updated to match Go release binary naming (GOOS-GOARCH)

## [1.1.0] - 2026-03-25

### Added
- Go language support with tree-sitter parsing, exported/unexported detection, receiver types, and composite literal instantiation
- Custom domain landing page at inari.hermestech.uk
- ROADMAP.md as standalone file
- Multi-agent integration table in README (Claude Code, Cursor, Copilot, Gemini CLI, Codex, Aider, Windsurf)

### Changed
- README rewritten with professional structure, categorised command tables, and live CI badge
- Landing page copy updated to developer-focused messaging
- CLAUDE.md.snippet expanded with `similar` and `source` commands

### Fixed
- Clipboard copy button now works on Firefox and non-HTTPS contexts via execCommand fallback

## [1.0.0] - 2026-03-25

### Added
- Go implementation ported from rynhardt-engelbrecht/scope (Rust)
- 18 CLI commands: init, index, sketch, refs, callers, deps, rdeps, impact, find, similar, source, trace, entrypoints, map, status, workspace (init/list/index)
- 4 language plugins: TypeScript, C#, Python, Rust
- SQLite dependency graph with FTS5 full-text search (BM25 ranking)
- Full and incremental indexing with SHA-256 change detection
- File watcher with 300ms debounce for auto re-indexing
- Multi-project workspace support with federated queries
- Benchmark runner with 3-arm experiment design
- JSON output on all commands via --json flag
- Fully implemented rdeps, similar, and source commands (stubs in original)
