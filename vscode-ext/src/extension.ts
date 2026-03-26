import * as vscode from "vscode";
import { InariClient } from "./inari";
import { getConfig } from "./config";
import { StatusBarManager } from "./statusBar";
import { IndexManager } from "./indexManager";
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

let indexManager: IndexManager | undefined;

export async function activate(
  context: vscode.ExtensionContext
): Promise<void> {
  const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
  if (!workspaceFolder) return;

  const workspaceRoot = workspaceFolder.uri.fsPath;
  const config = getConfig();

  const outputChannel = vscode.window.createOutputChannel("Inari");
  context.subscriptions.push(outputChannel);

  const client = new InariClient(config.binaryPath, workspaceRoot);

  // Verify CLI is available.
  try {
    const ver = await client.version();
    outputChannel.appendLine(`[Inari] CLI version: ${ver}`);
  } catch {
    vscode.window.showWarningMessage(
      'Inari CLI not found. Install from https://github.com/KilimcininKorOglu/inari or set "inari.path" in settings.'
    );
    outputChannel.appendLine("[Inari] CLI not found. Extension features disabled.");
    return;
  }

  // Set context for welcome view.
  try {
    const statusResult = await client.status();
    await vscode.commands.executeCommand(
      "setContext",
      "inari.indexExists",
      statusResult.data.index_exists
    );
  } catch {
    await vscode.commands.executeCommand("setContext", "inari.indexExists", false);
  }

  // Status bar.
  const statusBar = new StatusBarManager(client);
  context.subscriptions.push(statusBar);
  await statusBar.update();
  statusBar.startPolling();

  // Index manager.
  indexManager = new IndexManager(
    client,
    statusBar,
    outputChannel,
    config.binaryPath,
    workspaceRoot,
    config.indexMode
  );
  context.subscriptions.push(indexManager);
  indexManager.start();

  // Hover provider.
  if (config.hoverEnabled) {
    const hoverProvider = new InariHoverProvider(client);
    context.subscriptions.push(
      vscode.languages.registerHoverProvider({ scheme: "file" }, hoverProvider)
    );
  }

  // CodeLens provider.
  if (config.codeLensEnabled && config.codeLensCallers) {
    const codeLensProvider = new InariCodeLensProvider(client, workspaceRoot);
    context.subscriptions.push(
      vscode.languages.registerCodeLensProvider(
        { scheme: "file" },
        codeLensProvider
      )
    );
  }

  // TreeView providers.
  const symbolTree = new SymbolTreeProvider(client, workspaceRoot);
  const depsTree = new DepsTreeProvider(client, workspaceRoot);
  const callersTree = new CallersTreeProvider(client, workspaceRoot);

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
  context.subscriptions.push(registerSketchCommand(client, outputChannel));
  context.subscriptions.push(registerFindCommand(client, workspaceRoot));
  context.subscriptions.push(registerRefsCommand(client, workspaceRoot));
  context.subscriptions.push(registerDepsCommand(client, outputChannel));
  context.subscriptions.push(registerCallersCommand(client, workspaceRoot));
  context.subscriptions.push(registerTraceCommand(client, outputChannel));
  context.subscriptions.push(registerMapCommand(client, outputChannel));
  for (const cmd of registerIndexCommands(client, indexManager, config.binaryPath)) {
    context.subscriptions.push(cmd);
  }

  // React to config changes.
  context.subscriptions.push(
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("inari.indexMode") && indexManager) {
        const newConfig = getConfig();
        indexManager.switchMode(newConfig.indexMode);
      }
    })
  );

  outputChannel.appendLine(`[Inari] Extension activated. Index mode: ${config.indexMode}`);
}

export function deactivate(): void {
  if (indexManager) {
    indexManager.dispose();
    indexManager = undefined;
  }
}
