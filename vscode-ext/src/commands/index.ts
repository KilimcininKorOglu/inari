import * as vscode from "vscode";
import { execFile } from "child_process";
import type { WorkspaceManager } from "../workspaceManager";

export function registerIndexCommands(
  wm: WorkspaceManager,
  binaryPath: string
): vscode.Disposable[] {
  const reindex = vscode.commands.registerCommand("inari.reindex", async () => {
    if (wm.isMultiRoot) {
      const clients = wm.getAllClients();
      const items = [
        { label: "$(sync) Reindex All", description: `${clients.length} projects`, all: true },
        ...clients.map((c) => ({
          label: `$(folder) ${c.workspaceRoot.split("/").pop() ?? "unknown"}`,
          description: c.workspaceRoot,
          all: false,
          client: c,
        })),
      ];
      const picked = await vscode.window.showQuickPick(items, {
        placeHolder: "Select project to reindex",
      });
      if (!picked) return;

      if (picked.all) {
        for (const im of wm.getAllIndexManagers()) {
          await vscode.window.withProgress(
            { location: vscode.ProgressLocation.Notification, title: "Inari: Reindexing all...", cancellable: false },
            async () => { await im.reindex(); }
          );
        }
      } else if ("client" in picked && picked.client) {
        const im = wm.getIndexManagerForClient(picked.client);
        if (im) {
          await vscode.window.withProgress(
            { location: vscode.ProgressLocation.Notification, title: "Inari: Reindexing...", cancellable: false },
            async () => { await im.reindex(); }
          );
        }
      }
    } else {
      const ims = wm.getAllIndexManagers();
      if (ims.length > 0) {
        await vscode.window.withProgress(
          { location: vscode.ProgressLocation.Notification, title: "Inari: Reindexing...", cancellable: false },
          async () => { await ims[0].reindex(); }
        );
      }
    }
  });

  const initProject = vscode.commands.registerCommand("inari.initProject", async () => {
    const client = await wm.getClientOrPick();
    if (!client) return;

    try {
      await vscode.window.withProgress(
        { location: vscode.ProgressLocation.Notification, title: "Inari: Initializing project...", cancellable: false },
        async () => { await client.init(); }
      );
      vscode.window.showInformationMessage("Inari project initialized. Indexing...");
      const im = wm.getIndexManagerForClient(client);
      if (im) await im.reindex();
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
          { location: vscode.ProgressLocation.Notification, title: `Inari: Initializing ${folderPath}...`, cancellable: false },
          () =>
            new Promise<void>((resolve, reject) => {
              execFile(binaryPath, ["init"], { cwd: folderPath, timeout: 30_000 }, (error, _stdout, stderr) => {
                if (error) { reject(new Error(stderr || error.message)); return; }
                resolve();
              });
            })
        );
        const indexNow = await vscode.window.showInformationMessage(
          `Inari initialized in ${folderPath}. Index now?`,
          "Index",
          "Later"
        );
        if (indexNow === "Index") {
          await vscode.window.withProgress(
            { location: vscode.ProgressLocation.Notification, title: "Inari: Indexing...", cancellable: false },
            () =>
              new Promise<void>((resolve, reject) => {
                execFile(binaryPath, ["index", "--full"], { cwd: folderPath, timeout: 120_000 }, (error, _stdout, stderr) => {
                  if (error) { reject(new Error(stderr || error.message)); return; }
                  resolve();
                });
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
          { location: vscode.ProgressLocation.Notification, title: "Inari: Creating workspace...", cancellable: false },
          () =>
            new Promise<void>((resolve, reject) => {
              execFile(binaryPath, ["workspace", "init", "--name", name], { cwd: workspaceRoot, timeout: 30_000 }, (error, _stdout, stderr) => {
                if (error) { reject(new Error(stderr || error.message)); return; }
                resolve();
              });
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

  const workspaceIndex = vscode.commands.registerCommand(
    "inari.workspaceIndex",
    async () => {
      const client = wm.getAllClients()[0];
      if (!client) return;

      try {
        await vscode.window.withProgress(
          { location: vscode.ProgressLocation.Notification, title: "Inari: Indexing workspace...", cancellable: false },
          async () => { await client.workspaceIndex(); }
        );
        vscode.window.showInformationMessage("Workspace indexing complete.");
        await wm.getStatusBar().update();
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        vscode.window.showErrorMessage(`Workspace index failed: ${msg}`);
      }
    }
  );

  const workspaceList = vscode.commands.registerCommand(
    "inari.workspaceList",
    async () => {
      const client = wm.getAllClients()[0];
      if (!client) return;

      try {
        const result = await client.workspaceList();
        const members = result.data.members;

        const items = members.map((m) => ({
          label: `$(folder) ${m.name}`,
          description: m.status,
          detail: `${m.symbol_count} symbols, ${m.file_count} files  \u2014  ${m.path}`,
        }));

        await vscode.window.showQuickPick(items, {
          placeHolder: `Workspace: ${result.data.workspace_name} (${members.length} members)`,
        });
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        vscode.window.showErrorMessage(`Workspace list failed: ${msg}`);
      }
    }
  );

  return [reindex, initProject, initFolder, createWorkspace, workspaceIndex, workspaceList];
}
