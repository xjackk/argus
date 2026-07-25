import type { CellChange } from "./types";
import { canonRef } from "./refs";

/**
 * The full path of changed cells from an authored origin down to `here`,
 * following each cell's `dependsOn` precedents (over changed cells only).
 * Returns [origin, …intermediate hops…, here] so a reviewer can trace the
 * whole calculation, e.g. Assumptions!B5 → Returns!B9 → Returns!B11 → Returns!B14.
 *
 * Falls back to [origin, here] if no path through changed cells is found
 * (e.g. every intermediate cell was unchanged and thus isn't in the diff).
 */
export function dependencyChain(
  hereRef: string,
  originRef: string,
  changeByRef: Map<string, CellChange>
): string[] {
  const start = canonRef(hereRef);
  const target = canonRef(originRef);
  if (start === target) return [start];

  // Breadth-first from `here` back toward the origin → shortest causal path.
  const queue: string[][] = [[start]];
  const seen = new Set<string>([start]);
  while (queue.length) {
    const path = queue.shift() as string[];
    const tail = path[path.length - 1];
    const ch = changeByRef.get(tail);
    if (!ch) continue;
    for (const depRaw of ch.dependsOn) {
      const dep = canonRef(depRaw);
      if (dep === target) return [...path, dep].reverse();
      if (!seen.has(dep) && changeByRef.has(dep)) {
        seen.add(dep);
        queue.push([...path, dep]);
      }
    }
  }
  return [target, start]; // no changed-cell path found → show endpoints only
}
