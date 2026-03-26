import { execFile } from "child_process";
import type {
  JsonEnvelope,
  SketchData,
  SearchResult,
  Reference,
  Dependency,
  ImpactResult,
  TraceResult,
  MapData,
  StatusData,
  SketchFileData,
  WorkspaceListData,
  WorkspaceIndexData,
  WorkspaceStatusData,
} from "./types";

export class InariError extends Error {
  constructor(
    message: string,
    public readonly stderr: string
  ) {
    super(message);
    this.name = "InariError";
  }
}

export class InariClient {
  constructor(
    private binaryPath: string,
    public readonly workspaceRoot: string
  ) {}

  async sketch(symbol: string): Promise<JsonEnvelope<SketchData>> {
    return this.exec(["sketch", symbol, "--json"]);
  }

  async sketchFile(filePath: string): Promise<JsonEnvelope<SketchFileData>> {
    return this.exec(["sketch", filePath, "--json"]);
  }

  async find(
    query: string,
    kind?: string,
    limit?: number
  ): Promise<JsonEnvelope<SearchResult[]>> {
    const args = ["find", query, "--json"];
    if (kind) {
      args.push("--kind", kind);
    }
    if (limit !== undefined) {
      args.push("--limit", String(limit));
    }
    return this.exec(args);
  }

  async refs(
    symbol: string,
    kind?: string,
    limit?: number
  ): Promise<JsonEnvelope<Reference[]>> {
    const args = ["refs", symbol, "--json"];
    if (kind) {
      args.push("--kind", kind);
    }
    if (limit !== undefined) {
      args.push("--limit", String(limit));
    }
    return this.exec(args);
  }

  async callers(
    symbol: string,
    depth?: number
  ): Promise<JsonEnvelope<Reference[] | ImpactResult>> {
    const args = ["callers", symbol, "--json"];
    if (depth !== undefined) {
      args.push("--depth", String(depth));
    }
    return this.exec(args);
  }

  async deps(
    symbol: string,
    depth?: number
  ): Promise<JsonEnvelope<Dependency[]>> {
    const args = ["deps", symbol, "--json"];
    if (depth !== undefined) {
      args.push("--depth", String(depth));
    }
    return this.exec(args);
  }

  async rdeps(
    symbol: string,
    depth?: number
  ): Promise<JsonEnvelope<ImpactResult>> {
    const args = ["rdeps", symbol, "--json"];
    if (depth !== undefined) {
      args.push("--depth", String(depth));
    }
    return this.exec(args);
  }

  async trace(symbol: string): Promise<JsonEnvelope<TraceResult>> {
    return this.exec(["trace", symbol, "--json"]);
  }

  async map(): Promise<JsonEnvelope<MapData>> {
    return this.exec(["map", "--json"]);
  }

  async status(): Promise<JsonEnvelope<StatusData>> {
    return this.exec(["status", "--json"]);
  }

  async index(full?: boolean): Promise<JsonEnvelope<unknown>> {
    const args = ["index", "--json"];
    if (full) {
      args.push("--full");
    }
    return this.exec(args);
  }

  async init(): Promise<JsonEnvelope<unknown>> {
    return this.exec(["init", "--json"]);
  }

  async entrypoints(): Promise<JsonEnvelope<unknown>> {
    return this.exec(["entrypoints", "--json"]);
  }

  async source(symbol: string): Promise<JsonEnvelope<unknown>> {
    return this.exec(["source", symbol, "--json"]);
  }

  async similar(symbol: string): Promise<JsonEnvelope<SearchResult[]>> {
    return this.exec(["similar", symbol, "--json"]);
  }

  async workspaceList(): Promise<JsonEnvelope<WorkspaceListData>> {
    return this.exec(["workspace", "list", "--json"]);
  }

  async workspaceIndex(full?: boolean): Promise<JsonEnvelope<WorkspaceIndexData>> {
    const args = ["workspace", "index", "--json"];
    if (full) {
      args.push("--full");
    }
    return this.exec(args);
  }

  async workspaceStatus(): Promise<JsonEnvelope<WorkspaceStatusData>> {
    return this.exec(["status", "--json"]);
  }

  async version(): Promise<string> {
    return new Promise((resolve, reject) => {
      execFile(
        this.binaryPath,
        ["--version"],
        { cwd: this.workspaceRoot, timeout: 5_000 },
        (error, stdout) => {
          if (error) {
            reject(new InariError(error.message, ""));
            return;
          }
          resolve(stdout.trim());
        }
      );
    });
  }

  private exec<T>(args: string[]): Promise<JsonEnvelope<T>> {
    return new Promise((resolve, reject) => {
      execFile(
        this.binaryPath,
        args,
        {
          cwd: this.workspaceRoot,
          timeout: 30_000,
          maxBuffer: 10 * 1024 * 1024,
        },
        (error, stdout, stderr) => {
          if (error) {
            reject(new InariError(error.message, stderr));
            return;
          }
          try {
            resolve(JSON.parse(stdout) as JsonEnvelope<T>);
          } catch {
            reject(
              new InariError(
                `Failed to parse JSON: ${stdout.slice(0, 200)}`,
                stderr
              )
            );
          }
        }
      );
    });
  }
}
