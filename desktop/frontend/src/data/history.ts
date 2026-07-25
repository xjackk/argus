import manifest from "./commit-history.json";
import { diffForCommit } from "./diffs";
import { relativeTime } from "./refs";

// The version rail's rows. IDENTITY (author, message, timestamp, sign-off) comes
// from the commit-history.json manifest — the single source of truth. COUNTS and
// the anomaly badge come from the actual loaded DiffResult, so the rail can never
// disagree with the grid it links to (the old hardcoded COMMIT_HISTORY drifted:
// it claimed c05 = "42 authored, structural" while the real diff is 49 + anomaly).

export interface CommitRow {
  id: string;
  file: string; // the workbook (Excel file) this commit belongs to
  author: string;
  message: string;
  description?: string;
  when: string; // display-ready relative time
  authoredCount: number;
  computedCount: number;
  base?: boolean; // the initial commit — nothing before it
  signedOff?: string; // signer name, if signed off
  anomaly?: boolean;
}

interface ManifestCommit {
  id: string;
  author: string;
  message: string;
  timestamp: string;
  parent: string | null;
  signedOff?: { by: string; at: string } | null;
}

// Reviewer-rationale text isn't in the manifest; supplement the commits that
// carry a written justification (shown in the commit-description panel).
const DESCRIPTIONS: Record<string, string> = {
  c06: "Comp set refresh puts peers at 9.2–9.8x. Marking to 9.5x ahead of IC. Flagging that this moves IRR below our 25% threshold.",
  c07: "Hardcoded Exit EV to 2,100 while we confirm the comp set. Will re-link to the formula before IC.",
};

// Demo "now" anchor so relative times read like the mockup (last commit ~20m ago).
const NOW = new Date("2026-07-09T14:32:00Z");

export const COMMIT_HISTORY: CommitRow[] = (manifest.commits as ManifestCommit[])
  .slice()
  .reverse() // newest first
  .map((c) => {
    const diff = diffForCommit(c.id);
    return {
      id: c.id,
      // The bundled chain is one model's history, so it's a single workbook.
      file: "Project Atlas — LBO",
      author: c.author,
      message: c.message,
      description: DESCRIPTIONS[c.id],
      when: relativeTime(c.timestamp, NOW),
      authoredCount: diff?.summary.authoredCount ?? 0,
      computedCount: diff?.summary.computedCount ?? 0,
      base: c.parent === null,
      signedOff: c.signedOff?.by,
      anomaly: (diff?.anomalies.length ?? 0) > 0,
    };
  });
