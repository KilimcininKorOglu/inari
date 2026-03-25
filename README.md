# Inari

Structural code intelligence for LLM coding agents.

[![Go](https://img.shields.io/badge/built_with-Go-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-v1.3.4-blue.svg)](https://github.com/KilimcininKorOglu/inari/releases)
[![CI](https://github.com/KilimcininKorOglu/inari/actions/workflows/ci.yml/badge.svg)](https://github.com/KilimcininKorOglu/inari/actions/workflows/ci.yml)
[![Website](https://img.shields.io/badge/website-inari.hermestech.uk-e8843c)](https://inari.hermestech.uk)

Inari builds a local code intelligence index and exposes it through a CLI optimised for LLM coding agents. An agent runs `inari sketch PaymentService` and receives class structure, method signatures, caller counts, and the dependency surface in ~180 tokens -- without reading the 6,000-token source file.

The index is built from tree-sitter AST parsing, stored in a SQLite dependency graph with FTS5 full-text search, and queried through 19 commands that return structured, token-efficient output. Everything lives in a `.inari/` directory. No server, no Docker, no API key.

```
$ inari sketch PaymentService

PaymentService                                    class  src/payments/service.ts:12-89
──────────────────────────────────────────────────────────────────────────────────────
deps:      StripeClient, UserRepository, Logger, PaymentConfig
extends:   BaseService
implements: IPaymentService

methods:
  async  processPayment  (amount: Decimal, userId: string)  -> Promise<PaymentResult>   [11 callers]
         refundPayment   (txId: string, reason?: string)    -> Promise<bool>             [3 callers]
  private validateCard   (card: CardDetails)                -> ValidationResult          [internal]
         getTransaction  (id: string)                       -> Transaction | null        [2 callers]

fields:
  private readonly  client  : StripeClient
  private           repo    : UserRepository
  private           logger  : Logger

// ~180 tokens  |  source file is 6,200 tokens
```

---

## Installation

```bash
# Linux and macOS
curl -fsSL https://inari.hermestech.uk/install.sh | sh
```

```powershell
# Windows (PowerShell)
irm https://inari.hermestech.uk/install.ps1 | iex
```

```bash
# Or via go install
go install github.com/KilimcininKorOglu/inari/cmd/inari@latest
```

Verify:

```bash
inari --version
# inari 1.3.4
```

---

## Quick start

```bash
inari init                               # detect languages, create .inari/config.toml
inari index                              # build the index (incremental on subsequent runs)
inari sketch PaymentService              # structural overview of a class
inari refs processPayment                # find all callers
inari find "payment retry logic"         # full-text search by intent
inari index --watch                      # auto re-index on file changes
```

---

## Commands

### Orientation

| Command             | Description                                                                |
|---------------------|----------------------------------------------------------------------------|
| `inari map`         | Full repository overview: entry points, core symbols, architecture layers. |
| `inari entrypoints` | List API controllers, workers, and event handlers.                         |
| `inari status`      | Index health: symbol count, file count, freshness, search availability.    |

### Exploration

| Command                              | Description                                                    |
|--------------------------------------|----------------------------------------------------------------|
| `inari sketch <symbol>`              | Compressed structural overview with caller counts and deps.    |
| `inari refs <symbol> [--kind K]`     | All references grouped by kind (calls, imports, extends, ...). |
| `inari callers <symbol> [--depth N]` | Direct and transitive callers. Blast radius analysis.          |
| `inari deps <symbol> [--depth 1-3]`  | Forward dependencies: what does this symbol depend on?         |
| `inari rdeps <symbol> [--depth 1-3]` | Reverse dependencies: what depends on this symbol?             |
| `inari trace <symbol>`               | Call paths from entry points to the target symbol.             |
| `inari find "<query>" [--kind K]`    | Full-text search with BM25 ranking. Search by intent.          |
| `inari similar <symbol>`             | Find structurally similar symbols.                             |
| `inari source <symbol>`              | Fetch full source code of a symbol.                            |

### Index management

| Command                | Description                                                      |
|------------------------|------------------------------------------------------------------|
| `inari init`           | Initialise Inari for a project. Detects languages automatically. |
| `inari index [--full]` | Build or refresh the index. Incremental by default.              |
| `inari index --watch`  | Monitor files and auto re-index with 300ms debounce.             |
| `inari update`         | Check for and install the latest version from GitHub Releases.   |
| `inari update --check` | Check for updates without installing.                            |

### Workspaces

| Command                   | Description                                          |
|---------------------------|------------------------------------------------------|
| `inari workspace init`    | Discover projects and create `inari-workspace.toml`. |
| `inari workspace index`   | Index all workspace members.                         |
| `inari workspace list`    | List members with status and symbol counts.          |
| Any command `--workspace` | Fan out queries across all members.                  |

### Global flags

| Flag                | Description                                 |
|---------------------|---------------------------------------------|
| `--json`            | Structured JSON output on all commands.     |
| `--workspace`       | Query across all workspace members.         |
| `--project`         | Target a specific workspace member by name. |
| `--verbose`         | Debug output to stderr.                     |
| `--no-update-check` | Disable automatic background update checks. |

---

## Supported languages

| Language   | Status | Highlights                                                           |
|------------|--------|----------------------------------------------------------------------|
| TypeScript | Ready  | Full edge detection, async/static/abstract modifiers, JSX support.   |
| C#         | Ready  | Partial class merging, visibility modifiers, async/virtual/override. |
| Python     | Ready  | Decorator extraction, docstring capture, classmethod/staticmethod.   |
| Rust       | Ready  | Impl block association, visibility modifiers (`pub`, `pub(crate)`).  |
| Go         | Ready  | Exported/unexported detection, receiver types, composite literals.   |
| Java       | Ready  | Access modifiers, abstract/final/synchronized, extends/implements.   |
| Kotlin     | Ready  | Visibility modifiers, data/sealed/inner classes, suspend/inline.     |
| Ruby       | Ready  | Module support, mixin edges (include/extend/prepend), attr macros.   |
| PHP        | Ready  | Namespace as module, trait edges, static calls, visibility modifiers. |
| Lua        | Ready  | Table-based OOP, require imports, colon-syntax method calls.         |
| Swift      | Ready  | Protocol as interface, class/struct/enum, navigation expressions.    |
| Bash       | Ready  | Function definitions, source/. imports, command call detection.      |
| C          | Ready  | Functions, structs, enums, typedefs, #include imports.               |
| C++        | Ready  | Classes, namespaces, inheritance, member/scoped calls, new.          |

Each language is a plugin: a tree-sitter grammar and two `.scm` query files (`symbols.scm`, `edges.scm`). Adding a new language requires ~200 lines of Go.

---

## How it works

```
Source files
    |
    v
tree-sitter parser .............. Fast, error-tolerant AST parsing.
    |                              No compiler required.
    v
SQLite graph DB + FTS5 search .. Symbols, edges, file hashes.
    |                              BM25-ranked full-text search.
    v
inari CLI ...................... 19 commands. Token-efficient output.
                                   Human-readable or --json.
```

The index lives in `.inari/graph.db` (SQLite with WAL mode). Incremental indexing uses SHA-256 file hashing -- only changed files are re-parsed. Watch mode uses `fsnotify` with a 300ms debounce and an atomic lock file to prevent concurrent watchers.

---

## Configuration

Inari reads `.inari/config.toml` in the project root:

```toml
[project]
name = "api"
languages = ["typescript", "csharp"]

[index]
ignore = ["node_modules", "dist", "build", ".git"]
include_tests = true

[output]
max_refs = 20
max_depth = 3
```

---

## Agent integration

Inari works with any AI coding agent that can execute shell commands. Add the snippet below to your agent's instruction file.

| Agent              | Instruction file                        |
|--------------------|-----------------------------------------|
| Claude Code        | `CLAUDE.md`                             |
| Cursor             | `.cursor/rules/*.mdc` or `.cursorrules` |
| GitHub Copilot     | `.github/copilot-instructions.md`       |
| Gemini CLI         | `GEMINI.md`                             |
| Codex              | `AGENTS.md`                             |
| Aider              | `.aider.conf.yml`                       |
| Windsurf / Codeium | `.windsurfrules`                        |

The full snippet is available at [`docs/CLAUDE.md.snippet`](docs/CLAUDE.md.snippet).

```markdown
## Code Navigation

This project uses [Inari](https://github.com/KilimcininKorOglu/inari) for structural code intelligence.
Start with `inari map` for a repo overview, then `inari sketch` for specific symbols.

**Orientation:**
- `inari map` -- full repo overview: entry points, core symbols, architecture (~500-1000 tokens)
- `inari entrypoints` -- list API controllers, workers, event handlers
- `inari status` -- check index health and freshness

**Before editing a class or function:**
- `inari sketch <symbol>` -- structural overview: methods, deps, modifiers (~200 tokens)
- `inari refs <symbol> [--kind calls|imports|extends|implements|...]` -- all references with file + line
- `inari callers <symbol> [--depth N]` -- direct and transitive callers for blast radius

**Finding code:**
- `inari find "<query>" [--kind function|class|method|interface]` -- full-text search by intent

**Understanding dependencies and flow:**
- `inari deps <symbol> [--depth 1-3]` -- what does this depend on?
- `inari rdeps <symbol> [--depth 1-3]` -- what depends on this?
- `inari trace <symbol>` -- call paths from entry points to target
- `inari similar <symbol>` -- find structurally similar symbols
- `inari source <symbol>` -- fetch full source code of a symbol

**Keeping the index fresh:**
- `inari index` -- incremental re-index after edits (< 1s for a few files)
- `inari index --watch` -- auto re-index on file changes (runs in background)
- Line numbers reflect the last index run. Re-index if they look wrong.

Always `inari sketch` before reading full source. Only read source when ready to edit.
```

---

## Building from source

Prerequisites: Go 1.25+, C compiler (tree-sitter requires CGO).

```bash
git clone https://github.com/KilimcininKorOglu/inari.git
cd inari
make build                     # bin/inari + bin/inari-benchmark
make test                      # all tests (130+ integration + unit)
make vet                       # go vet
make fmt-check                 # formatting check
```

---

## Roadmap

See [ROADMAP.md](ROADMAP.md) for shipped features, next priorities, and long-term plans.

---

## Acknowledgements

Inari is a Go port of [Scope](https://github.com/rynhardt-engelbrecht/scope), originally written in Rust by [Rynhardt Engelbrecht](https://github.com/rynhardt-engelbrecht). The core architecture, tree-sitter query files, and SQLite schema are derived from the original project. Three commands that were stubs in the original (`rdeps`, `similar`, `source`) have been fully implemented in this port.

---

## License

MIT -- see [LICENSE](LICENSE).
