import { useEffect, useMemo, useState } from "react";
import { diffForCommit } from "./data/diffs";
import { COMMIT_HISTORY, type CommitRow } from "./data/history";
import { fetchLiveHistory, fetchLiveDiff } from "./data/store";
import { qualify, canonRef, splitRef } from "./data/refs";
import { isAuthored } from "./data/classify";
import type { DiffResult, CellChange, Cascade, Anomaly } from "./data/types";
import { TopBar } from "./components/TopBar";
import { VersionRail } from "./components/VersionRail";
import { DiffColumn } from "./components/DiffColumn";
import { HoverCard, HoverState } from "./components/HoverCard";
import type { ViewMode } from "./components/VirtualGrid";

// The commit shown on open when using the bundled chain — the signed-off
// exit-multiple cascade (demo centerpiece). Live mode picks the newest commit.
const DEFAULT_COMMIT =
  COMMIT_HISTORY.find((c) => c.id === "c06")?.id ?? COMMIT_HISTORY[0].id;

export default function App() {
  const [diff, setDiff] = useState<DiffResult | null>(null);
  const [commits, setCommits] = useState<CommitRow[]>(COMMIT_HISTORY);
  const [live, setLive] = useState(false); // true once a daemon store is found
  const [selectedWorkbook, setSelectedWorkbook] = useState("Project Atlas — LBO");
  const [selectedCommitId, setSelectedCommitId] = useState(DEFAULT_COMMIT);
  const [selectedSheet, setSelectedSheet] = useState<string>("Returns");
  const [mode, setMode] = useState<ViewMode>("cascade"); // State 1: cascade active
  const [hover, setHover] = useState<HoverState | null>(null);
  const [selectedCell, setSelectedCell] = useState<{
    change: CellChange;
    sheet: string;
  } | null>(null);

  // The distinct workbooks (files) in the timeline, and the commits for the one
  // currently selected in the workbook dropdown — the rail shows a single file's
  // history, like GitHub Desktop shows one repo.
  const workbooks = useMemo(
    () => [...new Set(commits.map((c) => c.file))],
    [commits]
  );
  const shownCommits = useMemo(
    () => commits.filter((c) => c.file === selectedWorkbook),
    [commits, selectedWorkbook]
  );

  // ── Poll the live daemon store. If a daemon is running, its history replaces
  // the bundled chain and new saves appear automatically; if not, we stay on the
  // bundled c01→c07 chain. Same UI either way. ──
  useEffect(() => {
    let active = true;
    async function poll() {
      const h = await fetchLiveHistory();
      if (!active || !h) return;
      setCommits(h);
      setLive(true);
    }
    poll();
    const t = window.setInterval(poll, 3000);
    return () => {
      active = false;
      window.clearInterval(t);
    };
  }, []);

  // Keep the selected workbook valid as the tracked files change.
  useEffect(() => {
    if (workbooks.length && !workbooks.includes(selectedWorkbook)) {
      setSelectedWorkbook(workbooks[0]);
    }
  }, [workbooks, selectedWorkbook]);

  // Keep the selected commit valid within the selected workbook (default to its
  // newest non-base commit).
  useEffect(() => {
    if (shownCommits.length && !shownCommits.some((c) => c.id === selectedCommitId)) {
      setSelectedCommitId(
        shownCommits.find((c) => !c.base)?.id ?? shownCommits[0].id
      );
    }
  }, [shownCommits, selectedCommitId]);

  // ── Load the selected commit's diff (live store or bundled). Clears any open
  // cell and keeps the sheet selection if it still exists in the new diff. ──
  useEffect(() => {
    let active = true;
    async function load() {
      const d = live
        ? await fetchLiveDiff(selectedCommitId)
        : diffForCommit(selectedCommitId);
      if (!active) return;
      setDiff(d);
      setSelectedCell(null);
      if (d && d.sheets.length) {
        setSelectedSheet((prev) =>
          d.sheets.some((s) => s.name === prev) ? prev : d.sheets[0].name
        );
      }
    }
    load();
    return () => {
      active = false;
    };
  }, [selectedCommitId, live]);

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
    () => commits.find((c) => c.id === selectedCommitId) ?? commits[0],
    [commits, selectedCommitId]
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
      <TopBar
        live={live}
        workbooks={workbooks}
        selected={selectedWorkbook}
        onSelect={setSelectedWorkbook}
      />
      <div className="body">
        <VersionRail
          commits={shownCommits}
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
