import { useEffect, useMemo, useState } from "react";
import { diffForCommit } from "./data/diffs";
import { COMMIT_HISTORY } from "./data/history";
import { qualify, canonRef, splitRef } from "./data/refs";
import { isAuthored } from "./data/classify";
import type { DiffResult, CellChange, Cascade, Anomaly } from "./data/types";

// The commit shown on open — the signed-off exit-multiple cascade (the demo
// centerpiece / UI-SPEC State 1). Falls back to the newest commit.
const DEFAULT_COMMIT =
  COMMIT_HISTORY.find((c) => c.id === "c06")?.id ?? COMMIT_HISTORY[0].id;
import { TopBar } from "./components/TopBar";
import { VersionRail } from "./components/VersionRail";
import { DiffColumn } from "./components/DiffColumn";
import { HoverCard, HoverState } from "./components/HoverCard";
import type { ViewMode } from "./components/VirtualGrid";

export default function App() {
  const [diff, setDiff] = useState<DiffResult | null>(null);
  const [selectedCommitId, setSelectedCommitId] = useState(DEFAULT_COMMIT);
  const [selectedSheet, setSelectedSheet] = useState<string>("Returns");
  const [mode, setMode] = useState<ViewMode>("cascade"); // State 1: cascade active
  const [hover, setHover] = useState<HoverState | null>(null);
  const [selectedCell, setSelectedCell] = useState<{
    change: CellChange;
    sheet: string;
  } | null>(null);

  // ── Load the selected commit's diff. Reloads whenever a different commit is
  // picked in the rail. Clears any open cell and keeps the sheet selection if
  // that sheet still exists in the new diff, else jumps to the first one. ──
  useEffect(() => {
    const d = diffForCommit(selectedCommitId);
    setDiff(d);
    setSelectedCell(null);
    if (d && d.sheets.length) {
      setSelectedSheet((prev) =>
        d.sheets.some((s) => s.name === prev) ? prev : d.sheets[0].name
      );
    }
  }, [selectedCommitId]);

  // Lookup maps derived once per diff.
  const { changeByRef, cascadeByOrigin, anomaliesByRef } = useMemo(() => {
    const byRef = new Map<string, CellChange>();
    const casc = new Map<string, Cascade>();
    const anom = new Map<string, Anomaly>();
    if (diff) {
      // Keys are canonical + unquoted; engine refs (origin/ref, possibly quoted)
      // are normalized so 'P&L'-style refs resolve. See data/refs.ts.
      for (const s of diff.sheets)
        for (const ch of s.changes) byRef.set(qualify(s.name, ch.coord), ch);
      for (const c of diff.cascades) casc.set(canonRef(c.origin), c);
      for (const a of diff.anomalies) anom.set(canonRef(a.ref), a);
    }
    return { changeByRef: byRef, cascadeByOrigin: casc, anomaliesByRef: anom };
  }, [diff]);

  // Highlight the cascade blast-radius when an authored cell is selected.
  const rippled = useMemo(() => {
    const set = new Set<string>();
    if (selectedCell && isAuthored(selectedCell.change)) {
      const origin = qualify(selectedCell.sheet, selectedCell.change.coord);
      const c = cascadeByOrigin.get(origin);
      if (c) c.affected.forEach((r) => set.add(canonRef(r)));
    }
    return set;
  }, [selectedCell, cascadeByOrigin]);

  const commit = useMemo(
    () =>
      COMMIT_HISTORY.find((c) => c.id === selectedCommitId) ?? COMMIT_HISTORY[0],
    [selectedCommitId]
  );

  function handleSelectCell(change: CellChange, sheet: string) {
    setSelectedCell((prev) =>
      prev && prev.sheet === sheet && prev.change.coord === change.coord
        ? null
        : { change, sheet }
    );
  }

  // Jump to a cell by its (possibly cross-sheet) ref — used to click through the
  // dependency chain. No-op if the ref isn't a changed cell in this diff.
  function handleNavigateToRef(ref: string) {
    const change = changeByRef.get(canonRef(ref));
    if (!change) return;
    const { sheet } = splitRef(ref);
    setSelectedSheet(sheet);
    setSelectedCell({ change, sheet });
  }

  // A ref is navigable only if it's a changed cell we can open.
  const isNavigable = (ref: string) => changeByRef.has(canonRef(ref));

  return (
    <div className="app">
      <TopBar />
      <div className="body">
        <VersionRail
          commits={COMMIT_HISTORY}
          selectedId={selectedCommitId}
          onSelect={setSelectedCommitId}
          diff={diff}
          onSelectCell={handleSelectCell}
        />
        {diff ? (
          <DiffColumn
            diff={diff}
            commit={commit}
            mode={mode}
            onModeChange={setMode}
            selectedSheet={selectedSheet}
            onSelectSheet={setSelectedSheet}
            changeByRef={changeByRef}
            cascadeByOrigin={cascadeByOrigin}
            anomaliesByRef={anomaliesByRef}
            selectedCell={selectedCell}
            onSelectCell={handleSelectCell}
            onNavigate={handleNavigateToRef}
            isNavigable={isNavigable}
            rippled={rippled}
            onHover={setHover}
          />
        ) : (
          <div className="empty-diff">
            <div className="empty-diff-title">
              {commit.base ? "Initial version" : "No diff"}
            </div>
            <div className="empty-diff-sub">
              {commit.base
                ? `${commit.message} — this is the first commit, so there's nothing before it to compare against.`
                : "No changes to show for this commit."}
            </div>
          </div>
        )}
      </div>
      {hover && <HoverCard hover={hover} cascadeByOrigin={cascadeByOrigin} />}
    </div>
  );
}
