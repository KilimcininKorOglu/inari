import * as vscode from "vscode";
import type { InariClient } from "./inari";
import type { StatusData, WorkspaceStatusData } from "./types";

// Detect if status response is workspace format (has `totals` and `members`).
function extractStatusData(data: unknown): StatusData {
  const d = data as Record<string, unknown>;
  if (d && "totals" in d && "members" in d) {
    // Workspace format — use aggregate totals.
    return (d as unknown as WorkspaceStatusData).totals;
  }
  return data as StatusData;
}

export class StatusBarManager implements vscode.Disposable {
  private item: vscode.StatusBarItem;
  private timer: ReturnType<typeof setInterval> | undefined;
  private clients: InariClient[];
  private multiRoot: boolean;

  constructor(clients: InariClient[], multiRoot: boolean) {
    this.clients = clients;
    this.multiRoot = multiRoot;
    this.item = vscode.window.createStatusBarItem(
      vscode.StatusBarAlignment.Left,
      100
    );
    this.item.command = multiRoot ? "inari.workspaceIndex" : "inari.reindex";
    this.item.show();
  }

  setClients(clients: InariClient[]): void {
    this.clients = clients;
  }

  async update(forceRelative?: string): Promise<void> {
    if (this.multiRoot) {
      await this.updateMultiRoot();
    } else {
      await this.updateSingle(forceRelative);
    }
  }

  private async updateSingle(forceRelative?: string): Promise<void> {
    const client = this.clients[0];
    if (!client) return;

    try {
      const result = await client.status();
      const s = extractStatusData(result.data);
      if (!s.index_exists) {
        this.item.text = "$(warning) Inari: No index";
        this.item.tooltip = "Click to initialize and index the project.";
        return;
      }
      const relative = forceRelative ?? s.last_indexed_relative ?? "unknown";
      this.item.text = `$(database) Inari: ${s.symbol_count} symbols (${relative})`;
      this.item.tooltip = [
        `Files: ${s.file_count}`,
        `Symbols: ${s.symbol_count}`,
        `Edges: ${s.edge_count}`,
        `Search: ${s.search_available ? "available" : "unavailable"}`,
        `Last indexed: ${relative}`,
        "",
        "Click to re-index.",
      ].join("\n");
    } catch {
      this.item.text = "$(error) Inari: CLI not found";
      this.item.tooltip =
        "Could not reach the inari binary. Check inari.path setting.";
    }
  }

  private async updateMultiRoot(): Promise<void> {
    let totalSymbols = 0;
    let totalFiles = 0;
    let projectCount = 0;
    const tooltipLines: string[] = [];

    for (const client of this.clients) {
      const folderName =
        client.workspaceRoot.split("/").pop() ?? "unknown";
      try {
        const result = await client.status();
        const s = extractStatusData(result.data);
        if (s.index_exists) {
          projectCount++;
          totalSymbols += s.symbol_count;
          totalFiles += s.file_count;
          tooltipLines.push(
            `${folderName}: ${s.symbol_count} symbols (${s.last_indexed_relative ?? "unknown"})`
          );
        } else {
          tooltipLines.push(`${folderName}: No index`);
        }
      } catch {
        tooltipLines.push(`${folderName}: Error`);
      }
    }

    this.item.text = `$(database) Inari: ${projectCount} projects (${totalSymbols} symbols)`;
    this.item.tooltip = [
      ...tooltipLines,
      "",
      `Total: ${totalFiles} files, ${totalSymbols} symbols`,
      "",
      "Click to re-index workspace.",
    ].join("\n");
  }

  showIndexing(): void {
    this.item.text = "$(sync~spin) Inari: Indexing...";
  }

  startPolling(intervalMs = 30_000): void {
    this.stopPolling();
    this.timer = setInterval(() => void this.update(), intervalMs);
  }

  stopPolling(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = undefined;
    }
  }

  dispose(): void {
    this.stopPolling();
    this.item.dispose();
  }
}
