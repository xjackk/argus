# DATA-CONTRACT.md — engine ⇄ UI

> **Why this file exists.** This is the interface between the diff engine and the desktop UI. Define it once, and the engine and frontend can be built **in parallel by separate agents** without touching each other. The UI builds against a static fixture; the engine builds against an assertion. They meet at the end and it just works.
>
> **Rule: this contract is frozen for the hackathon.** If something's missing, add an optional field — never rename or repurpose an existing one mid-day.

---

## Top-level shape

```json
{
  "schemaVersion": 1,
  "from": { "label": "Revenue build v2", "path": "atlas_v1_base.xlsx", "committedAt": "2026-07-19T09:00:00Z", "author": "M. Rivera" },
  "to":   { "label": "Lowered exit multiple", "path": "atlas_v2_exit_multiple.xlsx", "committedAt": "2026-07-19T14:30:00Z", "author": "J. Killilea" },
  "summary": {
    "authoredCount": 1,
    "computedCount": 4,
    "sheetsAffected": ["Assumptions", "Returns"],
    "narrative": "The exit multiple was lowered from 10.5x to 9.5x, reducing Exit EV by $183mm and dropping projected IRR from 27.5% to 24.5%."
  },
  "sheets": [ /* SheetDiff[] — see below */ ],
  "cascades": [ /* Cascade[] — see below */ ],
  "anomalies": [ /* Anomaly[] — see below */ ]
}
```

`summary.narrative` is the **AI-generated** field (PROJECT.md §11). Everything else is deterministic. Engine emits `narrative: null` if the AI step is skipped or fails — **the UI must render fine without it.**

---

## SheetDiff

One per sheet that has changes. `rowsInserted`/`rowsDeleted` stay empty arrays on day one (positional diff — see HACKATHON.md), but the fields exist so structural alignment slots in later without a contract change.

```json
{
  "name": "Returns",
  "changes": [ /* CellChange[] */ ],
  "rowsInserted": [],
  "rowsDeleted": []
}
```

## CellChange — the core record

```json
{
  "coord": "B9",
  "row": 9,
  "col": 2,
  "label": "Exit EV",
  "classification": "computed",
  "oldValue": 1928.49,
  "newValue": 1745.34,
  "oldFormula": "=Assumptions!B5*B8",
  "newFormula": "=Assumptions!B5*B8",
  "displayFormat": "$#,##0",
  "dependsOn": ["Assumptions!B5", "Returns!B8"],
  "dependents": ["Returns!B11"],
  "causedBy": ["Assumptions!B5"],
  "magnitude": -0.095
}
```

**Field notes:**

| Field | Notes |
|---|---|
| `coord` | Sheet-local A1 (`"B9"`). Fully-qualified refs elsewhere use `"Sheet!Coord"` (`"Assumptions!B5"`, `"'P&L'!B6"` — quote sheet names containing spaces/`&`). |
| `row`, `col` | 1-based ints. Redundant with `coord`, but the grid renderer wants them directly — don't make the UI parse A1. |
| `label` | Row label from column A, if present. Powers human-readable UI ("Exit EV" not "B9"). `null` when absent. |
| `classification` | **`"authored"`** — formula string changed, or a formula became a constant, or a constant changed. A human typed this. **`"computed"`** — formula identical, value differs (an upstream dep moved). This single field drives the entire cascade toggle. |
| `oldValue` / `newValue` | Cached computed values from the file. Number, string, bool, or null. **Never recomputed by us** — Excel already stored them. |
| `oldFormula` / `newFormula` | Formula string incl. leading `=`, or `null` for a constant cell. `oldFormula != null && newFormula == null` is the **hardcode-override anomaly**. |
| `displayFormat` | Excel number format, so the UI renders `2.99x` / `24.5%` / `$1,745` rather than raw floats. Falls back to `"General"`. |
| `dependsOn` | Fully-qualified precedents parsed from `newFormula`. Empty for constants. |
| `dependents` | Fully-qualified cells whose formulas reference this one (the inverted graph). |
| `causedBy` | Which **authored** cell(s) this computed change traces back to. Empty for authored cells. This is what makes "show me what this one edit broke" a lookup rather than a search. |
| `magnitude` | Fractional change `(new-old)/abs(old)` for numerics; `null` otherwise. Powers gradient coloring. Guard against `old == 0`. |

## Cascade — one per authored change

```json
{
  "origin": "Assumptions!B5",
  "originLabel": "Exit EV / EBITDA",
  "oldValue": 10.5,
  "newValue": 9.5,
  "affectedCount": 4,
  "affected": ["Returns!B9", "Returns!B11", "Returns!B13", "Returns!B14"],
  "topMovers": [
    { "ref": "Returns!B14", "label": "IRR", "oldValue": 0.2746, "newValue": 0.2454, "magnitude": -0.106 },
    { "ref": "Returns!B13", "label": "MOIC", "oldValue": 3.3645, "newValue": 2.9932, "magnitude": -0.110 }
  ]
}
```

`affected` is the full BFS reachable set (4 cells on the compact test model; hundreds on a real PE model). `topMovers` is the 5–10 highest-`|magnitude|` cells — **this is what the AI narrates and what the UI headlines.** Don't make the frontend sort 340 entries to find the interesting ones.

## Anomaly — the "smells wrong" flags

Deterministic detections (no LLM required). Powers the ⚠️ badges.

