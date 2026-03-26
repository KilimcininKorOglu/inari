import * as vscode from "vscode";
import * as path from "path";
import type { WorkspaceManager } from "../workspaceManager";
import type { InariClient } from "../inari";
import type { Reference } from "../types";

interface CallerItem {
  ref: Reference;
  client: InariClient;
}

export class CallersTreeProvider implements vscode.TreeDataProvider<CallerItem> {
  private onDidChangeEmitter = new vscode.EventEmitter<CallerItem | undefined>();
  readonly onDidChangeTreeData = this.onDidChangeEmitter.event;
  private currentSymbol: string | undefined;
  private currentClient: InariClient | undefined;

  constructor(private wm: WorkspaceManager) {}

  setSymbol(symbol: string, client: InariClient): void {
    this.currentSymbol = symbol;
    this.currentClient = client;
    this.onDidChangeEmitter.fire(undefined);
  }

  getTreeItem(element: CallerItem): vscode.TreeItem {
    const r = element.ref;
    const item = new vscode.TreeItem(
      r.from_name,
      vscode.TreeItemCollapsibleState.Collapsed
    );
    item.description = `${r.kind}  ${r.file_path}${r.line != null ? `:${r.line}` : ""}`;
    item.iconPath = new vscode.ThemeIcon("symbol-method");

    if (r.file_path && r.line != null) {
      item.command = {
        command: "vscode.open",
        title: "Open",
        arguments: [
          vscode.Uri.file(path.join(element.client.workspaceRoot, r.file_path)),
          { selection: new vscode.Range((r.line ?? 1) - 1, 0, (r.line ?? 1) - 1, 0) },
        ],
      };
    }

    return item;
  }

  async getChildren(element?: CallerItem): Promise<CallerItem[]> {
    if (!element) {
      if (!this.currentSymbol || !this.currentClient) return [];
      try {
        const result = await this.currentClient.callers(this.currentSymbol);
        if (!Array.isArray(result.data)) return [];
        return (result.data as Reference[]).map((ref) => ({ ref, client: this.currentClient! }));
      } catch {
        return [];
      }
    }

    try {
      const result = await element.client.callers(element.ref.from_name);
      if (!Array.isArray(result.data)) return [];
      return (result.data as Reference[]).map((ref) => ({ ref, client: element.client }));
    } catch {
      return [];
    }
  }
}
