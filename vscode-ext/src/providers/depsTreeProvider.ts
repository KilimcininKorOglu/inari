import * as vscode from "vscode";
import * as path from "path";
import type { InariClient } from "../inari";
import type { Dependency } from "../types";

interface DepItem {
  dep: Dependency;
}

export class DepsTreeProvider implements vscode.TreeDataProvider<DepItem> {
  private onDidChangeEmitter = new vscode.EventEmitter<DepItem | undefined>();
  readonly onDidChangeTreeData = this.onDidChangeEmitter.event;
  private currentSymbol: string | undefined;

  constructor(
    private client: InariClient,
    private workspaceRoot: string
  ) {}

  setSymbol(symbol: string): void {
    this.currentSymbol = symbol;
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
        arguments: [vscode.Uri.file(path.join(this.workspaceRoot, d.file_path))],
      };
    }

    return item;
  }

  async getChildren(element?: DepItem): Promise<DepItem[]> {
    if (!element) {
      if (!this.currentSymbol) return [];
      try {
        const result = await this.client.deps(this.currentSymbol);
        return result.data.map((dep) => ({ dep }));
      } catch {
        return [];
      }
    }

    if (element.dep.is_external) return [];

    try {
      const result = await this.client.deps(element.dep.name);
      return result.data.map((dep) => ({ dep }));
    } catch {
      return [];
    }
  }
}
