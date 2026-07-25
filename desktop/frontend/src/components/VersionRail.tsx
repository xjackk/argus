import { useState } from "react";
import type { CommitRow } from "../data/history";
import type { CellChange, DiffResult } from "../data/types";
import { formatValue } from "../data/format";

interface Props {
  commits: CommitRow[];
  selectedId: string;
  onSelect: (id: string) => void;
  diff: DiffResult | null;
  onSelectCell: (change: CellChange, sheet: string) => void;
}

export function VersionRail({
  commits,
  selectedId,
  onSelect,
  diff,
  onSelectCell,
}: Props) {
  const [tab, setTab] = useState<"changes" | "history">("history");
  const [filter, setFilter] = useState("");

  const rows = commits.filter((c) =>
    (c.message + " " + c.author).toLowerCase().includes(filter.toLowerCase())
  );

  // Flatten the current diff into a per-cell changed list for the Changes tab.
  const changed: { sheet: string; change: CellChange }[] = [];
  if (diff) {
    for (const s of diff.sheets)
      for (const ch of s.changes) changed.push({ sheet: s.name, change: ch });
  }

  return (
    <div className="rail">
      <div className="rail-tabs">
        <div
          className={"rail-tab" + (tab === "changes" ? " on" : "")}
          onClick={() => setTab("changes")}
        >
          Changes{diff ? ` (${changed.length})` : ""}
        </div>
        <div
          className={"rail-tab" + (tab === "history" ? " on" : "")}
          onClick={() => setTab("history")}
        >
          History
        </div>
      </div>

      {tab === "history" ? (
        <>
          <div className="rail-filter">
            <input
              placeholder="Filter versions…"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
            />
          </div>
          <div className="rail-list">
            {rows.map((c) => (
              <div
                key={c.id}
                className={"crow" + (c.id === selectedId ? " sel" : "")}
                onClick={() => onSelect(c.id)}
              >
                <div className="csum">{c.message}</div>
                <div className="cmeta">
                  <div className="av" />
                  <span className="cby">
                    {c.author} · {c.when}
                  </span>
                </div>
                <div className="cnt">
                  {c.base ? (
                    <span className="tag-m">base version</span>
                  ) : c.structural ? (
                    <span className="tag-m">structural — row inserted</span>
                  ) : (
                    <>
                      <span className="tag-a">{c.authoredCount} authored</span>
                      <span className="tag-m">·</span>
                      <span className="tag-c">{c.computedCount} computed</span>
                    </>
                  )}
                  {c.signedOff && (
                    <span
                      className="signoff"
                      title={`Signed off by ${c.signedOff}`}
                    >
                      ✓ signed
                    </span>
                  )}
                  {c.anomaly && (
                    <span className="anom-badge" title="Anomaly flagged">
                      ⚠
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </>
      ) : (
        // Changes tab: the selected commit's changed cells, grouped nowhere —
        // a flat GitHub-style list. Click a row → open that cell's detail.
        <div className="rail-list">
          {changed.length === 0 && (
            <div className="rail-empty">No changes in this version.</div>
          )}
          {changed.map(({ sheet, change }) => (
            <div
              key={sheet + "!" + change.coord}
              className="chrow"
              onClick={() => onSelectCell(change, sheet)}
            >
              <span
                className={
                  "ch-dot " +
                  (change.classification === "authored" ? "a" : "c")
                }
              />
              <div className="ch-main">
                <div className="ch-ref">
                  <span className="sha">
                    {sheet}!{change.coord}
                  </span>
                  {change.label ? ` ${change.label}` : ""}
                </div>
                <div className="ch-val">
                  {formatValue(change.oldValue, change.displayFormat)} →{" "}
                  <b>{formatValue(change.newValue, change.displayFormat)}</b>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
