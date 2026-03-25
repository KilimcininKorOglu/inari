```
+-------------------------------------------------------+
|                                                       |
|   ██╗███╗   ██╗ █████╗ ██████╗ ██╗                    |
|   ██║████╗  ██║██╔══██╗██╔══██╗██║                    |
|   ██║██╔██╗ ██║███████║██████╔╝██║                    |
|   ██║██║╚██╗██║██╔══██║██╔══██╗██║                    |
|   ██║██║ ╚████║██║  ██║██║  ██║██║                    |
|   ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝                    |
|                                                       |
|   Code intelligence for LLM coding agents.            |
|   Know before you touch.                              |
|                                                       |
+-------------------------------------------------------+
```

[![Go](https://img.shields.io/badge/built_with-Go-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-v1.0.0-blue.svg)](https://github.com/KilimcininKorOglu/inari/releases)
[![Build](https://img.shields.io/badge/build-passing-22863a)](https://github.com/KilimcininKorOglu/inari/actions)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](#installation)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

---

## Table of contents

- [What it does](#what-it-does)
- [Supported languages](#supported-languages)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Commands](#commands)
- [Watch mode](#watch-mode)
- [Workspaces](#workspaces)
- [How it works](#how-it-works)
- [Configuration](#configuration)
- [CLAUDE.md integration](#claudemd-integration)
- [Building from source](#building-from-source)
- [Roadmap](#roadmap)
- [License](#license)

---

## What it does

Inari builds a local code intelligence index for any codebase and exposes it through a CLI designed for LLM coding agents. Before an agent edits a function, it can run `inari sketch PaymentService` and get back the class structure, method signatures, caller counts, and dependency surface in approximately 180 tokens -- without reading the 6,000-token source file.

The index is built from tree-sitter AST parsing (fast, error-tolerant, no compiler required), stored in a SQLite dependency graph with FTS5 full-text search, and queried through commands that return structured, agent-readable output. Everything lives in a `.inari/` directory in your project root. No server process, no Docker, no API key required.

Inari integrates with Claude Code, Cursor, Aider, and any other agent that can run a shell command. Add the provided CLAUDE.md snippet to your project and agents will use it automatically.

```
$ inari sketch PaymentService

PaymentService                                    class  src/payments/service.ts:12-89
─────────────────────────────────────────────────────────────────────────────────────
deps:      StripeClient, UserRepository, Logger, PaymentConfig
extends:   BaseService
implements: IPaymentService

methods:
  async  processPayment  (amount: Decimal, userId: string) → Promise<PaymentResult>   [11 callers]
         refundPayment   (txId: string, reason?: string)   → Promise<bool>             [3 callers]
  private validateCard   (card: CardDetails)               → ValidationResult          [internal]
         getTransaction  (id: string)                      → Transaction | null        [2 callers]

fields:
  private readonly  client  : StripeClient
  private           repo    : UserRepository
  private           logger  : Logger

// ~180 tokens  ·  source file is 6,200 tokens
```

---

## Supported languages

### Production-ready

![TypeScript](https://img.shields.io/badge/TypeScript-ready-22863a?style=flat-square&logo=typescript&logoColor=white)
![C#](https://img.shields.io/badge/C%23-ready-22863a?style=flat-square&logo=csharp&logoColor=white)
![Python](https://img.shields.io/badge/Python-ready-22863a?style=flat-square&logo=python&logoColor=white)
![Rust](https://img.shields.io/badge/Rust-ready-22863a?style=flat-square&logo=rust&logoColor=white)

All four languages have full support: tree-sitter grammar integration, symbol extraction, edge detection (calls, imports, extends, implements), and enriched metadata (async, static, private, abstract, decorators, visibility). C# includes partial class merging; Python includes decorator and docstring extraction; Rust includes impl block method association and visibility modifiers.

### Planned

![Go](https://img.shields.io/badge/Go-planned-e6a817?style=flat-square&logo=go&logoColor=white)
![Java](https://img.shields.io/badge/Java-planned-e6a817?style=flat-square&logo=openjdk&logoColor=white)

---

## Installation

### curl (Linux and macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/KilimcininKorOglu/inari/main/install.sh | sh
```

### go install

```bash
go install github.com/KilimcininKorOglu/inari/cmd/inari@latest
```

After installation, verify with:

```bash
inari --version
# inari 1.0.0
```

---

## Quick start

### 1. Initialise

Run once from your project root. Detects languages and writes a default `.inari/config.toml`.

```bash
inari init
```

```
Initialised .inari/ for project: api
Detected languages: TypeScript
Run 'inari index' to build the index.
```

### 2. Build the index

First run indexes the full codebase. Subsequent runs are incremental -- only changed files are re-indexed.

```bash
inari index
```

```
Indexing 847 files...
  TypeScript   612 files   4,821 symbols
  C#           235 files   1,943 symbols
Built in 12.4s. Index size: 8.2MB
```

### 3. Explore the codebase

Start with the high-level overview, then drill down.

```bash
inari map                                # full repo overview (~500-1000 tokens)
inari entrypoints                        # API controllers, workers, event handlers
inari sketch PaymentService              # structural overview of a class
inari refs processPayment                # find all callers
inari callers processPayment --depth 2   # transitive callers
inari trace processPayment               # entry-point-to-symbol call paths
inari deps PaymentService                # what does it depend on?
inari find "payment retry logic"         # semantic search
inari status                             # is my index fresh?
```

### 4. Keep the index fresh

```bash
inari status                             # check freshness
inari index                              # incremental -- < 1s for a few files
inari index --watch                      # auto re-index on file changes
```

---

## Commands

| Command             | Signature                                     | Description                                    |
|---------------------|-----------------------------------------------|------------------------------------------------|
| `inari init`        | `[--json]`                                    | Initialise Inari for a project.                |
| `inari index`       | `[--full] [--watch] [--json]`                 | Build or refresh the code index.               |
| `inari map`         | `[--limit N] [--json]`                        | Full repository overview.                      |
| `inari entrypoints` | `[--json]`                                    | List API controllers, workers, event handlers. |
| `inari sketch`      | `<symbol> [--json]`                           | Compressed structural overview.                |
| `inari refs`        | `<symbol> [--kind] [--limit N] [--json]`      | All references grouped by kind.                |
| `inari callers`     | `<symbol> [--depth N] [--context N] [--json]` | Direct and transitive callers.                 |
| `inari deps`        | `<symbol> [--depth 1-3] [--json]`             | What does this symbol depend on?               |
| `inari rdeps`       | `<symbol> [--depth 1-3] [--json]`             | What depends on this symbol?                   |
| `inari trace`       | `<symbol> [--limit N] [--json]`               | Call paths from entry points to target.        |
| `inari find`        | `"<query>" [--kind] [--limit N] [--json]`     | Full-text search with BM25 ranking.            |
| `inari similar`     | `<symbol> [--kind] [--json]`                  | Find structurally similar symbols.             |
| `inari source`      | `<symbol> [--json]`                           | Fetch full source of a symbol.                 |
| `inari status`      | `[--json]`                                    | Index health and freshness.                    |
| `inari impact`      | `<symbol> [--depth] [--json]`                 | Deprecated -- delegates to `inari callers`.    |
| `inari workspace`   | `init \| list \| index`                       | Manage multi-project workspaces.               |

### Global flags

| Flag          | Description                                                                       |
|---------------|-----------------------------------------------------------------------------------|
| `--workspace` | Query across all workspace members. Requires `inari-workspace.toml`.              |
| `--project`   | Target a specific workspace member by name.                                       |
| `--verbose`   | Enable debug output to stderr.                                                    |
| `--json`      | Output structured JSON instead of human-readable text. Supported on all commands. |

---

## Watch mode

```bash
inari index --watch
```

Monitors your project for file changes and automatically re-indexes with a 300ms debounce. Uses `fsnotify` for cross-platform file system events. Respects `.gitignore` and config ignore patterns. A lock file (`.inari/.watch.lock`) prevents multiple watchers.

### NDJSON output

```bash
inari index --watch --json
```

Emits newline-delimited JSON events: `start`, `reindex`, `stop`.

---

## Workspaces

Workspaces let you query across multiple Inari projects as a single unit.

```bash
# Initialise each project, then create workspace
inari workspace init                     # discovers projects, creates inari-workspace.toml
inari workspace index                    # index all members
inari map --workspace                    # query across all projects
inari refs PaymentService --workspace    # cross-project references
```

---

## How it works

```
Your codebase
      |
      v
+-----------------------------+
|  tree-sitter parser          |  Fast, incremental, error-tolerant AST parsing.
|  (TypeScript, C#, ...)       |  No compiler required. Extracts symbols, types,
+--------------+--------------+  modifiers, docstrings, line ranges.
               |
       +-------+--------+
       v                v
+----------+    +--------------+
|  SQLite  |    | SQLite FTS5  |
|  graph   |    |   search     |
| symbols  |    | BM25-ranked  |
| + edges  |    | symbol text  |
+----------+    +--------------+
       |                |
       +-------+--------+
               v
+-----------------------------+
|  inari query engine         |
|  Token-efficient output     |
+-----------------------------+
```

---

## Configuration

Inari reads `.inari/config.toml` in the project root.

```toml
[project]
name = "api"
languages = ["typescript", "csharp"]

[index]
ignore = ["node_modules", "dist", "build", ".git"]
include_tests = true

[embeddings]
provider = "local"

[output]
max_refs = 20
max_depth = 3
```

---

## CLAUDE.md integration

Add the following to your project's `CLAUDE.md`. Also available at [`docs/CLAUDE.md.snippet`](docs/CLAUDE.md.snippet).

```markdown
## Code Navigation

This project uses [Inari](https://github.com/KilimcininKorOglu/inari) for structural code intelligence.
Start with `inari map` for a repo overview, then `inari sketch` for specific symbols.

**Before editing a class or function:**
- `inari sketch <symbol>` -- structural overview (~200 tokens)
- `inari refs <symbol>` -- all references with file + line
- `inari callers <symbol> [--depth N]` -- blast radius

**Finding code:**
- `inari find "<query>"` -- full-text search by intent

**Understanding flow:**
- `inari deps <symbol>` -- what does this depend on?
- `inari trace <symbol>` -- call paths from entry points

Always `inari sketch` before reading full source.
```

---

## Building from source

**Prerequisites:** Go 1.25 or later, C compiler (for tree-sitter CGO bindings)

```bash
git clone https://github.com/KilimcininKorOglu/inari.git
cd inari
make build
# Binaries at bin/inari and bin/inari-benchmark
```

Run the test suite:

```bash
make test                               # all tests
make vet                                # lint
make fmt-check                          # formatting check
```

---

## Roadmap

**Shipped**
- [x] TypeScript, C#, Python, Rust symbol extraction with edge detection
- [x] SQLite dependency graph with recursive impact traversal
- [x] Full-text search with FTS5, BM25 ranking, importance-tier boosting
- [x] 18 commands including rdeps, similar, source (fully implemented)
- [x] `inari index --watch` -- auto re-index on file changes
- [x] Multi-project workspaces with federated queries
- [x] `LanguagePlugin` interface for pluggable language support
- [x] `--json` output on all commands

**Next**
- [ ] Go and Java language support
- [ ] Vector embeddings via local ONNX model
- [ ] Cross-project edge detection via `inari link`
- [ ] MCP adapter

**Later**
- [ ] Kotlin and Ruby language support
- [ ] CI/CD integration for impact analysis on PRs

---

## Acknowledgements

Inari is a Go port of [Scope](https://github.com/rynhardt-engelbrecht/scope), originally written in Rust by [Rynhardt Engelbrecht](https://github.com/rynhardt-engelbrecht). The core architecture, tree-sitter query files, and SQLite schema are derived from the original project. Three commands that were stubs in the original (`rdeps`, `similar`, `source`) have been fully implemented in this port.

---

## License

MIT -- see [LICENSE](LICENSE) for the full text.
