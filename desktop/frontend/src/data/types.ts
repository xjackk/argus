// TypeScript model mirroring the Go DiffResult types in DATA-CONTRACT.md.
// The engine's stdout JSON has exactly this shape, so the real output drops in
// later with no other changes (see loadDiff.ts for the single swap point).

export type Classification = "authored" | "computed";

export type CellValue = number | string | boolean | null;

export interface VersionMeta {
  label: string;
  path: string;
  committedAt: string;
  author: string;
}

export interface Summary {
  authoredCount: number;
  computedCount: number;
  sheetsAffected: string[];
  narrative: string | null; // null if AI step skipped — UI must tolerate
}

export interface CellChange {
  coord: string; // sheet-local A1, e.g. "B9"
  row: number; // 1-based
  col: number; // 1-based
  label: string | null;
  classification: Classification;
  oldValue: CellValue;
  newValue: CellValue;
  oldFormula: string | null;
  newFormula: string | null;
  displayFormat: string; // Excel number format, falls back to "General"
  dependsOn: string[];
  dependents: string[];
  causedBy: string[];
  magnitude: number | null;
}

export interface SheetDiff {
  name: string;
  changes: CellChange[];
  rowsInserted: number[];
  rowsDeleted: number[];
}

export interface Mover {
  ref: string;
  label: string | null;
  oldValue: CellValue;
  newValue: CellValue;
  magnitude: number | null;
}

export interface Cascade {
  origin: string; // fully-qualified "Sheet!Coord"
  originLabel: string | null;
  oldValue: CellValue;
  newValue: CellValue;
  affectedCount: number;
  affected: string[];
  topMovers: Mover[];
}

export type AnomalyType =
  | "formula_replaced_by_constant"
  | "large_magnitude_change"
  | "formula_inconsistent_in_row";

export type Severity = "high" | "medium" | "low";

export interface Anomaly {
  type: AnomalyType | string;
  ref: string;
  label: string | null;
  severity: Severity | string;
  message: string;
  oldFormula?: string | null;
  newValue?: CellValue;
}

export interface DiffResult {
  schemaVersion: number;
  from: VersionMeta;
  to: VersionMeta;
  summary: Summary;
  sheets: SheetDiff[];
  cascades: Cascade[];
  anomalies: Anomaly[];
}
