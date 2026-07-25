import type { CellChange, Cascade } from "../data/types";
import { formatDelta } from "../data/format";
import { revisionsFor } from "../data/cellHistory";
import { qualify, canonRef, splitRef } from "../data/refs";
import { isAuthored, classDot } from "../data/classify";
import { dependencyChain } from "../data/graph";
import { ValueDelta } from "./ValueDelta";
import { OpenInExcel } from "./OpenInExcel";

interface Props {
  change: CellChange;
  sheet: string;
  cascadeByOrigin: Map<string, Cascade>;
  changeByRef: Map<string, CellChange>;
  toPath: string; // DiffResult.to.path — the workbook version to open in Excel
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
  toPath,
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

  // Downstream, for an AUTHORED cell. A computed cell can be traced upwards to
  // its cause; before this, an authored cell — the one a human actually typed,
  // and the most important cell in any commit — was a dead end showing only a
  // "ripples to N cells" count. The cascade already carries the affected refs
  // and a pre-sorted topMovers list; this renders them, so the blast radius is
  // navigable in the direction the product is actually about.
  const sheetsTouched = cascade
    ? new Set(cascade.affected.map((r) => splitRef(r).sheet)).size
    : 0;
  // topMovers can carry up to 10 (DATA-CONTRACT). Showing all of them buries
  // the history and reassurance sections below the fold, and past the first
  // handful "biggest movers" stops meaning much — the rest are highlighted in
  // the grid anyway.
  const MAX_MOVERS = 6;
  const movers = (cascade?.topMovers ?? []).slice(0, MAX_MOVERS);

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

      {/* The before → after is THE fact about this cell, so it gets the room
          rather than sitting as one row among several. */}
      <div className="d-hero">
        <div className="d-hero-val">
          <ValueDelta
            old={change.oldValue}
            next={change.newValue}
            fmt={change.displayFormat}
          />
        </div>
        <div className={"d-hero-delta" + (up ? " up" : "")}>
          {formatDelta(change.oldValue, change.newValue, change.displayFormat)}
        </div>
      </div>

      {/* Full circle: jump back to this cell's workbook in the real spreadsheet
          app. Opens a read-only copy of this version at the cell's sheet
          (disabled in browser dev — only the desktop build can launch Excel). */}
      <OpenInExcel
        path={toPath}
        sheet={sheet}
        label={qualified}
        className="detail-openx"
      />

      <div className="dr">
        <span className="k">Type</span>
        <span className={"tag-" + classDot(change.classification)}>
          {change.classification}
        </span>
      </div>

      {/* Downstream — what this authored edit moved. The mirror image of the
          dependency chain below: that answers "how did this value happen?",
          this answers "what did I break?". */}
      {authored && cascade && cascade.affectedCount > 0 && (
        <div className="chain downstream">
          <div className="h">What this changed</div>
          <div className="ds-sub">
            {cascade.affectedCount} {cascade.affectedCount === 1 ? "cell" : "cells"}
            {sheetsTouched > 1 && ` across ${sheetsTouched} sheets`}
            {movers.length > 0 && " — biggest movers:"}
          </div>
          {movers.map((m) => {
            // Mover carries no displayFormat (DATA-CONTRACT), so recover it
            // from the changed cell keyed by the same canonical ref.
            const node = changeByRef.get(canonRef(m.ref));
            const clickable = isNavigable(m.ref);
            const rising = (m.magnitude ?? 0) > 0;
            return (
              <div
                key={m.ref}
                className={"chain-node ds-node" + (clickable ? " nav" : "")}
                onClick={clickable ? () => onNavigate(m.ref) : undefined}
                title={clickable ? "Jump to this cell" : undefined}
              >
                <div className="cn-head">
                  <span className="sha">{canonRef(m.ref)}</span>
                  {m.label && <span className="cn-label">{m.label}</span>}
                </div>
                <div className="cn-val">
                  <ValueDelta
                    old={m.oldValue}
                    next={m.newValue}
                    fmt={node?.displayFormat}
                  />{" "}
                  <span className={"d-delta" + (rising ? " up" : "")}>
                    {formatDelta(m.oldValue, m.newValue, node?.displayFormat)}
                  </span>
                </div>
              </div>
            );
          })}
          {cascade.affectedCount > movers.length && (
            <div className="ds-rest">
              + {cascade.affectedCount - movers.length} more — highlighted in the
              grid
            </div>
          )}
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

      {/* No recorded timeline. Say so rather than ending the panel abruptly —
          an absent section reads as a broken one. Note this is currently a
          data limit, not a UI one: revisionsFor() is fed by the bundled
          commit-history fixture, and the live daemon store supplies no
          per-cell history at all. */}
      {revisions.length === 0 && (
        <div className="timeline">
          <div className="h">History</div>
          <div className="tl-empty">No earlier revisions recorded for this cell.</div>
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
