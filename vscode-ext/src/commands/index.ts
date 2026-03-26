import * as vscode from "vscode";
import type { InariClient } from "../inari";
import type { IndexManager } from "../indexManager";

export function registerIndexCommands(
  client: InariClient,
  indexManager: IndexManager
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

  return [reindex, init];
}
