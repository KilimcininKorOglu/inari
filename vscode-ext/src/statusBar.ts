import * as vscode from "vscode";
import type { InariClient } from "./inari";

export class StatusBarManager implements vscode.Disposable {
  private item: vscode.StatusBarItem;
  private timer: ReturnType<typeof setInterval> | undefined;

  constructor(private client: InariClient) {
    this.item = vscode.window.createStatusBarItem(
      vscode.StatusBarAlignment.Left,
      100
    );
    this.item.command = "inari.reindex";
    this.item.show();
  }

  async update(forceRelative?: string): Promise<void> {
    try {
      const result = await this.client.status();
      const s = result.data;
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
