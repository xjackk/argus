import type { CSSProperties } from "react";
import type { CellChange, Cascade } from "../data/types";
import { qualify } from "../data/refs";
import { isAuthored, classDot } from "../data/classify";
import { ValueDelta } from "./ValueDelta";

const CARD_W = 300;
const CARD_H = 180;
const GAP = 14;

export interface HoverState {
  change: CellChange;
  sheet: string;
  x: number;
  y: number;
}

interface Props {
  hover: HoverState;
  cascadeByOrigin: Map<string, Cascade>;
}

export function HoverCard({ hover, cascadeByOrigin }: Props) {
  const { change, sheet, x, y } = hover;
  const qualified = qualify(sheet, change.coord);
  const authored = isAuthored(change);
  const cascade = cascadeByOrigin.get(qualified);

  // Keep the card on-screen: flip left/up near the right/bottom edges.
  const flipX = x > window.innerWidth - CARD_W;
  const flipY = y > window.innerHeight - CARD_H;
  const style: CSSProperties = {
    left: flipX ? undefined : x + GAP,
    right: flipX ? window.innerWidth - x + GAP : undefined,
    top: flipY ? undefined : y + GAP,
    bottom: flipY ? window.innerHeight - y + GAP : undefined,
  };

  return (
    <div className="hovercard" style={style}>
      <div className="hc-coord">
        {qualified}
        {change.label && <span className="sha">· {change.label}</span>}
      </div>
      <div className="hc-row">
        <span className="k">Change</span>
        <span className="val">
          <ValueDelta
            old={change.oldValue}
            next={change.newValue}
            fmt={change.displayFormat}
          />
        </span>
      </div>
      <div className="hc-row">
        <span className="k">Type</span>
        <span className={"pill " + classDot(change.classification)}>
          {change.classification}
        </span>
      </div>
      {authored && cascade ? (
        <div className="hc-row">
          <span className="k">Ripples to</span>
          <span className="val">{cascade.affectedCount} cells →</span>
        </div>
      ) : change.causedBy.length > 0 ? (
        <div className="hc-row">
          <span className="k">Caused by</span>
          <span className="val" style={{ color: "var(--green)" }}>
            {change.causedBy.join(", ")}
          </span>
        </div>
      ) : null}
    </div>
  );
}
