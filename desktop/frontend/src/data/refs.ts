// Helpers for the fully-qualified "Sheet!Coord" refs used across the contract.
// Sheet names with spaces/& are quoted by the engine: "'P&L'!B6".
//
// The UI keys every lookup map (anomalies, cascades, ripples, cell history) by
// a CANONICAL, UNQUOTED "Sheet!Coord". Build keys with `qualify(sheet, coord)`
// (the UI always has an unquoted sheet name), and normalize any ref that came
// from the engine (which may be quoted) with `canonRef(ref)` before use. This
// is the single fix for the quoted-vs-unquoted mismatch that otherwise makes a
// 'P&L' anomaly badge or cascade silently fail to resolve.

export function splitRef(ref: string): { sheet: string; coord: string } {
  const bang = ref.lastIndexOf("!");
  if (bang === -1) return { sheet: "", coord: ref };
  let sheet = ref.slice(0, bang);
  const coord = ref.slice(bang + 1);
  if (sheet.startsWith("'") && sheet.endsWith("'")) sheet = sheet.slice(1, -1);
  return { sheet, coord };
}

/** Canonical unquoted lookup key from an unquoted sheet name + coord. */
export function qualify(sheet: string, coord: string): string {
  return `${sheet}!${coord}`;
}

/** Normalize any (possibly quoted) engine ref to the canonical unquoted key. */
export function canonRef(ref: string): string {
  const { sheet, coord } = splitRef(ref);
  return qualify(sheet, coord);
}

// Column index (1-based) → spreadsheet letters (1 → A, 27 → AA).
export function colLetter(col: number): string {
  let n = col;
  let s = "";
  while (n > 0) {
    const r = (n - 1) % 26;
    s = String.fromCharCode(65 + r) + s;
    n = Math.floor((n - 1) / 26);
  }
  return s;
}

// Relative time from an ISO timestamp, anchored to now.
export function relativeTime(iso: string, now: Date = new Date()): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const secs = Math.round((now.getTime() - then) / 1000);
  const mins = Math.round(secs / 60);
  const hours = Math.round(mins / 60);
  const days = Math.round(hours / 24);
  if (secs < 60) return "just now";
  if (mins < 60) return `${mins}m ago`;
  if (hours < 24) return `${hours}h ago`;
  if (days === 1) return "yesterday";
  if (days < 7) return `${days}d ago`;
  const weeks = Math.round(days / 7);
  if (weeks < 5) return `${weeks}w ago`;
  return new Date(iso).toLocaleDateString("en-US", { month: "short", day: "numeric" });
}
