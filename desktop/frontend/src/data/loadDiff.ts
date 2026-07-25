import type { DiffResult } from "./types";
import fixture from "../fixture.json";

// ─────────────────────────────────────────────────────────────────────────────
// THE ENGINE INTEGRATION POINT.
//
// Today this returns the bundled static fixture. When the engine is ready, its
// stdout JSON has the identical DiffResult shape (see DATA-CONTRACT.md), so the
// ONLY change required is the body of this one function — e.g.:
//
//     export async function loadDiff(): Promise<DiffResult> {
//       return (await window.go.main.App.Diff(pathA, pathB)) as DiffResult;
//     }
//
// Nothing else in the UI needs to change.
// ─────────────────────────────────────────────────────────────────────────────
export async function loadDiff(): Promise<DiffResult> {
  return fixture as unknown as DiffResult;
}
