import * as vscode from "vscode";
import type { WorkspaceManager } from "../workspaceManager";
import type { SketchClassData, SketchMethodData, Symbol as InariSymbol } from "../types";

interface CacheEntry {
  data: unknown;
  timestamp: number;
}

const CACHE_TTL_MS = 10_000;

export class InariHoverProvider implements vscode.HoverProvider {
  private cache = new Map<string, CacheEntry>();

  constructor(private wm: WorkspaceManager) {}

  async provideHover(
    document: vscode.TextDocument,
    position: vscode.Position,
    token: vscode.CancellationToken
  ): Promise<vscode.Hover | null> {
    const wordRange = document.getWordRangeAtPosition(position, /[a-zA-Z_]\w*/);
    if (!wordRange) return null;

    const word = document.getText(wordRange);
    if (word.length < 2) return null;

    if (token.isCancellationRequested) return null;

    const client = this.wm.getClientForUri(document.uri);
    if (!client) return null;

    const cacheKey = `${client.workspaceRoot}:${word}`;
    const cached = this.cache.get(cacheKey);
    if (cached && Date.now() - cached.timestamp < CACHE_TTL_MS) {
      return this.buildHover(cached.data);
    }

    try {
      const result = await client.sketch(word);
      if (token.isCancellationRequested) return null;

      this.cache.set(cacheKey, { data: result.data, timestamp: Date.now() });
      return this.buildHover(result.data);
    } catch {
      return null;
    }
  }

  private buildHover(data: unknown): vscode.Hover | null {
    const d = data as Record<string, unknown>;
    if (!d) return null;

    const md = new vscode.MarkdownString();
    md.isTrusted = true;

    if ("methods" in d && "relationships" in d) {
      this.renderClassSketch(md, d as unknown as SketchClassData);
    } else if ("calls" in d || "called_by" in d) {
      this.renderMethodSketch(md, d as unknown as SketchMethodData);
    } else if ("symbol" in d) {
      this.renderGenericSketch(md, (d as { symbol: InariSymbol }).symbol);
    } else {
      return null;
    }

    return new vscode.Hover(md);
  }

  private renderClassSketch(md: vscode.MarkdownString, data: SketchClassData): void {
    const s = data.symbol;
    md.appendMarkdown(
      `**${s.name}** \`${s.kind}\` \u2014 \`${s.file_path}:${s.line_start}-${s.line_end}\`\n\n`
    );

    const rel = data.relationships;
    if (rel.extends.length > 0) {
      md.appendMarkdown(`**extends:** ${rel.extends.join(", ")}\n\n`);
    }
    if (rel.implements.length > 0) {
      md.appendMarkdown(`**implements:** ${rel.implements.join(", ")}\n\n`);
    }
    if (rel.dependencies.length > 0) {
      const deps = rel.dependencies.slice(0, 5).join(", ");
      const more = rel.dependencies.length > 5 ? ` (+${rel.dependencies.length - 5})` : "";
      md.appendMarkdown(`**deps:** ${deps}${more}\n\n`);
    }

    if (data.methods.length > 0) {
      md.appendMarkdown("| Method | Callers |\n|---|---|\n");
      for (const m of data.methods.slice(0, 8)) {
        const count = data.caller_counts[m.id] ?? 0;
        const label = count > 0 ? String(count) : "internal";
        md.appendMarkdown(`| ${m.name} | ${label} |\n`);
      }
      if (data.methods.length > 8) {
        md.appendMarkdown(`\n*...and ${data.methods.length - 8} more methods*\n`);
      }
    }

    if (s.docstring) {
      md.appendMarkdown(`\n---\n> ${s.docstring.split("\n")[0]}\n`);
    }
  }

  private renderMethodSketch(md: vscode.MarkdownString, data: SketchMethodData): void {
    const s = data.symbol;
    md.appendMarkdown(
      `**${s.name}** \`${s.kind}\` \u2014 \`${s.file_path}:${s.line_start}-${s.line_end}\`\n\n`
    );

    if (s.signature) {
      md.appendCodeblock(s.signature, s.language);
    }

    if (data.called_by && data.called_by.length > 0) {
      const callerList = data.called_by
        .slice(0, 5)
        .map((c) => `${c.name} (${c.count}x)`)
        .join(", ");
      md.appendMarkdown(`\n**callers:** ${callerList}\n\n`);
    }

    if (data.calls && data.calls.length > 0) {
      const callList = data.calls.slice(0, 5).join(", ");
      const more = data.calls.length > 5 ? ` (+${data.calls.length - 5})` : "";
      md.appendMarkdown(`**calls:** ${callList}${more}\n`);
    }

    if (s.docstring) {
      md.appendMarkdown(`\n---\n> ${s.docstring.split("\n")[0]}\n`);
    }
  }

  private renderGenericSketch(md: vscode.MarkdownString, s: InariSymbol): void {
    md.appendMarkdown(
      `**${s.name}** \`${s.kind}\` \u2014 \`${s.file_path}:${s.line_start}-${s.line_end}\`\n\n`
    );
    if (s.signature) {
      md.appendCodeblock(s.signature, s.language);
    }
    if (s.docstring) {
      md.appendMarkdown(`\n> ${s.docstring.split("\n")[0]}\n`);
    }
  }
}
