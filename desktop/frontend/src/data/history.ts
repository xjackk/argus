// Hardcoded version history for the left rail (HACKATHON.md: "Hardcode 3–4 fake
// history rows. Nobody will know."). Author / message / counts are cribbed from
// test-workbooks/commit-history.json for realism. Only the SELECTED row maps to
// the loaded DiffResult; the rest are decoration.

export interface CommitRow {
  id: string;
  author: string;
  message: string;
  description?: string;
  when: string; // display-ready relative time
  authoredCount: number;
  computedCount: number;
  structural?: boolean; // e.g. a row was inserted — no cascade counts
  base?: boolean;
  signedOff?: string; // signer name, if signed off
  anomaly?: boolean;
  // The row whose diff is currently loaded. Swapping the engine in means this
  // row's counts come from the real DiffResult.summary instead.
  isCurrentDiff?: boolean;
}

export const COMMIT_HISTORY: CommitRow[] = [
  {
    id: "c07",
    author: "M. Rivera",
    message: "Manual Exit EV override pending diligence",
    description:
      "Hardcoded Exit EV to 2,100 while we confirm the comp set. Will re-link to the formula before IC.",
    when: "20m ago",
    authoredCount: 1,
    computedCount: 3,
    anomaly: true,
  },
  {
    id: "c06",
    author: "S. Patel (VP)",
    message: "Marked exit multiple down to 9.5x per comp set",
    description:
      "Comp set refresh puts peers at 9.2–9.8x. Marking to 9.5x ahead of IC. Flagging that this moves IRR below our 25% threshold.",
    when: "2h ago",
    authoredCount: 1,
    computedCount: 4,
    signedOff: "S. Patel (VP)",
    isCurrentDiff: true,
  },
  {
    id: "c05",
    author: "M. Rivera",
    message: "Added stock-based comp line to P&L",
    when: "1d ago",
    authoredCount: 42,
    computedCount: 0,
    structural: true,
  },
  {
    id: "c04",
    author: "A. Chen (Modeling)",
    message: "Debt repricing: senior interest to 8.25%",
    when: "2d ago",
    authoredCount: 2,
    computedCount: 35,
  },
  {
    id: "c03",
    author: "A. Chen (Modeling)",
    message: "Tightened exit EBITDA margin to 21%",
    when: "3d ago",
    authoredCount: 1,
    computedCount: 35,
  },
  {
    id: "c02",
    author: "M. Rivera",
    message: "Revenue growth case to 9.5%",
    when: "4d ago",
    authoredCount: 1,
    computedCount: 40,
  },
  {
    id: "c01",
    author: "J. Killilea",
    message: "Initial model shared",
    when: "6d ago",
    authoredCount: 0,
    computedCount: 0,
    base: true,
  },
];
