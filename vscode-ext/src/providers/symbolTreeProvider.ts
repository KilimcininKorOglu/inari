import * as vscode from "vscode";
import * as path from "path";
import type { InariClient } from "../inari";
import type { DirStats, SketchFileData, Symbol as InariSymbol } from "../types";

type TreeItem = DirItem | FileItem | SymbolItem;

interface DirItem {
  type: "dir";
  dir: DirStats;
}

interface FileItem {
  type: "file";
  filePath: string;
  directory: string;
}

interface SymbolItem {
  type: "symbol";
  symbol: InariSymbol;
}

export class SymbolTreeProvider implements vscode.TreeDataProvider<TreeItem> {
  private onDidChangeEmitter = new vscode.EventEmitter<TreeItem | undefined>();
  readonly onDidChangeTreeData = this.onDidChangeEmitter.event;
  private architecture: DirStats[] = [];

  constructor(
    private client: InariClient,
    private workspaceRoot: string
  ) {}

  refresh(): void {
    this.architecture = [];
    this.onDidChangeEmitter.fire(undefined);
  }

  getTreeItem(element: TreeItem): vscode.TreeItem {
    switch (element.type) {
      case "dir": {
        const item = new vscode.TreeItem(
          element.dir.directory,
          vscode.TreeItemCollapsibleState.Collapsed
        );
        item.description = `${element.dir.file_count} files, ${element.dir.symbol_count} symbols`;
        item.iconPath = new vscode.ThemeIcon("folder");
        return item;
      }
      case "file": {
        const fileName = path.basename(element.filePath);
        const item = new vscode.TreeItem(
          fileName,
          vscode.TreeItemCollapsibleState.Collapsed
        );
        item.description = element.filePath;
        item.iconPath = new vscode.ThemeIcon("file-code");
        item.resourceUri = vscode.Uri.file(
          path.join(this.workspaceRoot, element.filePath)
        );
        return item;
      }
      case "symbol": {
        const s = element.symbol;
        const item = new vscode.TreeItem(
          s.name,
          s.kind === "class" || s.kind === "interface"
            ? vscode.TreeItemCollapsibleState.Collapsed
            : vscode.TreeItemCollapsibleState.None
        );
        item.description = s.kind;
        item.iconPath = new vscode.ThemeIcon(kindToIcon(s.kind));
        item.command = {
          command: "vscode.open",
          title: "Open",
          arguments: [
            vscode.Uri.file(path.join(this.workspaceRoot, s.file_path)),
            { selection: new vscode.Range(s.line_start - 1, 0, s.line_start - 1, 0) },
          ],
        };
        return item;
      }
    }
  }

  async getChildren(element?: TreeItem): Promise<TreeItem[]> {
    if (!element) {
      return this.getRootChildren();
    }

    switch (element.type) {
      case "dir":
        return this.getDirChildren(element.dir.directory);
      case "file":
        return this.getFileChildren(element.filePath);
      case "symbol":
        return this.getSymbolChildren(element.symbol);
    }
  }

  private async getRootChildren(): Promise<TreeItem[]> {
    if (this.architecture.length === 0) {
      try {
        const result = await this.client.map();
        this.architecture = result.data.architecture;
      } catch {
        return [];
      }
    }
    return this.architecture.map((dir) => ({ type: "dir" as const, dir }));
  }

  private async getDirChildren(directory: string): Promise<TreeItem[]> {
    const dirPath = path.join(this.workspaceRoot, directory);
    try {
      const entries = await vscode.workspace.fs.readDirectory(
        vscode.Uri.file(dirPath)
      );
      return entries
        .filter(
          ([name, type]) =>
            type === vscode.FileType.File && isSourceFile(name)
        )
        .map(([name]) => ({
          type: "file" as const,
          filePath: path.posix.join(directory, name),
          directory,
        }));
    } catch {
      return [];
    }
  }

  private async getFileChildren(filePath: string): Promise<TreeItem[]> {
    try {
      const result = await this.client.sketchFile(filePath);
      const data = result.data as SketchFileData;
      if (!data.symbols) return [];
      return data.symbols
        .filter((s) => !s.parent_id)
        .map((s) => ({ type: "symbol" as const, symbol: s }));
    } catch {
      return [];
    }
  }

  private async getSymbolChildren(symbol: InariSymbol): Promise<TreeItem[]> {
    if (symbol.kind !== "class" && symbol.kind !== "interface") return [];
    try {
      const result = await this.client.sketchFile(symbol.file_path);
      const data = result.data as SketchFileData;
      if (!data.symbols) return [];
      return data.symbols
        .filter((s) => s.parent_id === symbol.id)
        .map((s) => ({ type: "symbol" as const, symbol: s }));
    } catch {
      return [];
    }
  }
}

function kindToIcon(kind: string): string {
  switch (kind) {
    case "function": return "symbol-method";
    case "class": return "symbol-class";
    case "method": return "symbol-method";
    case "interface": return "symbol-interface";
    case "struct": return "symbol-struct";
    case "enum": return "symbol-enum";
    case "const": return "symbol-constant";
    case "type": return "symbol-type-parameter";
    case "property": return "symbol-property";
    case "module": return "symbol-namespace";
    default: return "symbol-misc";
  }
}

function isSourceFile(name: string): boolean {
  const ext = path.extname(name).toLowerCase();
  const sourceExts = new Set([
    ".ts", ".tsx", ".js", ".jsx", ".cs", ".py", ".rs", ".go",
    ".java", ".kt", ".rb", ".php", ".lua", ".swift", ".sh",
    ".bash", ".c", ".h", ".cpp", ".cc", ".cxx", ".hpp", ".hxx",
    ".proto", ".sql",
  ]);
  return sourceExts.has(ext);
}
