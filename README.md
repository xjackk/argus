# Argus

Argus diffs two Excel workbooks and explains what actually changed — separating
the handful of inputs a human typed from the hundreds of cells that moved as a
consequence.

The engine **never recomputes spreadsheet values.** It reads the cached values
Excel already stored, diffs them positionally, builds a cross-sheet dependency
graph from the formulas, classifies each change as `authored` or `computed`, and
traces the cascade (blast radius) from each authored edit.

## Layout

| Path              | What it is                                                        |
| ----------------- | ----------------------------------------------------------------- |
| `engine/`         | The deterministic diff engine. No AI, no I/O beyond reading files. |
| `engine/types.go` | The frozen engine ⇄ UI contract (`DiffResult`).                    |
| `narrator/`       | Optional post-processing: turns a `DiffResult` into prose.         |
| `cmd/argus-diff/` | CLI — diffs two workbooks, prints `DiffResult` JSON.               |
| `desktop/`        | Wails desktop shell (Go + React/TypeScript frontend).              |

## Usage

```sh
go build ./...
go test ./engine/...

go run ./cmd/argus-diff old.xlsx new.xlsx           # DiffResult JSON, narrative=null
go run ./cmd/argus-diff --prompt-only old.xlsx new.xlsx  # show the grounded prompt, no model call
go run ./cmd/argus-diff --narrate old.xlsx new.xlsx      # fill narrative via `claude -p`
```

## The AI boundary

The engine is fully deterministic and emits `summary.narrative = null`. The
narrator is a strictly separate post-processing step — the engine never imports
it. The model only narrates facts already computed, with every number
pre-rendered through its `displayFormat`; it never judges whether a value is
correct, and it never does arithmetic. If narration fails, `narrative` stays
`null` and the UI tolerates it.

## Desktop app

```sh
cd desktop
wails dev     # or: wails build
```
