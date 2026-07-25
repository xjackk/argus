import type { DiffResult } from "./types";
import fixture from "../fixture.json";

// ─────────────────────────────────────────────────────────────────────────────
// THE ENGINE INTEGRATION POINT.
//
// Two runtimes render this UI:
//   • Browser (`npm run dev`) — the Wails runtime is absent, so we render the
//     bundled static fixture. This is the fast UI-iteration loop.
//   • Wails app (`wails dev` / packaged) — `window.go.main.App.Diff` is present,
//     so we call the real engine. Identical DiffResult shape (see types.ts),
//     so nothing else in the UI changes.
//
// The guard below keeps BOTH working: browser dev never breaks just because the
// engine binding exists. When a file picker lands, pass real paths to loadDiff.
// ─────────────────────────────────────────────────────────────────────────────

interface WailsBridge {
  main?: { App?: { Diff?(pathA: string, pathB: string): Promise<DiffResult> } };
}

function wailsApp() {
  return (window as unknown as { go?: WailsBridge }).go?.main?.App;
}

/** True when running inside the Wails desktop app (vs. a plain browser). */
export function inWails(): boolean {
  return !!wailsApp()?.Diff;
}

/**
 * Load a DiffResult. Inside Wails with two workbook paths, it calls the real
 * engine (App.Diff). In the browser — or before any paths are chosen — it
 * returns the bundled fixture so `npm run dev` always renders.
 */
export async function loadDiff(pathA?: string, pathB?: string): Promise<DiffResult> {
  const app = wailsApp();
  if (app?.Diff && pathA && pathB) {
    return app.Diff(pathA, pathB);
  }
  return fixture as unknown as DiffResult;
}
