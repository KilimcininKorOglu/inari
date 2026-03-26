import * as vscode from "vscode";
import * as path from "path";
import type { WorkspaceManager } from "../workspaceManager";
import type { InariClient } from "../inari";
import type { DirStats, SketchFileData, Symbol as InariSymbol } from "../types";

type TreeItem = ActionItem | WorkspaceMemberItem | DirItem | FileItem | SymbolItem;

interface ActionItem {
  type: "action";
  label: string;
  icon: string;
  command: string;
}

interface WorkspaceMemberItem {
  type: "workspace-member";
  name: string;
  client: InariClient;
  workspaceRoot: string;
}

interface DirItem {
  type: "dir";
  dir: DirStats;
  client: InariClient;
  workspaceRoot: string;
}

interface FileItem {
  type: "file";
  filePath: string;
  directory: string;
  client: InariClient;
  workspaceRoot: string;
}

interface SymbolItem {
  type: "symbol";
  symbol: InariSymbol;
  client: InariClient;
  workspaceRoot: string;
}

export class SymbolTreeProvider implements vscode.TreeDataProvider<TreeItem> {
  private onDidChangeEmitter = new vscode.EventEmitter<TreeItem | undefined>();
  readonly onDidChangeTreeData = this.onDidChangeEmitter.event;

  constructor(private wm: WorkspaceManager) {}

  refresh(): void {
    this.onDidChangeEmitter.fire(undefined);
  }

  getTreeItem(element: TreeItem): vscode.TreeItem {
    switch (element.type) {
      case "action": {
        const item = new vscode.TreeItem(
          element.label,
          vscode.TreeItemCollapsibleState.None
        );
        item.iconPath = new vscode.ThemeIcon(element.icon);
        item.command = {
          command: element.command,
          title: element.label,
        };
        return item;
      }
      case "workspace-member": {
        const item = new vscode.TreeItem(
          element.name,
          vscode.TreeItemCollapsibleState.Collapsed
        );
        item.iconPath = new vscode.ThemeIcon("root-folder");
        item.contextValue = "workspaceMember";
        return item;
      }
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
          path.join(element.workspaceRoot, element.filePath)
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
            vscode.Uri.file(path.join(element.workspaceRoot, s.file_path)),
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
      case "action":
        return [];
      case "workspace-member":
        return this.getMemberChildren(element);
      case "dir":
        return this.getDirChildren(element);
      case "file":
        return this.getFileChildren(element);
      case "symbol":
        return this.getSymbolChildren(element);
    }
  }

  private getRootChildren(): TreeItem[] | Promise<TreeItem[]> {
    const actions: ActionItem[] = this.wm.isMultiRoot
      ? [
          { type: "action", label: "Reindex Workspace", icon: "sync", command: "inari.workspaceIndex" },
          { type: "action", label: "Workspace Members", icon: "list-tree", command: "inari.workspaceList" },
          { type: "action", label: "Create Workspace", icon: "new-folder", command: "inari.createWorkspace" },
        ]
      : [
          { type: "action", label: "Reindex", icon: "refresh", command: "inari.reindex" },
          { type: "action", label: "Create Workspace", icon: "new-folder", command: "inari.createWorkspace" },
        ];

    if (this.wm.isMultiRoot) {
      const clients = this.wm.getAllClients();
      const members: TreeItem[] = clients.map((client) => ({
        type: "workspace-member" as const,
        name: client.workspaceRoot.split("/").pop() ?? "unknown",
        client,
        workspaceRoot: client.workspaceRoot,
      }));
      return [...actions, ...members];
    }

    const client = this.wm.getAllClients()[0];
    if (!client) return actions;
    return this.getArchitectureItems(client).then((dirs) => [...actions, ...dirs]);
  }

  private async getMemberChildren(
    member: WorkspaceMemberItem
  ): Promise<TreeItem[]> {
    return this.getArchitectureItems(member.client);
  }

  private async getArchitectureItems(client: InariClient): Promise<TreeItem[]> {
    try {
      const result = await client.map();
      return result.data.architecture.map((dir) => ({
        type: "dir" as const,
        dir,
        client,
        workspaceRoot: client.workspaceRoot,
      }));
    } catch {
      return [];
    }
  }

  private async getDirChildren(element: DirItem): Promise<TreeItem[]> {
    const dirPath = path.join(element.workspaceRoot, element.dir.directory);
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
          filePath: path.posix.join(element.dir.directory, name),
          directory: element.dir.directory,
          client: element.client,
          workspaceRoot: element.workspaceRoot,
        }));
    } catch {
      return [];
    }
  }

  private async getFileChildren(element: FileItem): Promise<TreeItem[]> {
    try {
      const result = await element.client.sketchFile(element.filePath);
      const data = result.data as SketchFileData;
      if (!data.symbols) return [];
      return data.symbols
        .filter((s) => !s.parent_id)
        .map((s) => ({
          type: "symbol" as const,
          symbol: s,
          client: element.client,
          workspaceRoot: element.workspaceRoot,
        }));
    } catch {
      return [];
    }
  }

  private async getSymbolChildren(element: SymbolItem): Promise<TreeItem[]> {
    if (element.symbol.kind !== "class" && element.symbol.kind !== "interface") return [];
    try {
      const result = await element.client.sketchFile(element.symbol.file_path);
      const data = result.data as SketchFileData;
      if (!data.symbols) return [];
      return data.symbols
        .filter((s) => s.parent_id === element.symbol.id)
        .map((s) => ({
          type: "symbol" as const,
          symbol: s,
          client: element.client,
          workspaceRoot: element.workspaceRoot,
        }));
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
