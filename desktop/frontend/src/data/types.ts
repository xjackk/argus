// TypeScript model mirroring the Go DiffResult types in DATA-CONTRACT.md.
// The engine's stdout JSON has exactly this shape, so real output drops in with
// no other changes (data/diffs.ts is the single swap point to the live engine).

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
  type: AnomalyType;
  ref: string;
  label: string | null;
  severity: Severity;
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
  /**
   * Every sheet in the new workbook, in Excel's own tab order — unchanged
   * sheets included (they never appear in `sheets`). Powers the bottom tab
   * strip. Optional: diffs generated before the engine emitted this field omit
   * it, so consumers must fall back to `sheets`.
   */
  allSheets?: string[];
}
