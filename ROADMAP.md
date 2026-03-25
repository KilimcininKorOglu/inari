# Roadmap

## Shipped

- [x] TypeScript, C#, Python, Rust, Go, Java, Kotlin symbol extraction with edge detection
- [x] SQLite dependency graph with recursive impact traversal
- [x] Full-text search with FTS5, BM25 ranking, importance-tier boosting
- [x] 19 commands including rdeps, similar, source (fully implemented)
- [x] `inari update` -- self-update from GitHub Releases with background update checks
- [x] `inari index --watch` -- auto re-index on file changes
- [x] Multi-project workspaces with federated queries
- [x] `LanguagePlugin` interface for pluggable language support
- [x] `--json` output on all commands

## Next

- [ ] Vector embeddings via local ONNX model
- [ ] Cross-project edge detection via `inari link`
- [ ] MCP adapter

## Later

- [ ] Ruby language support
- [ ] CI/CD integration for impact analysis on PRs
