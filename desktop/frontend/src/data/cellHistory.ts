import history from "./commit-history.json";
import type { CellValue } from "./types";

// Per-cell revision timeline — powers State 2's "git log for a cell". Sourced
// from commit-history.json (test-workbooks): every time a cell moved across the
// c01→c07 chain, who caused it, and whether it was authored or computed.
export interface Revision {
  commit: string;
  author: string;
  timestamp: string;
  oldValue: CellValue;
  newValue: CellValue;
  classification: string; // "authored" | "computed"
}

interface CellHistoryEntry {
  ref: string;
  label: string | null;
  revisions: Revision[];
}

const byRef = new Map<string, Revision[]>();
for (const e of (history as { cellHistory?: CellHistoryEntry[] }).cellHistory ?? []) {
  byRef.set(e.ref, e.revisions ?? []);
}

/** Revisions for a fully-qualified ref (e.g. "Returns!B14"), oldest→newest. */
export function revisionsFor(ref: string): Revision[] {
  return byRef.get(ref) ?? [];
}
