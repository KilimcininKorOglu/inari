import * as vscode from "vscode";
import type { InariClient } from "../inari";
import type { SketchClassData, SketchMethodData, Symbol as InariSymbol } from "../types";

export function registerSketchCommand(
  client: InariClient,
  outputChannel: vscode.OutputChannel
): vscode.Disposable {
  return vscode.commands.registerCommand("inari.sketchSymbol", async () => {
    const word = getWordAtCursor() ?? (await vscode.window.showInputBox({
      prompt: "Symbol name to sketch",
      placeHolder: "e.g. PaymentService",
    }));
    if (!word) return;

    try {
      const result = await client.sketch(word);
      outputChannel.clear();
      formatSketch(outputChannel, result.data);
      outputChannel.show(true);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      vscode.window.showErrorMessage(`Inari sketch failed: ${msg}`);
    }
  });
}

function formatSketch(ch: vscode.OutputChannel, data: unknown): void {
  const d = data as Record<string, unknown>;

  if ("methods" in d && "relationships" in d) {
    const c = d as unknown as SketchClassData;
    ch.appendLine(`${c.symbol.name}  ${c.symbol.kind}  ${c.symbol.file_path}:${c.symbol.line_start}-${c.symbol.line_end}`);
    ch.appendLine("\u2500".repeat(70));
    const rel = c.relationships;
    if (rel.extends.length > 0) ch.appendLine(`  extends: ${rel.extends.join(", ")}`);
    if (rel.implements.length > 0) ch.appendLine(`  implements: ${rel.implements.join(", ")}`);
    if (rel.dependencies.length > 0) ch.appendLine(`  deps: ${rel.dependencies.join(", ")}`);
    if (c.methods.length > 0) {
      ch.appendLine("");
      for (const m of c.methods) {
        const count = c.caller_counts[m.id] ?? 0;
        const label = count > 0 ? `[${count} callers]` : "[internal]";
        ch.appendLine(`  ${m.name}  ${label}`);
      }
    }
  } else if ("calls" in d || "called_by" in d) {
    const m = d as unknown as SketchMethodData;
    ch.appendLine(`${m.symbol.name}  ${m.symbol.kind}  ${m.symbol.file_path}:${m.symbol.line_start}-${m.symbol.line_end}`);
    ch.appendLine("\u2500".repeat(70));
    if (m.symbol.signature) ch.appendLine(`  ${m.symbol.signature}`);
    if (m.called_by && m.called_by.length > 0) {
      ch.appendLine(`  callers: ${m.called_by.map((c) => `${c.name} (${c.count}x)`).join(", ")}`);
    }
    if (m.calls && m.calls.length > 0) {
      ch.appendLine(`  calls: ${m.calls.join(", ")}`);
    }
  } else if ("symbol" in d) {
    const s = (d as { symbol: InariSymbol }).symbol;
    ch.appendLine(`${s.name}  ${s.kind}  ${s.file_path}:${s.line_start}-${s.line_end}`);
    ch.appendLine("\u2500".repeat(70));
    if (s.signature) ch.appendLine(`  ${s.signature}`);
  }
}

function getWordAtCursor(): string | undefined {
  const editor = vscode.window.activeTextEditor;
  if (!editor) return undefined;
  const range = editor.document.getWordRangeAtPosition(editor.selection.active, /[a-zA-Z_]\w*/);
  return range ? editor.document.getText(range) : undefined;
}
