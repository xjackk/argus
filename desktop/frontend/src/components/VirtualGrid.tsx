import { useRef, useMemo } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import type { CellChange, SheetDiff } from "../data/types";
import { formatValue } from "../data/format";
import { colLetter } from "../data/refs";
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
  selectedRef: string | null;
  onHover: (h: HoverState | null) => void;
  onSelectCell: (c: CellChange) => void;
}

export function VirtualGrid({
  sheet,
  mode,
  rippled,
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
    // PE model has hundreds of rows and this same path handles it.
    const rc = Math.max(46, maxRow + 12);
    const colCount = Math.max(6, maxCol + 2);
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
                  return (
                    <DiffCell
                      key={c}
                      change={ch}
                      sheet={sheet.name}
                      mode={mode}
                      rippled={rippled.has(`${sheet.name}!${ch.coord}`)}
                      selected={selectedRef === `${sheet.name}!${ch.coord}`}
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
  selected: boolean;
  onHover: (h: HoverState | null) => void;
  onSelectCell: (c: CellChange) => void;
}

function DiffCell({
  change,
  sheet,
  mode,
  rippled,
  selected,
  onHover,
  onSelectCell,
}: DiffCellProps) {
  const authored = change.classification === "authored";
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
}
