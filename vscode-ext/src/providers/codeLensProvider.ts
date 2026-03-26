import * as vscode from "vscode";
import type { InariClient } from "../inari";
import type { SketchFileData, Symbol as InariSymbol } from "../types";

interface FileCache {
  version: number;
  symbols: InariSymbol[];
  callerCounts: Record<string, number>;
}

export class InariCodeLensProvider implements vscode.CodeLensProvider {
  private cache = new Map<string, FileCache>();
  private onDidChangeEmitter = new vscode.EventEmitter<void>();
  readonly onDidChangeCodeLenses = this.onDidChangeEmitter.event;

  constructor(
    private client: InariClient,
    private workspaceRoot: string
  ) {}

  refresh(): void {
    this.cache.clear();
    this.onDidChangeEmitter.fire();
  }

  async provideCodeLenses(
    document: vscode.TextDocument,
    token: vscode.CancellationToken
  ): Promise<vscode.CodeLens[]> {
    const cached = this.cache.get(document.uri.toString());
    if (cached && cached.version === document.version) {
      return this.buildLenses(cached);
    }

    const relativePath = vscode.workspace.asRelativePath(document.uri, false);

    try {
      const result = await this.client.sketchFile(relativePath);
      if (token.isCancellationRequested) return [];

      const data = result.data as SketchFileData;
      if (!data.symbols) return [];

      const entry: FileCache = {
        version: document.version,
        symbols: data.symbols,
        callerCounts: data.caller_counts ?? {},
      };
      this.cache.set(document.uri.toString(), entry);
      return this.buildLenses(entry);
    } catch {
      return [];
    }
  }

  resolveCodeLens(codeLens: vscode.CodeLens): vscode.CodeLens {
    return codeLens;
  }

  private buildLenses(data: FileCache): vscode.CodeLens[] {
    const lenses: vscode.CodeLens[] = [];

    for (const sym of data.symbols) {
      if (!isLensWorthy(sym.kind)) continue;

      const count = data.callerCounts[sym.id] ?? 0;
      if (count === 0) continue;

      const line = sym.line_start - 1;
      const range = new vscode.Range(line, 0, line, 0);
      const title = count === 1 ? "1 caller" : `${count} callers`;

      lenses.push(
        new vscode.CodeLens(range, {
          title,
          command: "inari.showCallers",
          arguments: [sym.name],
          tooltip: `Show all callers of ${sym.name}`,
        })
      );
    }

    return lenses;
  }
}

function isLensWorthy(kind: string): boolean {
  return (
    kind === "function" ||
    kind === "method" ||
    kind === "class" ||
    kind === "interface" ||
    kind === "type"
  );
}
