# Changelog

All notable changes to this project will be documented in this file.

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
