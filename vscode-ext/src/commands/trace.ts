import * as vscode from "vscode";
import type { InariClient } from "../inari";

export function registerTraceCommand(
  client: InariClient,
  outputChannel: vscode.OutputChannel
): vscode.Disposable {
  return vscode.commands.registerCommand("inari.traceEntryPoints", async () => {
    const word = getWordAtCursor() ?? (await vscode.window.showInputBox({
      prompt: "Symbol name to trace",
      placeHolder: "e.g. processPayment",
    }));
    if (!word) return;

    try {
      const result = await client.trace(word);
      const trace = result.data;
      outputChannel.clear();
      outputChannel.appendLine(`Trace paths to ${trace.target}:`);
      outputChannel.appendLine("\u2500".repeat(50));

      if (!trace.paths || trace.paths.length === 0) {
        outputChannel.appendLine("  No call paths found from entry points.");
      } else {
        for (let i = 0; i < trace.paths.length; i++) {
          const p = trace.paths[i];
          const chain = p.steps.map((s) => s.symbol_name).join(" \u2192 ");
          outputChannel.appendLine(`  Path ${i + 1}: ${chain}`);
          for (const step of p.steps) {
            outputChannel.appendLine(`    ${step.symbol_name}  ${step.kind}  ${step.file_path}:${step.line}`);
          }
          outputChannel.appendLine("");
        }
      }
      outputChannel.show(true);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      vscode.window.showErrorMessage(`Inari trace failed: ${msg}`);
    }
  });
}

function getWordAtCursor(): string | undefined {
  const editor = vscode.window.activeTextEditor;
  if (!editor) return undefined;
  const range = editor.document.getWordRangeAtPosition(editor.selection.active, /[a-zA-Z_]\w*/);
  return range ? editor.document.getText(range) : undefined;
}
