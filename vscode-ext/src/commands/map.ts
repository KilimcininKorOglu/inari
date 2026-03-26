import * as vscode from "vscode";
import type { InariClient } from "../inari";

export function registerMapCommand(
  client: InariClient,
  outputChannel: vscode.OutputChannel
): vscode.Disposable {
  return vscode.commands.registerCommand("inari.showMap", async () => {
    try {
      const result = await client.map();
      const m = result.data;
      outputChannel.clear();

      outputChannel.appendLine("Repository Map");
      outputChannel.appendLine("\u2500".repeat(70));
      outputChannel.appendLine(
        `  ${m.stats.file_count} files, ${m.stats.symbol_count} symbols, ${m.stats.edge_count} edges`
      );
      outputChannel.appendLine(`  Languages: ${m.stats.languages.join(", ")}`);
      outputChannel.appendLine("");

      if (m.entrypoints.length > 0) {
        outputChannel.appendLine("Entry Points:");
        for (const group of m.entrypoints) {
          outputChannel.appendLine(`  ${group.Name}:`);
          for (const e of group.Entries) {
            const info = e.method_count > 0
              ? `(${e.method_count} methods, ${e.outgoing_call_count} calls)`
              : `(${e.outgoing_call_count} calls)`;
            outputChannel.appendLine(`    ${e.name}  ${e.kind}  ${e.file_path}  ${info}`);
          }
        }
        outputChannel.appendLine("");
      }

      if (m.core_symbols.length > 0) {
        outputChannel.appendLine("Core Symbols (by caller count):");
        for (const s of m.core_symbols) {
          outputChannel.appendLine(
            `  ${s.name.padEnd(30)} ${s.kind.padEnd(12)} [${s.caller_count} callers]  ${s.file_path}`
          );
        }
        outputChannel.appendLine("");
      }

      if (m.architecture.length > 0) {
        outputChannel.appendLine("Architecture:");
        for (const d of m.architecture) {
          outputChannel.appendLine(
            `  ${d.directory.padEnd(30)} ${d.file_count} files, ${d.symbol_count} symbols`
          );
        }
      }

      outputChannel.show(true);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      vscode.window.showErrorMessage(`Inari map failed: ${msg}`);
    }
  });
}
