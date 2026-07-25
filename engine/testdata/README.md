> **Ported into the repo 2026-07-25.** Paths in the original text below refer to the old
> `~/Downloads/argus-files/test-workbooks/` layout. In this repo:
>
> - The workbooks (`atlas_v1–v5`, `atlas_c01–c07`) are **here**, in `engine/testdata/`.
> - The generators (`build_lbo.py`, `build_versions.py`, `build_chain.py`) are in `/scripts/`.
> - `commit-history.json` lives at `desktop/frontend/src/data/commit-history.json`.
> - `fixture-v1-to-v2.json` was **not** ported — the `fixture.json` it fed was deleted in `dd91ca9`.
> - `scripts/gen-fixtures.sh` still reads workbooks from `~/Downloads/...` by default;
>   override with `ARGUS_WORKBOOKS=engine/testdata`.

---

# Test workbooks — Project Atlas LBO model

Four version-pairs derived from one base LBO model, each designed to exercise a **specific** diff/cascade behavior the engine must handle. Use them two ways:

1. **Engine fixtures** — feed `atlas_v1_base.xlsx` + any later version to the diff engine and assert the expected behavior below.
2. **xltrail trial comparison** — upload the same pairs to xltrail's free trial and see how *its* comparison view handles them. The key question (§4 of PROJECT.md): does xltrail distinguish the *authored input change* from the *downstream recalculations*, or does it just show a flat list of every changed cell? That comparison is the whole competitive thesis.

## The model

`atlas_v1_base.xlsx` — a real LBO with **cross-sheet formula dependencies**, not hardcoded values:
- **Assumptions** — blue input cells (entry/exit multiples, growth, margins, interest, tax). These are the only hand-edited cells.
- **P&L** — revenue grows off the growth input; EBITDA margin ramps entry→exit; interest links to the Debt schedule; NI flows down. All formulas.
- **Debt** — opening/closing balances driven by the paydown input; feeds interest back into P&L.
- **Returns** — Entry/Exit EV, Equity, **MOIC**, **IRR**, all computed off the other three sheets.

Base outputs: Exit EV ≈ $1,928mm · MOIC ≈ 3.36x · IRR ≈ 27.5%. Change any single input and the whole chain recomputes — that ripple is the cascade the product visualizes.

## The version pairs

| File | Change (authored) | What it tests | Expected engine behavior |
|---|---|---|---|
| `atlas_v2_exit_multiple.xlsx` | Exit EV/EBITDA **10.5x → 9.5x** (one cell, `Assumptions!B5`) | **Pure cascade.** One authored input; everything downstream moves. | Classify `B5` as **authored**; classify the resulting Exit EV / MOIC / IRR moves (and every Returns/P&L cell that shifts) as **computed**. Blast radius traces from one cell. Outputs: MOIC 3.36→2.99x, IRR 27.5→24.5%. |
| `atlas_v3_downside.xlsx` | Growth 8→6.5%, entry margin 22→20%, interest 7.5→8.25% (**3 inputs, 2 sheets' worth of ripple**) | **Multi-input, overlapping cascades.** Ripples interact. | 3 authored cells; large computed set. Note IRR actually *rises* to 28.2% (lower interest offsets lower growth) — a great demo of why blast-radius visibility matters: the net effect is non-obvious. |
| `atlas_v4_inserted_row.xlsx` | Inserted a **"Stock-Based Comp" row** into P&L (row 7), shifting rows below | **Structural alignment — THE make-or-break test.** A naive `(row,col)` diff flags the whole lower half of P&L as changed. | Must **align rows** (match on row label / fingerprint) and report *one inserted row*, not "everything below row 7 changed." Returns outputs are unchanged by design, so this isolates the structural behavior with no output noise. |
| `atlas_v5_hardcode_override.xlsx` | Replaced the Exit EV **formula** (`=Assumptions!B5*B8`) with a **hardcoded 2100** in `Returns!B9` | **Anomaly / "smells wrong" test** for the AI-flagging feature. | Detect **formula → constant** substitution — a classic silent-error pattern (someone overwrote a calc with a typed number). This is exactly what the AI triage layer (§11) should flag for human attention. Outputs jump (MOIC 3.71x) with no input justification. |

## Regenerating
`build_lbo.py` builds the base; `build_versions.py` derives the four pairs. Both in the repo root of this package. Recalculated with LibreOffice so **cached values are stored** — which is what the engine reads to diff without recomputing.

---

# Linear commit chain (for the git-log / history view)

The `atlas_v*` files above are **forks off the base** — good for isolated diff tests, but they can't demo a *history*. The `atlas_c01…c07` files are a **linear chain**: each is derived from its parent, so **any two commits are legitimately comparable** and per-cell history over time is meaningful.

| Commit | File | Author | Team | Change |
|---|---|---|---|---|
| c01 | `atlas_c01_initial.xlsx` | J. Killilea | Deal Team A | Initial model shared |
| c02 | `atlas_c02_growth_case.xlsx` | M. Rivera | Deal Team A | Revenue growth → 9.5% |
| c03 | `atlas_c03_margin_tighten.xlsx` | A. Chen | Modeling | Exit EBITDA margin → 23.5% |
| c04 | `atlas_c04_debt_repricing.xlsx` | A. Chen | Modeling | Interest → 8.25%, paydown → 12.5% |
| c05 | `atlas_c05_add_sbc_line.xlsx` | M. Rivera | Deal Team A | **Inserted** stock-based comp row into P&L |
| c06 | `atlas_c06_exit_multiple.xlsx` | S. Patel (VP) | IC / Partners | Exit multiple → 9.5x · **✓ signed off** |
| c07 | `atlas_c07_hardcode_flag.xlsx` | M. Rivera | Deal Team A | **⚠ Hardcoded Exit EV override** |

### The story the chain tells (IRR progression)

```
c01  27.5%   initial
c02  29.5%   (+2.0)  growth case — optimistic
c03  27.7%   (-1.8)  modeling tightens margin
c04  28.3%   (+0.6)  debt repricing nets positive
c05  28.3%   ( 0.0)  structural only — SBC feeds EBIT, not EBITDA
c06  25.4%   (-2.9)  VP marks exit multiple down
c07  30.6%   (+5.2)  ⚠ hardcode override — NO legitimate input justifies this jump
```

That last jump is the demo's punchline: a 5-point IRR swing with no input change behind it. That's the silent error the anomaly flag catches.

### `commit-history.json`

Manifest powering the history sidebar and per-cell history view:
- **`commits[]`** — id, file, author, email, team, timestamp, message, parent, `authoredCount`/`computedCount`, `touchedCells`, `signedOff` (c06), `anomalyFlags` (c07).
- **`cellHistory[]`** — per-cell revision timeline. `Returns!B14` (IRR) has **5 revisions** across the chain — click any cell, see every time it moved, who caused it, and whether the change was authored or computed.

### ⚠️ Known artifact — c05 and positional diff

Diffing c04→c05 with **positional** diff reports **42 authored changes** for what is really *one inserted row*. This is the naive-diff failure mode `PROJECT.md` §12 Risk #2 warns about, made concrete. It's correct output for a positional differ — and it's your benchmark: real keyed-row alignment should report **1 row inserted**, not 42 edits. Don't demo c04→c05 until alignment is built.

### Regenerating
`build_chain.py` rebuilds the chain from `atlas_v1_base.xlsx`. All files recalculated via LibreOffice so cached values are stored.
