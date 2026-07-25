import { useMemo, useState } from "react";
import type { DiffResult, CellChange, Cascade, Anomaly } from "../data/types";
import type { CommitRow } from "../data/history";
import { formatValue, formatDelta } from "../data/format";
import { qualify, canonRef } from "../data/refs";
import { isAuthored } from "../data/classify";
import { VirtualGrid, ViewMode } from "./VirtualGrid";
import { CellDetail } from "./CellDetail";
import { SheetTabs } from "./SheetTabs";
import { OpenInExcel } from "./OpenInExcel";
import type { HoverState } from "./HoverCard";

// Unchanged sheets used to be a hardcoded ["P&L", "Debt"] — the demo workbook's
// tab names, which showed phantom rows for any other file. They now come from
// DiffResult.allSheets (Excel's real tab order); this falls back to an empty
// list for diffs generated before the engine emitted that field.
function unchangedSheets(diff: DiffResult): string[] {
  if (!diff.allSheets?.length) return [];
  return diff.allSheets.filter((n) => !diff.sheets.some((s) => s.name === n));
}

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
  onNavigate: (ref: string) => void;
  isNavigable: (ref: string) => boolean;
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
    onNavigate,
    isNavigable,
    rippled,
    onHover,
  } = props;

  const [summaryCollapsed, setSummaryCollapsed] = useState(false);
  // null when the selected sheet has no changes in this commit — reachable now
  // that the tab strip lets you open any sheet, not just changed ones. Falling
  // back to sheets[0] here would silently render a DIFFERENT sheet than the one
  // the tab says is active.
  const sheet = diff.sheets.find((s) => s.name === selectedSheet) ?? null;
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

  // Metric cards only make sense when the top movers have human labels (a
  // model-like sheet with named outputs). For plain/unlabeled sheets we skip
  // them and let the AI summary be the headline — keeps the app workbook-
  // agnostic instead of implying every sheet has KPIs.
  const showMetrics = movers.length > 0 && movers.every((m) => !!m.label);
  const hasSummary = !!narrative || showMetrics;
  const visibleCount = sheet ? sheetCount(sheet.changes, mode) : 0;
  const anomaly = diff.anomalies[0];

  // Per-sheet count for the tab strip: a number for sheets this commit touched,
  // null for untouched ones (which the strip renders muted, without a badge).
  const countFor = (name: string) => {
    const s = diff.sheets.find((x) => x.name === name);
    return s ? sheetCount(s.changes, mode) : null;
  };

  return (
    <div className="diffcol">
      {/* Commit header */}
      <div className="chead">
        <div className="chead-top">
          <div className="ctitle">{commit.message}</div>
          {hasSummary && (
            <button
              className="collapse-btn"
              onClick={() => setSummaryCollapsed((v) => !v)}
              title={summaryCollapsed ? "Show summary" : "Hide summary"}
            >
              {summaryCollapsed ? "▾ Summary" : "▴ Hide summary"}
            </button>
          )}
        </div>
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

      {/* Narrative banner — rendered only when present and not collapsed. */}
      {!summaryCollapsed && narrative && (
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

      {/* Metric cards — only for labeled model-like sheets (see showMetrics). */}
      {!summaryCollapsed && showMetrics && (
        <div className="metrics">
          {movers.map((m) => {
            // m.ref may be quoted ('P&L'!G10); changeByRef is keyed unquoted.
            const ch = changeByRef.get(canonRef(m.ref));
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
          {/* Unchanged sheets, muted — now clickable, so a reviewer can go look
              at a sheet this commit didn't touch. */}
          {unchangedSheets(diff).map((n) => (
            <div
              key={n}
              className={"srow off" + (n === selectedSheet ? " on" : "")}
              onClick={() => onSelectSheet(n)}
            >
              <span>{n}</span>
            </div>
          ))}
          {/* When the top banner is collapsed, keep the summary reachable here. */}
          {summaryCollapsed && narrative && (
            <div className="aibox">
              <div className="h">AI summary</div>
              <div className="t">{narrative}</div>
            </div>
          )}
        </div>

        <div className="gridpane">
          <div className="ghead">
            {/* Breadcrumb: which workbook, then which sheet. Answers "where am
                I?" right next to the data, instead of only in the top bar. */}
            <span className="g-crumb">{commit.file} ›</span>
            <span className="g-name">{selectedSheet}</span>
            <span className="g-sub">
              {sheet
                ? `${visibleCount} ${mode === "authored" ? "authored" : "changed"} ${
                    visibleCount === 1 ? "cell" : "cells"
                  }`
                : "no changes in this sheet"}
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

          {/* The legend explains the grid's colours, so it only earns its space
              when there is a grid. */}
          {sheet && (
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
          )}

          {sheet ? (
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
          ) : (
            // Reachable from the tab strip: a sheet this commit didn't touch.
            // Argus stores diffs, not whole sheets, so there is nothing to
            // render here — say so plainly rather than showing an empty grid.
            <div className="nosheet">
              <div className="nosheet-title">No changes in {selectedSheet}</div>
              <div className="nosheet-sub">
                This commit didn’t touch this sheet. Argus stores what changed,
                so there’s nothing to draw here — open the workbook itself to
                browse it in full.
              </div>
              <OpenInExcel
                path={diff.to.path}
                sheet={selectedSheet}
                label={`${commit.file} @ ${commit.id}`}
              />
            </div>
          )}

          {/* Excel's tab strip, along the bottom edge where users expect it. */}
          <SheetTabs
            diff={diff}
            selectedSheet={selectedSheet}
            onSelectSheet={onSelectSheet}
            countFor={countFor}
          />
        </div>

        {/* Right inspector — the clicked cell's detail (UI-SPEC State 2). */}
        {selectedCell && (
          <div className="inspector">
            <CellDetail
              change={selectedCell.change}
              sheet={selectedCell.sheet}
              cascadeByOrigin={cascadeByOrigin}
              changeByRef={changeByRef}
              onNavigate={onNavigate}
              isNavigable={isNavigable}
              onClose={() => onSelectCell(selectedCell.change, selectedCell.sheet)}
            />
          </div>
        )}
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
