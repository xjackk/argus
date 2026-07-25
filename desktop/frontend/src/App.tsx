import { useEffect, useMemo, useRef, useState } from "react";
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
  const [loaded, setLoaded] = useState(false); // first history source resolved
  // Selections persist across refresh (localStorage) so a reload keeps you on the
  // same workbook / commit / sheet instead of snapping back to the first file.
  const [selectedWorkbook, setSelectedWorkbook] = useState(
    () => localStorage.getItem("argus.workbook") ?? "Project Atlas — LBO"
  );
  const [selectedCommitId, setSelectedCommitId] = useState(
    () => localStorage.getItem("argus.commit") ?? DEFAULT_COMMIT
  );
  const [selectedSheet, setSelectedSheet] = useState<string>(
    () => localStorage.getItem("argus.sheet") ?? "Returns"
  );
  const [mode, setMode] = useState<ViewMode>(() =>
    localStorage.getItem("argus.mode") === "authored" ? "authored" : "cascade"
  );
  const [hover, setHover] = useState<HoverState | null>(null);
  const [selectedCell, setSelectedCell] = useState<{
    change: CellChange;
    sheet: string;
  } | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const [narrating, setNarrating] = useState(false); // live AI summary in flight
  const lastTopId = useRef<string | null>(null); // newest commit id last seen live

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
      if (!active) return;
      if (h) {
        // "Poof, it changed": when a new commit lands after the first live load,
        // flash a toast — the real-time, always-running signal.
        const topId = h[0]?.id ?? null;
        if (lastTopId.current !== null && topId !== lastTopId.current) {
          const c = h[0];
          setToast(`New change tracked — ${c.author} saved ${c.file}`);
          window.setTimeout(() => setToast(null), 4000);
        }
        lastTopId.current = topId;
        setCommits(h);
        setLive(true);
      }
      // Mark the source resolved (live or bundled fallback) so the validity
      // effects below don't reset a persisted selection before live data lands.
      setLoaded(true);
    }
    poll();
    const t = window.setInterval(poll, 3000);
    return () => {
      active = false;
      window.clearInterval(t);
    };
  }, []);

  // Keep the selected workbook valid as the tracked files change. Gated on
  // `loaded` so a persisted (live) workbook isn't reset against the bundled
  // chain in the moment before the live history arrives.
  useEffect(() => {
    if (!loaded) return;
    if (workbooks.length && !workbooks.includes(selectedWorkbook)) {
      setSelectedWorkbook(workbooks[0]);
    }
  }, [workbooks, selectedWorkbook, loaded]);

  // Keep the selected commit valid within the selected workbook (default to its
  // newest non-base commit). Same `loaded` gate as above.
  useEffect(() => {
    if (!loaded) return;
    if (shownCommits.length && !shownCommits.some((c) => c.id === selectedCommitId)) {
      setSelectedCommitId(
        shownCommits.find((c) => !c.base)?.id ?? shownCommits[0].id
      );
    }
  }, [shownCommits, selectedCommitId, loaded]);

  // Persist selections so a refresh restores the same view.
  useEffect(() => {
    localStorage.setItem("argus.workbook", selectedWorkbook);
  }, [selectedWorkbook]);
  useEffect(() => {
    localStorage.setItem("argus.commit", selectedCommitId);
  }, [selectedCommitId]);
  useEffect(() => {
    localStorage.setItem("argus.sheet", selectedSheet);
  }, [selectedSheet]);
  useEffect(() => {
    localStorage.setItem("argus.mode", mode);
  }, [mode]);

  // ⌘/Ctrl-R reloads and re-reads the store — matches the browser, and gives the
  // desktop app the same "pull fresh data now" gesture (it also auto-polls /store
  // every 3s, so this is a convenience, not a requirement).
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && (e.key === "r" || e.key === "R")) {
        e.preventDefault();
        window.location.reload();
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

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

  // ── Live AI summary: a fresh capture's diff is written with narrative=null,
  // and the daemon fills it ~2-3s later (async). While it's missing, pulse a
  // loading skeleton and re-fetch the diff until the summary lands — capped, so
  // a failed narration just stops pulsing. Live mode only; bundled diffs ship
  // with their narrative already baked in. ──
  useEffect(() => {
    if (!live || !diff || diff.summary.narrative || !diff.sheets.length) {
      setNarrating(false);
      return;
    }
    setNarrating(true);
    let active = true;
    let tries = 0;
    const t = window.setInterval(async () => {
      tries += 1;
      const d = await fetchLiveDiff(selectedCommitId);
      if (!active) return;
      if (d && d.summary.narrative) {
        setDiff(d);
        setNarrating(false);
        window.clearInterval(t);
      } else if (tries >= 30) {
        // ~60s cap — must outlast the daemon's 45s narration timeout so a slow
        // (big-file) summary that lands late isn't missed by the client.
        setNarrating(false);
        window.clearInterval(t);
      }
    }, 2000);
    return () => {
      active = false;
      window.clearInterval(t);
    };
  }, [diff, live, selectedCommitId]);

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
    const isSame =
      selectedCell &&
      selectedCell.sheet === sheet &&
      selectedCell.change.coord === change.coord;
    if (isSame) {
      setSelectedCell(null); // clicking the open cell again closes it
      return;
    }
    // Jump the center grid to the cell's sheet, so clicking a change in the rail
    // (which may live on a different sheet than the one on screen) takes you
    // there — the tab switches and the cell highlights, not just the inspector.
    setSelectedSheet(sheet);
    setSelectedCell({ change, sheet });
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
          mode={mode}
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
            narrating={narrating}
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
      {toast && <div className="toast live-toast">{toast}</div>}
    </div>
  );
}
