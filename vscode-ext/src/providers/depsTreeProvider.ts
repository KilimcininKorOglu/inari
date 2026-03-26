import * as vscode from "vscode";
import * as path from "path";
import type { WorkspaceManager } from "../workspaceManager";
import type { InariClient } from "../inari";
import type { Dependency } from "../types";

interface DepItem {
  dep: Dependency;
  client: InariClient;
}

export class DepsTreeProvider implements vscode.TreeDataProvider<DepItem> {
  private onDidChangeEmitter = new vscode.EventEmitter<DepItem | undefined>();
  readonly onDidChangeTreeData = this.onDidChangeEmitter.event;
  private currentSymbol: string | undefined;
  private currentClient: InariClient | undefined;

  constructor(private wm: WorkspaceManager) {}

  setSymbol(symbol: string, client: InariClient): void {
    this.currentSymbol = symbol;
    this.currentClient = client;
    this.onDidChangeEmitter.fire(undefined);
  }

  getTreeItem(element: DepItem): vscode.TreeItem {
    const d = element.dep;
    const hasChildren = !d.is_external;
    const item = new vscode.TreeItem(
      d.name,
      hasChildren
        ? vscode.TreeItemCollapsibleState.Collapsed
        : vscode.TreeItemCollapsibleState.None
    );
    item.description = `${d.kind}${d.is_external ? " (external)" : ""}`;
    item.iconPath = new vscode.ThemeIcon(d.is_external ? "globe" : "symbol-reference");

    if (d.file_path) {
      item.command = {
        command: "vscode.open",
        title: "Open",
        arguments: [vscode.Uri.file(path.join(element.client.workspaceRoot, d.file_path))],
      };
    }

    return item;
  }

  async getChildren(element?: DepItem): Promise<DepItem[]> {
    if (!element) {
      if (!this.currentSymbol || !this.currentClient) return [];
      try {
        const result = await this.currentClient.deps(this.currentSymbol);
        return result.data.map((dep) => ({ dep, client: this.currentClient! }));
      } catch {
        return [];
      }
    }

    if (element.dep.is_external) return [];

    try {
      const result = await element.client.deps(element.dep.name);
      return result.data.map((dep) => ({ dep, client: element.client }));
    } catch {
      return [];
    }
  }
}
