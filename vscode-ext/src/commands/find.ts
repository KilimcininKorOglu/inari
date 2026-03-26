import * as vscode from "vscode";
import * as path from "path";
import type { WorkspaceManager } from "../workspaceManager";
import type { SearchResult } from "../types";

export function registerFindCommand(
  wm: WorkspaceManager
): vscode.Disposable {
  return vscode.commands.registerCommand("inari.findSymbol", async () => {
    const client = await wm.getClientOrPick();
    if (!client) return;

    const query = await vscode.window.showInputBox({
      prompt: "Search query",
      placeHolder: "e.g. payment processing",
    });
    if (!query) return;

    try {
      const result = await client.find(query, undefined, 20);
      if (result.data.length === 0) {
        vscode.window.showInformationMessage("No symbols found.");
        return;
      }

      const items = result.data.map((r: SearchResult) => ({
        label: `$(symbol-${kindToIcon(r.kind)}) ${r.name}`,
        description: `${r.kind} (${Math.round(r.score * 100)}%)`,
        detail: `${r.file_path}:${r.line_start}`,
        result: r,
      }));

      const picked = await vscode.window.showQuickPick(items, {
        placeHolder: `${result.total} results`,
        matchOnDescription: true,
        matchOnDetail: true,
      });
      if (!picked) return;

      const filePath = path.join(client.workspaceRoot, picked.result.file_path);
      const doc = await vscode.workspace.openTextDocument(filePath);
      const line = picked.result.line_start - 1;
      await vscode.window.showTextDocument(doc, {
        selection: new vscode.Range(line, 0, line, 0),
      });
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      vscode.window.showErrorMessage(`Inari find failed: ${msg}`);
    }
  });
}

function kindToIcon(kind: string): string {
  switch (kind) {
    case "function": return "method";
    case "class": return "class";
    case "method": return "method";
    case "interface": return "interface";
    case "struct": return "struct";
    case "enum": return "enum";
    case "const": return "constant";
    case "type": return "type-parameter";
    case "property": return "property";
    case "module": return "namespace";
    default: return "misc";
  }
}
