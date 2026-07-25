import type { DiffResult } from "../data/types";
import "./SheetTabs.css";

// Excel's sheet tabs, along the bottom edge of the grid — the one piece of
// chrome every spreadsheet user already knows cold. The middle "changed sheets"
// pane answers "what did this commit touch?"; these answer "where am I, and let
// me go look at Debt anyway." Different questions, so both earn their space.
//
// Sheets come from DiffResult.allSheets (Excel's own tab order, unchanged sheets
// included). Diffs produced before that field existed omit it, so we fall back
// to the changed sheets and the strip degrades instead of vanishing.

interface Props {
  diff: DiffResult;
  selectedSheet: string;
  onSelectSheet: (name: string) => void;
  /** Changed-cell count per sheet, respecting the authored/cascade toggle. */
  countFor: (sheetName: string) => number | null;
}

export function SheetTabs({ diff, selectedSheet, onSelectSheet, countFor }: Props) {
  const names = diff.allSheets?.length
    ? diff.allSheets
    : diff.sheets.map((s) => s.name);

  if (names.length === 0) return null;

  return (
    <div className="sheettabs" role="tablist" aria-label="Workbook sheets">
      {names.map((name) => {
        const count = countFor(name);
        const changed = count !== null;
        return (
          <button
            key={name}
            role="tab"
            aria-selected={name === selectedSheet}
            className={
              "stab" +
              (name === selectedSheet ? " on" : "") +
              (changed ? "" : " unchanged")
            }
            onClick={() => onSelectSheet(name)}
            title={
              changed
                ? `${name} — ${count} changed ${count === 1 ? "cell" : "cells"}`
                : `${name} — no changes in this version`
            }
          >
            <span className="stab-name">{name}</span>
            {changed && count > 0 && <span className="stab-count">{count}</span>}
          </button>
        );
      })}
    </div>
  );
}
