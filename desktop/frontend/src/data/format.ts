import type { CellValue } from "./types";

// Render a raw cell value using Excel displayFormat semantics, so the fixture's
// raw floats show as 2.99x / 24.5% / $1,745 rather than 2.9934... .
// Pragmatic subset of Excel number formats — enough for the demo formats:
//   "0.0\x"  "0.00\x"  "\$#,##0"  "0.0%"  "General"
export function formatValue(value: CellValue, fmt?: string | null): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "boolean") return value ? "TRUE" : "FALSE";
  if (typeof value === "string") return value;
  if (typeof value !== "number") return String(value);

  const n = value;
  if (!fmt || fmt === "General") return formatGeneral(n);

  // Drop Excel escaping (\) and quotes; keep the structural tokens.
  const clean = fmt.replace(/\\/g, "").replace(/"/g, "");

  const isPercent = clean.includes("%");
  const isCurrency = clean.includes("$");
  const hasThousands = clean.includes(",");

  // Decimal places = digits after the '.' in the first numeric token.
  const decMatch = clean.match(/[0#]*\.([0#]+)/);
  const decimals = decMatch ? decMatch[1].length : 0;

  // Trailing literal letters, e.g. the "x" in a multiple.
  const suffixMatch = clean.match(/([A-Za-z]+)\s*$/);
  const suffix = suffixMatch ? suffixMatch[1] : "";

  let num = isPercent ? n * 100 : n;
  let out = num.toLocaleString("en-US", {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
    useGrouping: hasThousands,
  });

  if (isCurrency) out = "$" + out;
  if (isPercent) out = out + "%";
  if (suffix) out = out + suffix;
  return out;
}

function formatGeneral(n: number): string {
  if (Number.isInteger(n)) return n.toLocaleString("en-US");
  return n.toLocaleString("en-US", { maximumFractionDigits: 4 });
}

// A compact signed delta for metric cards, respecting the format family.
export function formatDelta(
  oldValue: CellValue,
  newValue: CellValue,
  fmt?: string | null
): string {
  if (typeof oldValue !== "number" || typeof newValue !== "number") return "";
  const diff = newValue - oldValue;
  const clean = (fmt || "").replace(/\\/g, "").replace(/"/g, "");
  const sign = diff > 0 ? "+" : "−";
  const mag = Math.abs(diff);

  if (clean.includes("%")) {
    return `${sign}${(mag * 100).toFixed(1)}pts`;
  }
  const suffixMatch = clean.match(/([A-Za-z]+)\s*$/);
  if (suffixMatch) {
    return `${sign}${mag.toFixed(2)}${suffixMatch[1]}`;
  }
  if (clean.includes("$") || clean.includes(",")) {
    return `${sign}${Math.round(mag).toLocaleString("en-US")}`;
  }
  return `${sign}${mag.toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
}
