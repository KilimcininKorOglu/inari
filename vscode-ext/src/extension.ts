import * as vscode from "vscode";
import { getConfig } from "./config";
import { WorkspaceManager } from "./workspaceManager";
import { InariHoverProvider } from "./providers/hoverProvider";
import { InariCodeLensProvider } from "./providers/codeLensProvider";
import { SymbolTreeProvider } from "./providers/symbolTreeProvider";
import { DepsTreeProvider } from "./providers/depsTreeProvider";
import { CallersTreeProvider } from "./providers/callersTreeProvider";
import { registerSketchCommand } from "./commands/sketch";
import { registerFindCommand } from "./commands/find";
import { registerRefsCommand } from "./commands/refs";
import { registerDepsCommand } from "./commands/deps";
import { registerCallersCommand } from "./commands/callers";
import { registerTraceCommand } from "./commands/trace";
import { registerMapCommand } from "./commands/map";
import { registerIndexCommands } from "./commands/index";

export async function activate(
  context: vscode.ExtensionContext
): Promise<void> {
  const folders = vscode.workspace.workspaceFolders;
  if (!folders || folders.length === 0) return;

  const outputChannel = vscode.window.createOutputChannel("Inari");
  context.subscriptions.push(outputChannel);

  // Create workspace manager (handles single or multi-root).
  const wm = new WorkspaceManager(outputChannel);
  context.subscriptions.push(wm);

  // Verify CLI and start index managers.
  const ok = await wm.initialize();
  if (!ok) {
    vscode.window.showWarningMessage(
      'Inari CLI not found. Install from https://github.com/KilimcininKorOglu/inari or set "inari.path" in settings.'
    );
    outputChannel.appendLine("[Inari] CLI not found. Extension features disabled.");
    return;
  }

  // Set context for welcome view.
  try {
    const firstClient = wm.getAllClients()[0];
    const statusResult = await firstClient.status();
    await vscode.commands.executeCommand(
      "setContext",
      "inari.indexExists",
      statusResult.data.index_exists
    );
  } catch {
    await vscode.commands.executeCommand("setContext", "inari.indexExists", false);
  }

  // Hover provider.
  const config = getConfig(folders[0].uri);
  if (config.hoverEnabled) {
    context.subscriptions.push(
      vscode.languages.registerHoverProvider({ scheme: "file" }, new InariHoverProvider(wm))
    );
  }

  // CodeLens provider.
  if (config.codeLensEnabled && config.codeLensCallers) {
    context.subscriptions.push(
      vscode.languages.registerCodeLensProvider(
        { scheme: "file" },
        new InariCodeLensProvider(wm)
      )
    );
  }

  // TreeView providers.
  const symbolTree = new SymbolTreeProvider(wm);
  const depsTree = new DepsTreeProvider(wm);
  const callersTree = new CallersTreeProvider(wm);

  context.subscriptions.push(
    vscode.window.createTreeView("inari.symbolExplorer", {
      treeDataProvider: symbolTree,
      showCollapseAll: true,
    })
  );
  context.subscriptions.push(
    vscode.window.createTreeView("inari.depsTree", {
      treeDataProvider: depsTree,
      showCollapseAll: true,
    })
  );
  context.subscriptions.push(
    vscode.window.createTreeView("inari.callersTree", {
      treeDataProvider: callersTree,
      showCollapseAll: true,
    })
  );

  // Commands.
  context.subscriptions.push(registerSketchCommand(wm, outputChannel));
  context.subscriptions.push(registerFindCommand(wm));
  context.subscriptions.push(registerRefsCommand(wm));
  context.subscriptions.push(registerDepsCommand(wm, outputChannel));
  context.subscriptions.push(registerCallersCommand(wm));
  context.subscriptions.push(registerTraceCommand(wm, outputChannel));
  context.subscriptions.push(registerMapCommand(wm, outputChannel));
  for (const cmd of registerIndexCommands(wm, config.binaryPath)) {
    context.subscriptions.push(cmd);
  }

  // React to config changes.
  context.subscriptions.push(
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("inari.indexMode")) {
        for (const im of wm.getAllIndexManagers()) {
          const newConfig = getConfig();
          im.switchMode(newConfig.indexMode);
        }
      }
    })
  );

  const mode = wm.isMultiRoot ? "multi-root" : "single-project";
  outputChannel.appendLine(`[Inari] Extension activated. Mode: ${mode}, ${wm.getAllClients().length} project(s).`);
}

export function deactivate(): void {
  // WorkspaceManager.dispose() is called automatically via context.subscriptions.
}
