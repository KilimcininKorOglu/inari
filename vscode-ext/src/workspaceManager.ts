import * as vscode from "vscode";
import * as fs from "fs";
import * as path from "path";
import { InariClient } from "./inari";
import { getConfig } from "./config";
import { StatusBarManager } from "./statusBar";
import { IndexManager } from "./indexManager";

export class WorkspaceManager implements vscode.Disposable {
  private clients = new Map<string, InariClient>();
  private indexManagers = new Map<string, IndexManager>();
  private folderNames = new Map<string, string>();
  private statusBar: StatusBarManager;
  private outputChannel: vscode.OutputChannel;
  private folderChangeListener: vscode.Disposable | undefined;
  private _isMultiRoot: boolean;

  constructor(outputChannel: vscode.OutputChannel) {
    this.outputChannel = outputChannel;

    const folders = vscode.workspace.workspaceFolders ?? [];
    const firstRoot = folders[0]?.uri.fsPath;
    const hasWorkspaceToml =
      firstRoot != null &&
      fs.existsSync(path.join(firstRoot, "inari-workspace.toml"));

    this._isMultiRoot = folders.length > 1 || hasWorkspaceToml;

    // Create per-folder clients and index managers.
    for (const folder of folders) {
      const config = getConfig(folder.uri);
      const client = new InariClient(config.binaryPath, folder.uri.fsPath);
      this.clients.set(folder.uri.toString(), client);
      this.folderNames.set(folder.uri.toString(), folder.name);
    }

    // Status bar (created early, initialized in initialize()).
    this.statusBar = new StatusBarManager(
      this.getAllClients(),
      this._isMultiRoot
    );

    // Listen for dynamic folder changes.
    this.folderChangeListener = vscode.workspace.onDidChangeWorkspaceFolders(
      (e) => {
        for (const added of e.added) {
          const config = getConfig(added.uri);
          const client = new InariClient(config.binaryPath, added.uri.fsPath);
          this.clients.set(added.uri.toString(), client);
          this.folderNames.set(added.uri.toString(), added.name);

          const im = new IndexManager(
            client,
            this.statusBar,
            this.outputChannel,
            config.binaryPath,
            added.uri.fsPath,
            config.indexMode,
            added.name
          );
          this.indexManagers.set(added.uri.toString(), im);
          im.start();
          this.outputChannel.appendLine(`[Inari] Added folder: ${added.name}`);
        }
        for (const removed of e.removed) {
          const key = removed.uri.toString();
          const im = this.indexManagers.get(key);
          if (im) {
            im.dispose();
            this.indexManagers.delete(key);
          }
          this.clients.delete(key);
          this.folderNames.delete(key);
          this.outputChannel.appendLine(
            `[Inari] Removed folder: ${removed.name}`
          );
        }
        this.statusBar.setClients(this.getAllClients());
        void this.statusBar.update();
      }
    );
  }

  get isMultiRoot(): boolean {
    return this._isMultiRoot;
  }

  async initialize(): Promise<boolean> {
    // Verify CLI on first client.
    const firstClient = this.getAllClients()[0];
    if (!firstClient) return false;

    try {
      const ver = await firstClient.version();
      this.outputChannel.appendLine(`[Inari] CLI version: ${ver}`);
    } catch {
      return false;
    }

    // Create index managers and start them.
    const folders = vscode.workspace.workspaceFolders ?? [];
    for (const folder of folders) {
      const key = folder.uri.toString();
      const client = this.clients.get(key);
      if (!client) continue;

      const config = getConfig(folder.uri);
      const im = new IndexManager(
        client,
        this.statusBar,
        this.outputChannel,
        config.binaryPath,
        folder.uri.fsPath,
        config.indexMode,
        folder.name
      );
      this.indexManagers.set(key, im);
      im.start();
    }

    // Update status bar.
    await this.statusBar.update();
    this.statusBar.startPolling();

    return true;
  }

  getClientForUri(uri: vscode.Uri): InariClient | undefined {
    const wsFolder = vscode.workspace.getWorkspaceFolder(uri);
    if (!wsFolder) return this.getAllClients()[0];
    return this.clients.get(wsFolder.uri.toString());
  }

  getClientForActiveEditor(): InariClient | undefined {
    const editor = vscode.window.activeTextEditor;
    if (editor) {
      return this.getClientForUri(editor.document.uri);
    }
    if (this.clients.size === 1) {
      return this.getAllClients()[0];
    }
    return undefined;
  }

  async getClientOrPick(): Promise<InariClient | undefined> {
    const client = this.getClientForActiveEditor();
    if (client) return client;

    if (this.clients.size === 0) return undefined;
    if (this.clients.size === 1) return this.getAllClients()[0];

    const items = Array.from(this.clients.entries()).map(([key, c]) => ({
      label: this.folderNames.get(key) ?? "Unknown",
      description: c.workspaceRoot,
      client: c,
    }));

    const picked = await vscode.window.showQuickPick(items, {
      placeHolder: "Select project",
    });
    return picked?.client;
  }

  getWorkspaceRootForUri(uri: vscode.Uri): string | undefined {
    const client = this.getClientForUri(uri);
    return client?.workspaceRoot;
  }

  getAllClients(): InariClient[] {
    return Array.from(this.clients.values());
  }

  getAllIndexManagers(): IndexManager[] {
    return Array.from(this.indexManagers.values());
  }

  getIndexManagerForClient(client: InariClient): IndexManager | undefined {
    for (const [key, c] of this.clients) {
      if (c === client) {
        return this.indexManagers.get(key);
      }
    }
    return undefined;
  }

  getStatusBar(): StatusBarManager {
    return this.statusBar;
  }

  dispose(): void {
    for (const im of this.indexManagers.values()) {
      im.dispose();
    }
    this.indexManagers.clear();
    this.clients.clear();
    this.statusBar.dispose();
    this.folderChangeListener?.dispose();
  }
}
