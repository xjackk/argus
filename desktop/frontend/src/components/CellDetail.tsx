import type { CellChange, Cascade } from "../data/types";
import { formatValue, formatDelta } from "../data/format";
import { revisionsFor } from "../data/cellHistory";

interface Props {
  change: CellChange;
  sheet: string;
  cascadeByOrigin: Map<string, Cascade>;
  onClose: () => void;
}

// State 2 — "git log for a cell": formula, before/after, dependency chain,
// revision timeline, and the classification-derived reassurance line.
export function CellDetail({ change, sheet, cascadeByOrigin, onClose }: Props) {
  const qualified = `${sheet}!${change.coord}`;
  const authored = change.classification === "authored";
  const origin = change.causedBy[0];
  const cascade = cascadeByOrigin.get(qualified);
  const revisions = revisionsFor(qualified); // oldest→newest

  return (
    <div className="detail">
      <div className="d-head">
        <span className="d-title">{change.label ?? change.coord}</span>
        <span className="sha">{qualified}</span>
        <span className="d-close" onClick={onClose}>
          ✕
        </span>
      </div>

      {change.newFormula && <div className="d-formula">{change.newFormula}</div>}

      <div className="dr">
        <span className="k">Change</span>
        <span>
          {formatValue(change.oldValue, change.displayFormat)} →{" "}
          <b>{formatValue(change.newValue, change.displayFormat)}</b>{" "}
          <span style={{ color: "var(--red)" }}>
            {formatDelta(change.oldValue, change.newValue, change.displayFormat)}
          </span>
        </span>
      </div>
      <div className="dr">
        <span className="k">Type</span>
        <span className={authored ? "tag-a" : "tag-c"}>
          {change.classification}
        </span>
      </div>
      {authored && cascade && (
        <div className="dr">
          <span className="k">Ripples to</span>
          <span>{cascade.affectedCount} cells</span>
        </div>
      )}
      {origin && (
        <div className="dr">
          <span className="k">Caused by</span>
          <span className="tag-a">{origin}</span>
        </div>
      )}

      {/* Dependency chain: origin → this cell, plus direct precedents. */}
      {origin && (
        <div className="chain">
          <div className="h">Dependency chain</div>
          <div className="chain-step">
            <span className="tag-a">{origin}</span>
          </div>
          <div className="chain-step chain-arrow">↓</div>
          {change.dependsOn
            .filter((d) => d !== origin)
            .map((d) => (
              <div key={d} className="chain-step">
                <span className="tag-c">{d}</span>
              </div>
            ))}
          {change.dependsOn.filter((d) => d !== origin).length > 0 && (
            <div className="chain-step chain-arrow">↓</div>
          )}
          <div className="chain-step here">
            {qualified} <span className="tag-m">← you are here</span>
          </div>
        </div>
      )}

      {/* Revision timeline — "git log for a cell". Newest first. */}
      {revisions.length > 0 && (
        <div className="timeline">
          <div className="h">History — {revisions.length} revisions</div>
          {revisions
            .slice()
            .reverse()
            .map((r, i) => (
              <div className="tl-row" key={r.commit + i}>
                <span
                  className={
                    "tl-dot " + (r.classification === "authored" ? "a" : "c")
                  }
                />
                <div className="tl-body">
                  <div className="tl-val">
                    {formatValue(r.oldValue, change.displayFormat)} →{" "}
                    <b>{formatValue(r.newValue, change.displayFormat)}</b>
                  </div>
                  <div className="tl-meta">
                    {r.author} ·{" "}
                    <span className={r.classification === "authored" ? "tag-a" : "tag-c"}>
                      {r.classification}
                    </span>{" "}
                    <span className="sha">{r.commit}</span>
                  </div>
                </div>
              </div>
            ))}
        </div>
      )}

      {!authored && origin && (
        <div className="reassure">
          {change.label ?? change.coord} has never been directly edited — every
          change here came from {origin}.
        </div>
      )}
    </div>
  );
}
