import type { CellChange, Cascade } from "../data/types";
import { formatValue } from "../data/format";

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
  const qualified = `${sheet}!${change.coord}`;
  const authored = change.classification === "authored";
  const cascade = cascadeByOrigin.get(qualified);

  // Keep the card on-screen: flip left/up near the right/bottom edges.
  const flipX = x > window.innerWidth - 300;
  const flipY = y > window.innerHeight - 180;
  const style: React.CSSProperties = {
    left: flipX ? undefined : x + 14,
    right: flipX ? window.innerWidth - x + 14 : undefined,
    top: flipY ? undefined : y + 14,
    bottom: flipY ? window.innerHeight - y + 14 : undefined,
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
          {formatValue(change.oldValue, change.displayFormat)} →{" "}
          <b>{formatValue(change.newValue, change.displayFormat)}</b>
        </span>
      </div>
      <div className="hc-row">
        <span className="k">Type</span>
        <span className={"pill " + (authored ? "a" : "c")}>
          {authored ? "authored" : "computed"}
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
