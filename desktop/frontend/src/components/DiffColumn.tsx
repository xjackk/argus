import { useMemo } from "react";
import type { DiffResult, CellChange, Cascade, Anomaly } from "../data/types";
import type { CommitRow } from "../data/history";
import { formatValue, formatDelta } from "../data/format";
import { qualify } from "../data/refs";
import { isAuthored } from "../data/classify";
import { VirtualGrid, ViewMode } from "./VirtualGrid";
import { CellDetail } from "./CellDetail";
import type { HoverState } from "./HoverCard";

// Sheets shown muted for structural context when they carry no changes.
const CONTEXT_SHEETS = ["P&L", "Debt"];

interface Props {
  diff: DiffResult;
  commit: CommitRow;
  mode: ViewMode;
  onModeChange: (m: ViewMode) => void;
  selectedSheet: string;
  onSelectSheet: (name: string) => void;
  changeByRef: Map<string, CellChange>;
  cascadeByOrigin: Map<string, Cascade>;
  anomaliesByRef: Map<string, Anomaly>;
  selectedCell: { change: CellChange; sheet: string } | null;
  onSelectCell: (change: CellChange, sheet: string) => void;
  rippled: Set<string>;
  onHover: (h: HoverState | null) => void;
}

function sheetCount(changes: CellChange[], mode: ViewMode): number {
  return mode === "authored" ? changes.filter(isAuthored).length : changes.length;
}

export function DiffColumn(props: Props) {
  const {
    diff,
    commit,
    mode,
    onModeChange,
    selectedSheet,
    onSelectSheet,
    changeByRef,
    cascadeByOrigin,
    anomaliesByRef,
    selectedCell,
    onSelectCell,
    rippled,
    onHover,
  } = props;

  const sheet = diff.sheets.find((s) => s.name === selectedSheet) ?? diff.sheets[0];
  const narrative = diff.summary.narrative;

  // Up to three headline metrics from the cascade top-movers (highest |Δ|).
  const movers = useMemo(
    () =>
      diff.cascades
        .flatMap((c) => c.topMovers)
        .sort((a, b) => Math.abs(b.magnitude ?? 0) - Math.abs(a.magnitude ?? 0))
        .slice(0, 3),
    [diff]
  );

  const visibleCount = sheetCount(sheet.changes, mode);
  const anomaly = diff.anomalies[0];

  return (
    <div className="diffcol">
      {/* Commit header */}
      <div className="chead">
        <div className="ctitle">{commit.message}</div>
        {commit.description && <div className="cdesc">{commit.description}</div>}
        <div className="cauth">
          <div className="av" />
          <span className="who">{commit.author}</span>
          <span className="sha">{commit.id}</span>
          <div className="stat">
            <span className="tag-a">{diff.summary.authoredCount} authored</span>
            <span className="tag-c">{diff.summary.computedCount} computed</span>
          </div>
        </div>
      </div>

      {/* Narrative banner — rendered only when present (may be null). */}
      {narrative && (
        <div className="narrative">
          <div className="n-text">{narrative}</div>
          <div className="n-sub">
            {diff.summary.authoredCount === 1
              ? "One person changed one input."
              : `${diff.summary.authoredCount} inputs changed.`}{" "}
            {diff.summary.computedCount} numbers moved as a result.
          </div>
        </div>
      )}

      {/* Metric cards */}
      {movers.length > 0 && (
        <div className="metrics">
          {movers.map((m) => {
            const ch = changeByRef.get(m.ref);
            const fmt = ch?.displayFormat;
            const up = (m.magnitude ?? 0) > 0;
            return (
              <div className="metric" key={m.ref}>
                <div className="m-l">{m.label ?? m.ref}</div>
                <div className="m-v">
                  {formatValue(m.newValue, fmt)}
                  <span className={"m-d" + (up ? " up" : "")}>
                    {formatDelta(m.oldValue, m.newValue, fmt)}
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Anomaly banner — rendered only when anomalies exist. */}
      {anomaly && (
        <div className="anomaly">
          <div className="a-hd">
            <span>⚠</span>
            <span>{titleFor(anomaly)}</span>
          </div>
          <div className="a-msg">
            <span className="sha">{anomaly.ref}</span>
            {anomaly.label ? ` (${anomaly.label}) ` : " "}
            {anomaly.message}
          </div>
        </div>
      )}

      {/* Split: sheet list + grid */}
      <div className="split">
        <div className="sheets">
          <div className="s-head">{diff.sheets.length} changed sheets</div>
          {diff.sheets.map((s) => {
            const cnt = sheetCount(s.changes, mode);
            const onlyComputed =
              s.changes.length > 0 &&
              s.changes.every((c) => c.classification === "computed");
            return (
              <div
                key={s.name}
                className={"srow" + (s.name === selectedSheet ? " on" : "")}
                onClick={() => onSelectSheet(s.name)}
              >
                <span>{s.name}</span>
                <span className={"s-cnt" + (onlyComputed ? " c" : "")}>
                  {cnt}
                </span>
              </div>
            );
          })}
          {/* Unchanged sheets, muted (structural context). */}
          {CONTEXT_SHEETS.filter(
            (n) => !diff.sheets.some((s) => s.name === n)
          ).map((n) => (
              <div key={n} className="srow off">
                <span>{n}</span>
              </div>
            ))}
          {narrative && (
            <div className="aibox">
              <div className="h">AI summary</div>
              <div className="t">{narrative}</div>
            </div>
          )}
        </div>

        <div className="gridpane">
          <div className="ghead">
            <span className="g-name">{sheet.name}</span>
            <span className="g-sub">
              {visibleCount} {mode === "authored" ? "authored" : "changed"}{" "}
              {visibleCount === 1 ? "cell" : "cells"}
            </span>
            <div
              className="toggle"
              onClick={() =>
                onModeChange(mode === "authored" ? "cascade" : "authored")
              }
              title="Toggle authored-only vs. full cascade"
            >
              <span className={mode === "authored" ? "on" : ""}>
                Authored only
              </span>
              <span className={mode === "cascade" ? "on" : ""}>
                Show cascade
              </span>
            </div>
          </div>

          <div className="legend">
            <span className="key">
              <span className="sw" style={{ background: "var(--green)" }} />{" "}
              authored — a human typed it
            </span>
            <span className="key">
              <span className="sw" style={{ background: "var(--purple)" }} />{" "}
              computed — a formula recalculated
            </span>
            <span className="key">
              <span className="fmark">ƒ</span> formula cell
            </span>
          </div>

          <VirtualGrid
            sheet={sheet}
            mode={mode}
            rippled={rippled}
            anomaliesByRef={anomaliesByRef}
            selectedRef={
              selectedCell
                ? qualify(selectedCell.sheet, selectedCell.change.coord)
                : null
            }
            onHover={onHover}
            onSelectCell={(c) => onSelectCell(c, sheet.name)}
          />

          {selectedCell && (
            <CellDetail
              change={selectedCell.change}
              sheet={selectedCell.sheet}
              cascadeByOrigin={cascadeByOrigin}
              onClose={() => onSelectCell(selectedCell.change, selectedCell.sheet)}
            />
          )}
        </div>
      </div>
    </div>
  );
}

function titleFor(a: Anomaly): string {
  switch (a.type) {
    case "formula_replaced_by_constant":
      return "A formula was replaced with a typed number";
    case "large_magnitude_change":
      return "Unusually large change";
    case "formula_inconsistent_in_row":
      return "Formula inconsistent with its row";
    default:
      return "Anomaly detected";
  }
}
