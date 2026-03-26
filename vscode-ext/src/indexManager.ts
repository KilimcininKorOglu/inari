import * as vscode from "vscode";
import { spawn, type ChildProcess } from "child_process";
import type { InariClient } from "./inari";
import type { StatusBarManager } from "./statusBar";
import type { IndexMode } from "./config";

export class IndexManager implements vscode.Disposable {
  private mode: IndexMode;
  private watchProcess: ChildProcess | null = null;
  private debounceTimer: ReturnType<typeof setTimeout> | undefined;
  private saveListener: vscode.Disposable | undefined;
  private disposed = false;

  constructor(
    private client: InariClient,
    private statusBar: StatusBarManager,
    private outputChannel: vscode.OutputChannel,
    private binaryPath: string,
    private workspaceRoot: string,
    mode: IndexMode
  ) {
    this.mode = mode;
  }

  start(): void {
    this.stop();
    switch (this.mode) {
      case "onSave":
        this.startOnSaveMode();
        break;
      case "watch":
        this.startWatchMode();
        break;
      case "manual":
        break;
    }
  }

  stop(): void {
    if (this.saveListener) {
      this.saveListener.dispose();
      this.saveListener = undefined;
    }
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
      this.debounceTimer = undefined;
    }
    if (this.watchProcess) {
      this.watchProcess.kill("SIGTERM");
      this.watchProcess = null;
    }
  }

  switchMode(newMode: IndexMode): void {
    this.mode = newMode;
    this.start();
  }

  async reindex(): Promise<void> {
    this.statusBar.showIndexing();
    this.outputChannel.appendLine("[Inari] Reindexing...");
    try {
      await this.client.index();
      this.outputChannel.appendLine("[Inari] Reindex complete.");
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      this.outputChannel.appendLine(`[Inari] Reindex failed: ${msg}`);
      vscode.window.showErrorMessage(`Inari reindex failed: ${msg}`);
    }
    await this.statusBar.update();
  }

  private startOnSaveMode(): void {
    this.saveListener = vscode.workspace.onDidSaveTextDocument((doc) => {
      if (!doc.uri.fsPath.startsWith(this.workspaceRoot)) {
        return;
      }
      if (this.debounceTimer) {
        clearTimeout(this.debounceTimer);
      }
      this.debounceTimer = setTimeout(() => void this.reindex(), 2000);
    });
    this.outputChannel.appendLine("[Inari] Index mode: onSave (2s debounce)");
  }

  private startWatchMode(): void {
    this.outputChannel.appendLine("[Inari] Starting watch mode...");
    const proc = spawn(this.binaryPath, ["index", "--watch", "--json"], {
      cwd: this.workspaceRoot,
      stdio: ["ignore", "pipe", "pipe"],
    });
    this.watchProcess = proc;

    let buffer = "";
    proc.stdout?.on("data", (chunk: Buffer) => {
      buffer += chunk.toString();
      const lines = buffer.split("\n");
      buffer = lines.pop() ?? "";
      for (const line of lines) {
        if (!line.trim()) continue;
        this.handleWatchEvent(line);
      }
    });

    proc.stderr?.on("data", (chunk: Buffer) => {
      this.outputChannel.appendLine(`[Inari Watch] ${chunk.toString().trim()}`);
    });

    proc.on("exit", (code) => {
      this.watchProcess = null;
      if (this.disposed) return;
      this.outputChannel.appendLine(
        `[Inari] Watch process exited (code ${code})`
      );
      if (code !== 0) {
        setTimeout(() => {
          if (!this.disposed && this.mode === "watch") {
            this.outputChannel.appendLine("[Inari] Restarting watch mode...");
            this.startWatchMode();
          }
        }, 5000);
      }
    });

    proc.on("error", (err) => {
      this.outputChannel.appendLine(
        `[Inari] Watch process error: ${err.message}`
      );
      vscode.window.showWarningMessage(
        `Inari watch mode failed: ${err.message}`
      );
    });
  }

  private handleWatchEvent(line: string): void {
    try {
      const event = JSON.parse(line) as {
        event: string;
        files_changed?: number;
        symbols_added?: number;
        symbols_removed?: number;
      };
      switch (event.event) {
        case "start":
          this.outputChannel.appendLine("[Inari Watch] Started.");
          break;
        case "reindex":
          this.outputChannel.appendLine(
            `[Inari Watch] Reindexed: ${event.files_changed ?? 0} files changed.`
          );
          void this.statusBar.update();
          break;
        case "stop":
          this.outputChannel.appendLine("[Inari Watch] Stopped.");
          break;
      }
    } catch {
      this.outputChannel.appendLine(`[Inari Watch] ${line}`);
    }
  }

  dispose(): void {
    this.disposed = true;
    this.stop();
  }
}
