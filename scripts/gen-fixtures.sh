#!/usr/bin/env bash
# Regenerate the bundled commit-chain diffs the frontend renders, straight from
# the engine — so they can never hand-drift from argus-diff's real output.
#
# Usage:  ./scripts/gen-fixtures.sh          (or: make fixtures)
# Env:    ARGUS_WORKBOOKS=/path/to/test-workbooks  (default: ~/Downloads/argus-files/test-workbooks)
#
# Each cNN.json is the diff of atlas_c(NN-1) -> atlas_cNN. The single-input
# commits get a live `claude -p` narrative baked in; c05 (the noisy positional
# row-insert case) is left un-narrated.
set -euo pipefail
cd "$(dirname "$0")/.."

WB="${ARGUS_WORKBOOKS:-$HOME/Downloads/argus-files/test-workbooks}"
OUT="desktop/frontend/src/data/history"
BIN="$(mktemp -d)/argus-diff"

[ -d "$WB" ] || { echo "workbook dir not found: $WB (set ARGUS_WORKBOOKS)"; exit 1; }
mkdir -p "$OUT"
go build -o "$BIN" ./cmd/argus-diff

# id  parent-file             child-file              narrate?
rows=(
  "c02 c01_initial            c02_growth_case         narrate"
  "c03 c02_growth_case        c03_margin_tighten      narrate"
  "c04 c03_margin_tighten     c04_debt_repricing      narrate"
  "c05 c04_debt_repricing     c05_add_sbc_line        plain"
  "c06 c05_add_sbc_line       c06_exit_multiple       narrate"
  "c07 c06_exit_multiple      c07_hardcode_flag       narrate"
)

for r in "${rows[@]}"; do
  read -r id parent child mode <<<"$r"
  flag=""
  [ "$mode" = "narrate" ] && flag="--narrate"
  echo "  $id  ($parent -> $child)${flag:+  [narrated]}"
  "$BIN" $flag "$WB/atlas_$parent.xlsx" "$WB/atlas_$child.xlsx" > "$OUT/$id.json"
done

echo "Regenerated ${#rows[@]} diffs into $OUT"
