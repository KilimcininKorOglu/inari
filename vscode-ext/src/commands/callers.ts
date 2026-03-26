import * as vscode from "vscode";
import * as path from "path";
import type { InariClient } from "../inari";
import type { Reference } from "../types";

export function registerCallersCommand(
  client: InariClient,
  workspaceRoot: string
): vscode.Disposable {
  return vscode.commands.registerCommand("inari.showCallers", async () => {
    const word = getWordAtCursor() ?? (await vscode.window.showInputBox({
      prompt: "Symbol name",
      placeHolder: "e.g. processPayment",
    }));
    if (!word) return;

    try {
      const result = await client.callers(word);
      const data = result.data;

      if (Array.isArray(data)) {
        const refs = data as Reference[];
        if (refs.length === 0) {
          vscode.window.showInformationMessage(`No callers found for "${word}".`);
          return;
        }

        const locations = refs
          .filter((r) => r.file_path && r.line != null)
          .map(
            (r) =>
              new vscode.Location(
                vscode.Uri.file(path.join(workspaceRoot, r.file_path)),
                new vscode.Position((r.line ?? 1) - 1, 0)
              )
          );

        if (locations.length > 0) {
          const editor = vscode.window.activeTextEditor;
          const uri = editor?.document.uri ?? locations[0].uri;
          const pos = editor?.selection.active ?? new vscode.Position(0, 0);
          await vscode.commands.executeCommand(
            "editor.action.showReferences",
            uri,
            pos,
            locations
          );
        }
      } else {
        vscode.window.showInformationMessage(
          `${word}: ${(data as { total_affected?: number }).total_affected ?? 0} transitive callers found. See Output panel for details.`
        );
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      vscode.window.showErrorMessage(`Inari callers failed: ${msg}`);
    }
  });
}

function getWordAtCursor(): string | undefined {
  const editor = vscode.window.activeTextEditor;
  if (!editor) return undefined;
  const range = editor.document.getWordRangeAtPosition(editor.selection.active, /[a-zA-Z_]\w*/);
  return range ? editor.document.getText(range) : undefined;
}
