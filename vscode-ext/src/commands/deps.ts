import * as vscode from "vscode";
import type { WorkspaceManager } from "../workspaceManager";

export function registerDepsCommand(
  wm: WorkspaceManager,
  outputChannel: vscode.OutputChannel
): vscode.Disposable {
  return vscode.commands.registerCommand("inari.showDependencies", async () => {
    const client = await wm.getClientOrPick();
    if (!client) return;

    const word = getWordAtCursor() ?? (await vscode.window.showInputBox({
      prompt: "Symbol name",
      placeHolder: "e.g. PaymentService",
    }));
    if (!word) return;

    try {
      const result = await client.deps(word);
      const deps = result.data;
      outputChannel.clear();
      outputChannel.appendLine(`Dependencies of ${word} (depth 1):`);
      outputChannel.appendLine("\u2500".repeat(50));
      if (deps.length === 0) {
        outputChannel.appendLine("  No dependencies found.");
      } else {
        for (const d of deps) {
          const loc = d.is_external ? "(external)" : d.file_path ?? "";
          outputChannel.appendLine(`  ${d.name.padEnd(30)} ${d.kind.padEnd(16)} ${loc}`);
        }
      }
      outputChannel.show(true);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      vscode.window.showErrorMessage(`Inari deps failed: ${msg}`);
    }
  });
}

function getWordAtCursor(): string | undefined {
  const editor = vscode.window.activeTextEditor;
  if (!editor) return undefined;
  const range = editor.document.getWordRangeAtPosition(editor.selection.active, /[a-zA-Z_]\w*/);
  return range ? editor.document.getText(range) : undefined;
}
