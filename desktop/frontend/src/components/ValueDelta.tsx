import { formatValue } from "../data/format";
import type { CellValue } from "../data/types";

interface Props {
  old: CellValue;
  next: CellValue;
  fmt?: string | null;
}

// The shared "oldValue → **newValue**" inline render, used by the hover card,
// cell-detail change row, revision timeline, and the changes list.
export function ValueDelta({ old, next, fmt }: Props) {
  return (
    <>
      {formatValue(old, fmt)} → <b>{formatValue(next, fmt)}</b>
    </>
  );
}
