import { useEffect, useMemo, useState } from "react";
import "./styles.css";
import { loadDiff } from "./data/loadDiff";
import { COMMIT_HISTORY } from "./data/history";
import type { DiffResult, CellChange, Cascade, Anomaly } from "./data/types";
import { TopBar } from "./components/TopBar";
import { VersionRail } from "./components/VersionRail";
import { DiffColumn } from "./components/DiffColumn";
import { HoverCard, HoverState } from "./components/HoverCard";
import type { ViewMode } from "./components/VirtualGrid";

export default function App() {
  const [diff, setDiff] = useState<DiffResult | null>(null);
  const [selectedCommitId, setSelectedCommitId] = useState(
    COMMIT_HISTORY.find((c) => c.isCurrentDiff)?.id ?? COMMIT_HISTORY[0].id
  );
  const [selectedSheet, setSelectedSheet] = useState<string>("Returns");
  const [mode, setMode] = useState<ViewMode>("cascade"); // State 1: cascade active
  const [hover, setHover] = useState<HoverState | null>(null);
  const [selectedCell, setSelectedCell] = useState<{
    change: CellChange;
    sheet: string;
  } | null>(null);

  // ── The single data source. Swap loadDiff()'s body for the engine later. ──
  useEffect(() => {
    loadDiff().then(setDiff);
  }, []);

  // Lookup maps derived once per diff.
  const { changeByRef, cascadeByOrigin, anomaliesByRef } = useMemo(() => {
    const byRef = new Map<string, CellChange>();
    const casc = new Map<string, Cascade>();
    const anom = new Map<string, Anomaly>();
    if (diff) {
      for (const s of diff.sheets)
        for (const ch of s.changes) byRef.set(`${s.name}!${ch.coord}`, ch);
      for (const c of diff.cascades) casc.set(c.origin, c);
      for (const a of diff.anomalies) anom.set(a.ref, a);
    }
    return { changeByRef: byRef, cascadeByOrigin: casc, anomaliesByRef: anom };
  }, [diff]);

  // Highlight the cascade blast-radius when an authored cell is selected.
  const rippled = useMemo(() => {
    const set = new Set<string>();
    if (selectedCell && selectedCell.change.classification === "authored") {
      const origin = `${selectedCell.sheet}!${selectedCell.change.coord}`;
      const c = cascadeByOrigin.get(origin);
      if (c) c.affected.forEach((r) => set.add(r));
    }
    return set;
  }, [selectedCell, cascadeByOrigin]);

  const commit =
    COMMIT_HISTORY.find((c) => c.id === selectedCommitId) ?? COMMIT_HISTORY[0];

  function handleSelectCell(change: CellChange, sheet: string) {
    setSelectedCell((prev) =>
      prev && prev.sheet === sheet && prev.change.coord === change.coord
        ? null
        : { change, sheet }
    );
  }

  return (
    <div className="app">
      <TopBar workbook="Project Atlas — LBO" scenario="deal-team/base" />
      <div className="body">
        <VersionRail
          commits={COMMIT_HISTORY}
          selectedId={selectedCommitId}
          onSelect={setSelectedCommitId}
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
            rippled={rippled}
            onHover={setHover}
          />
        ) : (
          <div
            style={{
              flex: 1,
              display: "grid",
              placeItems: "center",
              color: "var(--tx2)",
            }}
          >
            Loading diff…
          </div>
        )}
      </div>
      {hover && <HoverCard hover={hover} cascadeByOrigin={cascadeByOrigin} />}
    </div>
  );
}
