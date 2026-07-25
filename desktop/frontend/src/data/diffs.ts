import type { DiffResult } from "./types";
import fixture from "../fixture.json"; // curated c06 diff (v1→v2), carries narrative
import c02 from "./history/c02.json";
import c03 from "./history/c03.json";
import c04 from "./history/c04.json";
import c05 from "./history/c05.json";
import c07 from "./history/c07.json";

// Pre-generated DiffResults for each commit in the c01→c07 chain — real engine
// output (argus-diff on each consecutive pair), bundled so the History rail
// works entirely in the browser. c01 is the base (nothing before it → null).
// c06 uses the curated fixture (identical change to c05→c06, but hand-narrated).
const DIFFS: Record<string, unknown> = { c02, c03, c04, c05, c06: fixture, c07 };

/**
 * The diff a commit introduced (vs. its parent). Returns null for the base
 * commit c01. In the Wails app this will later call the live engine instead.
 */
export function diffForCommit(id: string): DiffResult | null {
  const d = DIFFS[id];
  return d ? (d as unknown as DiffResult) : null;
}
