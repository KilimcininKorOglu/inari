# Inari Code

Structural code intelligence for VS Code powered by [Inari](https://github.com/KilimcininKorOglu/inari) -- tree-sitter parsing, dependency graphs, and full-text search for 16 languages.

## Prerequisites

Install the Inari CLI before using this extension:

```bash
# macOS
brew install KilimcininKorOglu/tap/inari

# From source
go install github.com/KilimcininKorOglu/inari/cmd/inari@latest
```

Then initialize and index your project:

```bash
cd your-project
inari init
inari index
```

The extension activates automatically when `.inari/config.toml` or `inari-workspace.toml` is detected in the workspace.

## Features

### Hover Intelligence

Hover over any symbol to see a compact sketch summary: kind, file location, dependencies, method list with caller counts, and docstring.

### Command Palette

All commands are available via `Cmd+Shift+P` (macOS) or `Ctrl+Shift+P` (Windows/Linux):

| Command                           | Description                                        |
|-----------------------------------|----------------------------------------------------|
| Inari: Sketch Symbol              | Show structural overview of the symbol at cursor.  |
| Inari: Find Symbol                | Full-text search across the index with ranking.    |
| Inari: Show References            | All references to the symbol at cursor.            |
| Inari: Show Dependencies          | What the symbol depends on (imports, calls, types).|
| Inari: Show Callers               | Who calls this function or method.                 |
| Inari: Trace to Entry Points      | Call paths from API entry points to the symbol.    |
| Inari: Show Map                   | Full repository overview with architecture stats.  |
| Inari: Reindex                    | Manually trigger an incremental re-index.          |
| Inari: Initialize Project         | Run `inari init` in the workspace root.            |
| Inari: Initialize Project Here    | Run `inari init` in a specific folder (right-click in Explorer). |
| Inari: Create Workspace           | Create `inari-workspace.toml` for multi-project setups. |
| Inari: Index Workspace            | Index all workspace members at once.               |
| Inari: Workspace Members          | List all workspace members with status and symbol counts. |

### Sidebar

The Inari sidebar (database icon in the activity bar) provides three tree views:

- **Symbols** -- Browse the repository by directory, file, and symbol hierarchy. Includes always-visible action buttons for quick access to reindex and workspace operations.
- **Dependencies** -- View forward dependencies of a selected symbol. Expand nodes to see transitive dependencies.
- **Callers** -- View the caller chain of a selected symbol. Expand nodes to trace callers recursively.

### CodeLens

Inline caller counts displayed above functions, methods, classes, and interfaces. Click a count to jump to all callers in the peek view.

### Status Bar

The status bar shows the current index state:

- **Single project:** `Inari: 1037 symbols (2m ago)` -- click to reindex.
- **Workspace:** `Inari: 3 projects (2500 symbols)` -- click to index all workspace members.

### Multi-Root Workspace Support

The extension supports VS Code multi-root workspaces and Inari workspace manifests (`inari-workspace.toml`):

- Each workspace folder gets its own Inari client and index manager.
- Commands auto-detect which project a file belongs to via the active editor.
- If no editor is open, a project picker appears.
- The Symbols tree shows workspace members at the root level for easy navigation.
- Index mode (onSave, watch, manual) can be configured per folder.

### Explorer Context Menu

Right-click any folder in the Explorer to see **Inari: Initialize Project Here** -- runs `inari init` in that folder.

## Settings

| Setting                | Type    | Default   | Scope    | Description                                                  |
|------------------------|---------|-----------|----------|--------------------------------------------------------------|
| `inari.path`           | string  | `"inari"` | Resource | Path to the Inari CLI binary.                                |
| `inari.indexMode`      | enum    | `"onSave"`| Resource | How the index is kept up to date.                            |
| `inari.hoverEnabled`   | boolean | `true`    | Window   | Show sketch summary when hovering over symbols.              |
| `inari.codeLensEnabled`| boolean | `true`    | Window   | Show caller counts above functions and classes.              |
| `inari.codeLensCallers`| boolean | `true`    | Window   | Show caller count in CodeLens.                               |

### Index Mode Options

| Mode     | Behavior                                                                 |
|----------|--------------------------------------------------------------------------|
| `onSave` | Automatically re-index when files are saved (2-second debounce).         |
| `watch`  | Run `inari index --watch` as a background process for continuous updates.|
| `manual` | Only re-index when triggered via the command palette or status bar.      |

Settings with **Resource** scope can be configured per workspace folder in `.vscode/settings.json`, allowing different index modes for different projects in a multi-root workspace.

## Supported Languages

TypeScript, C#, Python, Rust, Go, Java, Kotlin, Ruby, PHP, Lua, Swift, Bash, C, C++, Protocol Buffers, SQL.

## Links

- [Inari CLI](https://github.com/KilimcininKorOglu/inari) -- The CLI tool that powers this extension.
- [Documentation](https://inari.hermestech.uk) -- Landing page with setup instructions and feature overview.
- [Issues](https://github.com/KilimcininKorOglu/inari/issues) -- Report bugs or request features.
