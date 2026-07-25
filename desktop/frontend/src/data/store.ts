import type { DiffResult } from "./types";
import type { CommitRow } from "./history";
import { relativeTime } from "./refs";

// The live store written by the capture daemon (cmd/argusd). When a daemon is
// running, its output is served at /store; when it isn't, these return null and
// the app falls back to the bundled c01→c07 chain. Same UI either way.

interface StoreCommit {
  id: string;
  file: string;
  author: string;
  message: string;
  timestamp: string;
  parent: string;
  authoredCount: number;
  computedCount: number;
  anomaly: boolean;
  base: boolean;
}

const STORE = "/store";

/** Live commit history from the daemon store, or null if no daemon is running. */
export async function fetchLiveHistory(): Promise<CommitRow[] | null> {
  try {
    const res = await fetch(`${STORE}/history.json`, { cache: "no-store" });
    if (!res.ok) return null;
    const data = (await res.json()) as { commits?: StoreCommit[] };
    if (!data.commits || data.commits.length === 0) return null;
    const now = new Date();
    return data.commits
      .slice()
      .reverse() // newest first
      .map((c) => ({
        id: c.id,
        file: c.file,
        author: c.author,
        message: c.message,
        when: relativeTime(c.timestamp, now),
        authoredCount: c.authoredCount,
        computedCount: c.computedCount,
        base: c.base,
        // Anomaly UI disabled for the demo (see diffs.ts) — force the rail badge
        // off even when the daemon store recorded one.
        anomaly: false,
      }));
  } catch {
    return null; // no daemon / not served → caller uses the bundled chain
  }
}

/** A single commit's diff from the live store, or null on miss. */
export async function fetchLiveDiff(id: string): Promise<DiffResult | null> {
  try {
    const res = await fetch(`${STORE}/diffs/${id}.json`, { cache: "no-store" });
    if (!res.ok) return null;
    const diff = (await res.json()) as DiffResult;
    // Anomaly UI disabled for the demo (see diffs.ts) — strip anomalies from
    // live daemon diffs too, so dropped-in workbooks don't surface false alarms.
    return { ...diff, anomalies: [] };
  } catch {
    return null;
  }
}