```json
{
  "type": "formula_replaced_by_constant",
  "ref": "Returns!B9",
  "label": "Exit EV",
  "severity": "high",
  "message": "Formula '=Assumptions!B5*B8' was replaced with the hardcoded value 2100.",
  "oldFormula": "=Assumptions!B5*B8",
  "newValue": 2100
}
```

Day-one types (all rule-based, cheap to implement):
- **`formula_replaced_by_constant`** — `oldFormula != null && newFormula == null`. The `atlas_v5` test case. Highest-value flag; classic silent-error pattern.
- **`large_magnitude_change`** — `|magnitude| > 0.5` on a computed cell.
- **`formula_inconsistent_in_row`** — a cell's formula differs from its row neighbors' pattern (broken fill).

---

## Go types

```go
type DiffResult struct {
    SchemaVersion int         `json:"schemaVersion"`
    From          VersionMeta `json:"from"`
    To            VersionMeta `json:"to"`
    Summary       Summary     `json:"summary"`
    Sheets        []SheetDiff `json:"sheets"`
    Cascades      []Cascade   `json:"cascades"`
    Anomalies     []Anomaly   `json:"anomalies"`
}

type VersionMeta struct {
    Label       string `json:"label"`
    Path        string `json:"path"`
    CommittedAt string `json:"committedAt"`
    Author      string `json:"author"`
}

type Summary struct {
    AuthoredCount  int      `json:"authoredCount"`
    ComputedCount  int      `json:"computedCount"`
    SheetsAffected []string `json:"sheetsAffected"`
    Narrative      *string  `json:"narrative"`  // nil if AI skipped — UI must tolerate
}

type SheetDiff struct {
    Name         string       `json:"name"`
    Changes      []CellChange `json:"changes"`
    RowsInserted []int        `json:"rowsInserted"`
    RowsDeleted  []int        `json:"rowsDeleted"`
}

type CellChange struct {
    Coord          string   `json:"coord"`
    Row            int      `json:"row"`
    Col            int      `json:"col"`
    Label          *string  `json:"label"`
    Classification string   `json:"classification"` // "authored" | "computed"
    OldValue       any      `json:"oldValue"`
    NewValue       any      `json:"newValue"`
    OldFormula     *string  `json:"oldFormula"`
    NewFormula     *string  `json:"newFormula"`
    DisplayFormat  string   `json:"displayFormat"`
    DependsOn      []string `json:"dependsOn"`
    Dependents     []string `json:"dependents"`
    CausedBy       []string `json:"causedBy"`
    Magnitude      *float64 `json:"magnitude"`
}

type Cascade struct {
    Origin        string   `json:"origin"`
    OriginLabel   *string  `json:"originLabel"`
    OldValue      any      `json:"oldValue"`
    NewValue      any      `json:"newValue"`
    AffectedCount int      `json:"affectedCount"`
    Affected      []string `json:"affected"`
    TopMovers     []Mover  `json:"topMovers"`
}

type Mover struct {
    Ref       string   `json:"ref"`
    Label     *string  `json:"label"`
    OldValue  any      `json:"oldValue"`
    NewValue  any      `json:"newValue"`
    Magnitude *float64 `json:"magnitude"`
}

type Anomaly struct {
    Type       string  `json:"type"`
    Ref        string  `json:"ref"`
    Label      *string `json:"label"`
    Severity   string  `json:"severity"`   // "high" | "medium" | "low"
    Message    string  `json:"message"`
    OldFormula *string `json:"oldFormula,omitempty"`
    NewValue   any     `json:"newValue,omitempty"`
}
```

---

## How the UI consumes it

- **Authored-only view** → filter `changes` where `classification == "authored"`.
> **Note on scale:** the bundled test workbooks are compact — `v1→v2` yields **1 authored + 4 computed**. A real PE model produces hundreds. The contract and UI must handle both; don't hardcode assumptions about list length.

- **Cascade view** → render everything; color by `classification`, gradient by `magnitude`.
- **Click an authored cell** → look up its `Cascade` by `origin`, highlight the `affected` set.
- **Hover any cell** → show `oldValue`→`newValue`, `classification`, and `causedBy` ("caused by Assumptions!B5").
- **Header banner** → `summary.narrative` (skip the banner entirely if null).
- **⚠️ badges** → from `anomalies`, keyed by `ref`.

## Parallel-agent workflow

1. **Agent A (engine)** builds `diff(pathA, pathB) → DiffResult`, writes JSON to stdout.
2. **Agent B (UI)** builds against a static `fixture.json` — hand-write one matching the example above for `v1→v2`, in the first fifteen minutes.
3. They meet when the engine's real output replaces the fixture. **Nothing else needs to change.**

## Acceptance test (`atlas_v1_base.xlsx` → `atlas_v2_exit_multiple.xlsx`)

```
summary.authoredCount == 1
sheets["Assumptions"].changes contains coord "B5",
    classification "authored", oldValue 10.5, newValue 9.5
cascades has exactly 1 entry, origin "Assumptions!B5"
that cascade's affected includes Returns!B9, B11, B13, B14
Returns!B14 (IRR): classification "computed",
    oldValue ≈ 0.2746, newValue ≈ 0.2454, causedBy ["Assumptions!B5"]
```

Write this assertion **before** the engine works. It's the regression net for the whole day — every refactor stays honest against it.
