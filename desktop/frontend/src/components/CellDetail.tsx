import type { CellChange, Cascade } from "../data/types";
import { formatDelta } from "../data/format";
import { revisionsFor } from "../data/cellHistory";
import { qualify, canonRef } from "../data/refs";
import { isAuthored, classDot } from "../data/classify";
import { dependencyChain } from "../data/graph";
import { ValueDelta } from "./ValueDelta";

interface Props {
  change: CellChange;
  sheet: string;
  cascadeByOrigin: Map<string, Cascade>;
  changeByRef: Map<string, CellChange>;
  onNavigate: (ref: string) => void;
  isNavigable: (ref: string) => boolean;
  onClose: () => void;
}

// State 2 — "git log for a cell": formula, before/after, dependency chain (with
// the actual formula + value move at each step), revision timeline, and the
// classification-derived reassurance line.
export function CellDetail({
  change,
  sheet,
  cascadeByOrigin,
  changeByRef,
  onNavigate,
  isNavigable,
  onClose,
}: Props) {
  const qualified = qualify(sheet, change.coord);
  const authored = isAuthored(change);
  const origin = change.causedBy[0];
  const cascade = cascadeByOrigin.get(qualified);
  const revisions = revisionsFor(qualified); // oldest→newest
  const up = (change.magnitude ?? 0) > 0; // delta color follows direction

  // The full multi-hop path of changed cells from the authored origin down to
  // this one (origin → … → here). Each node is enriched with its formula +
  // value move, and every node except "here" is clickable to jump there.
  const chainRefs = origin
    ? dependencyChain(qualified, origin, changeByRef)
    : [];

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

      {/* Dependency chain — origin → this cell, each node showing the actual
          formula and value move so a reviewer sees HOW the number was reached. */}
      {origin && (
        <div className="chain">
          <div className="h">How this value was reached</div>
          {chainRefs.map((ref, i) => {
            const node = ref === qualified ? change : changeByRef.get(canonRef(ref));
            const isHere = ref === qualified;
            const clickable = !isHere && isNavigable(ref);
            return (
              <div key={ref}>
                {i > 0 && <div className="chain-arrow">↓</div>}
                <div
                  className={
                    "chain-node" +
                    (isHere ? " here" : "") +
                    (clickable ? " nav" : "")
                  }
                  onClick={clickable ? () => onNavigate(ref) : undefined}
                  title={clickable ? "Jump to this cell" : undefined}
                >
                  <div className="cn-head">
                    <span className="sha">{canonRef(ref)}</span>
                    {node?.label && <span className="cn-label">{node.label}</span>}
                    {node && (
                      <span className={"cn-tag tag-" + classDot(node.classification)}>
                        {node.classification}
                      </span>
                    )}
                    {isHere && <span className="cn-here">← you are here</span>}
                  </div>
                  {node?.newFormula && (
                    <div className="cn-formula">{node.newFormula}</div>
                  )}
                  {node && (
                    <div className="cn-val">
                      <ValueDelta
                        old={node.oldValue}
                        next={node.newValue}
                        fmt={node.displayFormat}
                      />
                    </div>
                  )}
                </div>
              </div>
            );
          })}
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
