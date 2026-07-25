import type { Classification } from "./types";

// Single source of truth for the authored/computed distinction, replacing the
// `=== "authored"` literal and the `? "a" : "c"` className scattered across
// ~10 sites. Anything with a `classification` field works (CellChange, Revision).

export function isAuthored(x: { classification: Classification }): boolean {
  return x.classification === "authored";
}

/** className suffix used by classification-colored dots/cells: "a" | "c". */
export function classDot(classification: Classification): "a" | "c" {
  return classification === "authored" ? "a" : "c";
}
