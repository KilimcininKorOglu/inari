// TypeScript interfaces mirroring Inari CLI JSON output.
// Source of truth: internal/output/json.go, internal/core/graph.go,
// internal/output/formatter.go, internal/core/searcher.go.

export interface JsonEnvelope<T> {
  command: string;
  symbol?: string;
  data: T;
  truncated: boolean;
  total: number;
}

export interface Symbol {
  id: string;
  name: string;
  kind: string;
  file_path: string;
  line_start: number;
  line_end: number;
  signature: string | null;
  docstring: string | null;
  parent_id: string | null;
  language: string;
  metadata: string;
}

export interface ClassRelationships {
  extends: string[];
  implements: string[];
  dependencies: string[];
}

export interface CallerInfo {
  name: string;
  count: number;
}

export interface Reference {
  from_id: string;
  from_name: string;
  kind: string;
  file_path: string;
  line: number | null;
  context: string;
  snippet_line?: string;
  snippet?: string[];
}

export interface RefsGroup {
  kind: string;
  refs: Reference[];
}

export interface Dependency {
  name: string;
  file_path: string | null;
  kind: string;
  is_external: boolean;
  depth: number;
}

export interface ImpactNode {
  id: string;
  name: string;
  file_path: string;
  kind: string;
  depth: number;
}

// DepthGroup serializes as [depth, nodes[]] tuple in JSON.
export type DepthGroup = [number, ImpactNode[]];

export interface ImpactResult {
  nodes_by_depth: DepthGroup[];
  test_files: ImpactNode[];
  total_affected: number;
}

export interface CallPathStep {
  symbol_name: string;
  symbol_id: string;
  file_path: string;
  line: number;
  kind: string;
}

export interface CallPath {
  steps: CallPathStep[];
}

export interface TraceResult {
  target: string;
  paths: CallPath[] | null;
}

export interface SearchResult {
  id: string;
  name: string;
  file_path: string;
  kind: string;
  score: number;
  line_start: number;
  line_end: number;
}

export interface MapStats {
  file_count: number;
  symbol_count: number;
  edge_count: number;
  languages: string[];
}

export interface CoreSymbol {
  name: string;
  kind: string;
  file_path: string;
  caller_count: number;
  project?: string;
}

export interface DirStats {
  directory: string;
  file_count: number;
  symbol_count: number;
}

export interface EntrypointInfo {
  name: string;
  file_path: string;
  kind: string;
  method_count: number;
  outgoing_call_count: number;
}

export interface EntrypointGroup {
  Name: string;
  Entries: EntrypointInfo[];
}

export interface MapData {
  stats: MapStats;
  entrypoints: EntrypointGroup[];
  core_symbols: CoreSymbol[];
  architecture: DirStats[];
}

export interface StatusData {
  index_exists: boolean;
  search_available: boolean;
  symbol_count: number;
  file_count: number;
  edge_count: number;
  last_indexed_at: number | null;
  last_indexed_relative: string | null;
}

// Sketch output varies by symbol type. The CLI returns different shapes
// for class, method, interface, generic, and file-path queries.
export interface SketchClassData {
  symbol: Symbol;
  methods: Symbol[];
  caller_counts: Record<string, number>;
  relationships: ClassRelationships;
}

export interface SketchMethodData {
  symbol: Symbol;
  calls: string[];
  called_by: CallerInfo[];
}

export interface SketchFileData {
  symbols: Symbol[];
  caller_counts: Record<string, number>;
}

// Union type for sketch responses — check symbol.kind or presence of
// fields to determine which variant was returned.
export type SketchData = SketchClassData | SketchMethodData | SketchFileData | { symbol: Symbol };

// Watch mode NDJSON events from `inari index --watch --json`.
export interface WatchEvent {
  event: "start" | "reindex" | "stop";
  files_changed?: number;
  symbols_added?: number;
  symbols_removed?: number;
}
