import * as vscode from "vscode";
import { execFile } from "child_process";
import type { InariClient } from "../inari";
import type { IndexManager } from "../indexManager";

export function registerIndexCommands(
  client: InariClient,
  indexManager: IndexManager,
  binaryPath: string
): vscode.Disposable[] {
  const reindex = vscode.commands.registerCommand("inari.reindex", async () => {
    await vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: "Inari: Reindexing...",
        cancellable: false,
      },
      async () => {
        await indexManager.reindex();
      }
    );
  });

  const init = vscode.commands.registerCommand("inari.initProject", async () => {
    try {
      await vscode.window.withProgress(
        {
          location: vscode.ProgressLocation.Notification,
          title: "Inari: Initializing project...",
          cancellable: false,
        },
        async () => {
          await client.init();
        }
      );
      vscode.window.showInformationMessage("Inari project initialized. Indexing...");
      await indexManager.reindex();
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      vscode.window.showErrorMessage(`Inari init failed: ${msg}`);
    }
  });

  const initFolder = vscode.commands.registerCommand(
    "inari.initFolder",
    async (uri: vscode.Uri) => {
      if (!uri) return;
      const folderPath = uri.fsPath;
      try {
        await vscode.window.withProgress(
          {
            location: vscode.ProgressLocation.Notification,
            title: `Inari: Initializing ${folderPath}...`,
            cancellable: false,
          },
          () =>
            new Promise<void>((resolve, reject) => {
              execFile(
                binaryPath,
                ["init"],
                { cwd: folderPath, timeout: 30_000 },
                (error, _stdout, stderr) => {
                  if (error) {
                    reject(new Error(stderr || error.message));
                    return;
                  }
                  resolve();
                }
              );
            })
        );
        const indexNow = await vscode.window.showInformationMessage(
          `Inari initialized in ${folderPath}. Index now?`,
          "Index",
          "Later"
        );
        if (indexNow === "Index") {
          await vscode.window.withProgress(
            {
              location: vscode.ProgressLocation.Notification,
              title: "Inari: Indexing...",
              cancellable: false,
            },
            () =>
              new Promise<void>((resolve, reject) => {
                execFile(
                  binaryPath,
                  ["index", "--full"],
                  { cwd: folderPath, timeout: 120_000 },
                  (error, _stdout, stderr) => {
                    if (error) {
                      reject(new Error(stderr || error.message));
                      return;
                    }
                    resolve();
                  }
                );
              })
          );
          vscode.window.showInformationMessage("Inari indexing complete.");
        }
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        vscode.window.showErrorMessage(`Inari init failed: ${msg}`);
      }
    }
  );

  const createWorkspace = vscode.commands.registerCommand(
    "inari.createWorkspace",
    async () => {
      const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
      if (!workspaceRoot) {
        vscode.window.showWarningMessage("No workspace folder open.");
        return;
      }

      const name = await vscode.window.showInputBox({
        prompt: "Workspace name",
        value: workspaceRoot.split("/").pop() ?? "workspace",
        placeHolder: "e.g. my-monorepo",
      });
      if (!name) return;

      try {
        await vscode.window.withProgress(
          {
            location: vscode.ProgressLocation.Notification,
            title: "Inari: Creating workspace...",
            cancellable: false,
          },
          () =>
            new Promise<void>((resolve, reject) => {
              execFile(
                binaryPath,
                ["workspace", "init", "--name", name],
                { cwd: workspaceRoot, timeout: 30_000 },
                (error, _stdout, stderr) => {
                  if (error) {
                    reject(new Error(stderr || error.message));
                    return;
                  }
                  resolve();
                }
              );
            })
        );
        vscode.window.showInformationMessage(
          "Workspace created. Edit inari-workspace.toml to configure members."
        );
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        vscode.window.showErrorMessage(`Workspace creation failed: ${msg}`);
      }
    }
  );

  return [reindex, init, initFolder, createWorkspace];
}
