import { memo, useRef, useMemo, useEffect } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import type { CellChange, SheetDiff, Anomaly } from "../data/types";
import { formatValue } from "../data/format";
import { colLetter, qualify } from "../data/refs";
import { isAuthored } from "../data/classify";
import type { HoverState } from "./HoverCard";

export type ViewMode = "authored" | "cascade";

const ROWNUM_W = 44;
const LABEL_W = 200;
const CELL_W = 112;
const ROW_H = 30;

interface Props {
  sheet: SheetDiff;
  mode: ViewMode;
  rippled: Set<string>; // qualified refs highlighted from a selected authored cell
  anomaliesByRef: Map<string, Anomaly>; // qualified ref -> anomaly, for ⚠ badges
  selectedRef: string | null;
  onHover: (h: HoverState | null) => void;
  onSelectCell: (c: CellChange) => void;
}

export function VirtualGrid({
  sheet,
  mode,
  rippled,
  anomaliesByRef,
  selectedRef,
  onHover,
  onSelectCell,
}: Props) {
  const scrollRef = useRef<HTMLDivElement>(null);

  const { changesByRC, rowLabels, rowCount, cols, gridWidth } = useMemo(() => {
    const byRC = new Map<string, CellChange>();
    const labels = new Map<number, string>();
    let maxRow = 0;
    let maxCol = 2;
    for (const ch of sheet.changes) {
      byRC.set(`${ch.row},${ch.col}`, ch);
      if (ch.label) labels.set(ch.row, ch.label);
      maxRow = Math.max(maxRow, ch.row);
      maxCol = Math.max(maxCol, ch.col);
    }
    // Give the grid real scroll room so virtualization is meaningful; a real
    // PE model has hundreds of rows and this same path handles it. Render enough
    // columns to fill a wide viewport so the sheet reads as a real spreadsheet
    // (empty trailing columns = normal Excel), not dead space beside the grid.
    const rc = Math.max(46, maxRow + 12);
    const colCount = Math.max(16, maxCol + 3);
    const colList = Array.from({ length: colCount }, (_, i) => i + 1);
    const width = ROWNUM_W + LABEL_W + (colCount - 1) * CELL_W;
    return {
      changesByRC: byRC,
      rowLabels: labels,
      rowCount: rc,
      cols: colList,
      gridWidth: width,
    };
  }, [sheet]);

  const rowVirtualizer = useVirtualizer({
    count: rowCount,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_H,
    overscan: 10,
  });

  // Scroll the selected cell into view (row + column) when it changes — so
  // clicking a change in the rail actually TAKES you to the cell, not just to
  // the right sheet. No-op when the selected cell isn't on this sheet.
  useEffect(() => {
    const el = scrollRef.current;
    if (!selectedRef || !el) return;
    const sel = sheet.changes.find(
      (c) => qualify(sheet.name, c.coord) === selectedRef
    );
    if (!sel) return;
    rowVirtualizer.scrollToIndex(sel.row - 1, { align: "center" });
    const colX =
      sel.col <= 1 ? ROWNUM_W : ROWNUM_W + LABEL_W + (sel.col - 2) * CELL_W;
    el.scrollTo({
      left: Math.max(0, colX - el.clientWidth / 2 + CELL_W / 2),
      behavior: "smooth",
    });
    // rowVirtualizer identity changes each render; depend on the actual inputs.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedRef, sheet]);

  return (
    <div className="grid-scroll" ref={scrollRef}>
      <div style={{ width: gridWidth }}>
        {/* Column header (A, B, C, …) */}
        <div className="grid-colhead" style={{ width: gridWidth }}>
          <div
            className="gcell colhead-cell rownum"
            style={{ width: ROWNUM_W }}
          />
          <div className="gcell colhead-cell" style={{ width: LABEL_W }}>
            A
          </div>
          {cols.slice(1).map((c) => (
            <div
              key={c}
              className="gcell colhead-cell"
              style={{ width: CELL_W }}
            >
              {colLetter(c)}
            </div>
          ))}
        </div>

        {/* Virtualized body */}
        <div
          style={{
            height: rowVirtualizer.getTotalSize(),
            position: "relative",
            width: gridWidth,
          }}
        >
          {rowVirtualizer.getVirtualItems().map((vRow) => {
            const rowNum = vRow.index + 1;
            const label = rowLabels.get(rowNum) ?? "";
            return (
              <div
                key={vRow.key}
                className="grid-row"
                style={{
                  height: vRow.size,
                  transform: `translateY(${vRow.start}px)`,
                }}
              >
                <div className="gcell rownum" style={{ width: ROWNUM_W }}>
                  {rowNum}
                </div>
                <div className="gcell label" style={{ width: LABEL_W }}>
                  {label}
                </div>
                {cols.slice(1).map((c) => {
                  const ch = changesByRC.get(`${rowNum},${c}`);
                  if (!ch) {
                    return (
                      <div
                        key={c}
                        className="gcell empty"
                        style={{ width: CELL_W }}
                      >
                        ·
                      </div>
                    );
                  }
                  const ref = qualify(sheet.name, ch.coord);
                  return (
                    <DiffCell
                      key={c}
                      change={ch}
                      sheet={sheet.name}
                      mode={mode}
                      rippled={rippled.has(ref)}
                      anomaly={anomaliesByRef.get(ref)}
                      selected={selectedRef === ref}
                      onHover={onHover}
                      onSelectCell={onSelectCell}
                    />
                  );
                })}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

interface DiffCellProps {
  change: CellChange;
  sheet: string;
  mode: ViewMode;
  rippled: boolean;
  anomaly?: Anomaly;
  selected: boolean;
  onHover: (h: HoverState | null) => void;
  onSelectCell: (c: CellChange) => void;
}

const DiffCell = memo(function DiffCell({
  change,
  sheet,
  mode,
  rippled,
  anomaly,
  selected,
  onHover,
  onSelectCell,
}: DiffCellProps) {
  const authored = isAuthored(change);
  const isFormula = change.newFormula != null;

  // Authored-only view: filter to classification === "authored". Computed cells
  // are still present in the sheet, but render as their plain current value —
  // so toggling to Cascade visibly lights them up (the money shot).
  const showDiff = mode === "cascade" || authored;

  const cls = [
    "gcell",
    "diffcell",
    showDiff ? change.classification : "plain",
    rippled ? "rippled" : "",
    anomaly ? "anomalous" : "",
    selected ? "sel-ring" : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div
      className={cls}
      style={{ width: CELL_W }}
      onMouseEnter={(e) =>
        onHover({ change, sheet, x: e.clientX, y: e.clientY })
      }
      onMouseMove={(e) => onHover({ change, sheet, x: e.clientX, y: e.clientY })}
      onMouseLeave={() => onHover(null)}
      onClick={() => onSelectCell(change)}
    >
      {anomaly && (
        <span className="cell-warn" title={anomaly.message}>
          ⚠
        </span>
      )}
      {isFormula && <span className="fmark-inline">ƒ</span>}
      {showDiff ? (
        <>
          <span className={"dc-old " + change.classification}>
            {formatValue(change.oldValue, change.displayFormat)}
          </span>
          <span className={"dc-new " + change.classification}>
            {formatValue(change.newValue, change.displayFormat)}
          </span>
        </>
      ) : (
        <span>{formatValue(change.newValue, change.displayFormat)}</span>
      )}
    </div>
  );
});
