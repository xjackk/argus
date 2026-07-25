import type { CellChange, Cascade } from "../data/types";
import { formatDelta } from "../data/format";
import { revisionsFor } from "../data/cellHistory";
import { qualify } from "../data/refs";
import { isAuthored, classDot } from "../data/classify";
import { ValueDelta } from "./ValueDelta";

interface Props {
  change: CellChange;
  sheet: string;
  cascadeByOrigin: Map<string, Cascade>;
  onClose: () => void;
}

// State 2 — "git log for a cell": formula, before/after, dependency chain,
// revision timeline, and the classification-derived reassurance line.
export function CellDetail({ change, sheet, cascadeByOrigin, onClose }: Props) {
  const qualified = qualify(sheet, change.coord);
  const authored = isAuthored(change);
  const origin = change.causedBy[0];
  const cascade = cascadeByOrigin.get(qualified);
  const revisions = revisionsFor(qualified); // oldest→newest
  const precedents = change.dependsOn.filter((d) => d !== origin);
  const up = (change.magnitude ?? 0) > 0; // delta color follows direction

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
          <ValueDelta
            old={change.oldValue}
            next={change.newValue}
            fmt={change.displayFormat}
          />{" "}
          <span className={"d-delta" + (up ? " up" : "")}>
            {formatDelta(change.oldValue, change.newValue, change.displayFormat)}
          </span>
        </span>
      </div>
      <div className="dr">
        <span className="k">Type</span>
        <span className={"tag-" + classDot(change.classification)}>
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
          {precedents.map((d) => (
            <div key={d} className="chain-step">
              <span className="tag-c">{d}</span>
            </div>
          ))}
          {precedents.length > 0 && (
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
                <span className={"tl-dot " + classDot(r.classification)} />
                <div className="tl-body">
                  <div className="tl-val">
                    <ValueDelta
                      old={r.oldValue}
                      next={r.newValue}
                      fmt={change.displayFormat}
                    />
                  </div>
                  <div className="tl-meta">
                    {r.author} ·{" "}
                    <span className={"tag-" + classDot(r.classification)}>
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
