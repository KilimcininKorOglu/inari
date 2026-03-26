import * as vscode from "vscode";

export type IndexMode = "onSave" | "watch" | "manual";

export interface InariConfig {
  binaryPath: string;
  indexMode: IndexMode;
  hoverEnabled: boolean;
  codeLensEnabled: boolean;
  codeLensCallers: boolean;
}

export function getConfig(folderUri?: vscode.Uri): InariConfig {
  const cfg = vscode.workspace.getConfiguration("inari", folderUri ?? null);
  return {
    binaryPath: cfg.get<string>("path", "inari"),
    indexMode: cfg.get<IndexMode>("indexMode", "onSave"),
    hoverEnabled: cfg.get<boolean>("hoverEnabled", true),
    codeLensEnabled: cfg.get<boolean>("codeLensEnabled", true),
    codeLensCallers: cfg.get<boolean>("codeLensCallers", true),
  };
}
