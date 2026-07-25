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

---

# Developer guide

Everything you need to build, run, and iterate on Argus locally.

## Prerequisites

| Tool          | Version    | Notes                                                             |
| ------------- | ---------- | ----------------------------------------------------------------- |
| Go            | 1.25+      | `go version`                                                      |
| Node + npm    | 20+ / 10+  | For the React frontend                                            |
| Wails CLI     | v2.11+     | `go install github.com/wailsapp/wails/v2/cmd/wails@latest`        |
| Xcode CLT     | any        | `xcode-select --install` — clang, required by Wails on macOS      |
| LibreOffice   | optional   | Only to **regenerate test fixtures** (recalculates cached values) |

You do **not** need Excel or LibreOffice to build or run Argus — `excelize` reads
`.xlsx` directly, and the engine binary is pure Go (CGO-free). LibreOffice is
only used to author/refresh test workbooks so their cached values are stored.

## Repo structure — a Go workspace

This repo is a **Go workspace** (`go.work`) with **two modules**:

- **`argus`** (root) — the engine, CLI, and narrator. Lean dependency tree
  (`excelize`, `efp`), so `go test ./engine/...` stays fast and clean.
- **`desktop`** — the Wails app, which pulls in the full Wails/webview graph.

The workspace is what lets `desktop` import `argus/engine` without polluting the
engine module with Wails dependencies. `go.work` / `go.work.sum` are committed.

> ⚠️ **Do not move `engine/` or `narrator/` under `internal/`.** Because
> `desktop` is a separate module, Go would then forbid it from importing them and
> the Wails↔engine binding would break.

## Run the engine (CLI)

```sh
go build ./...                                   # build everything
go test ./engine/...                             # acceptance + anomaly + multi-input tests
go vet ./...                                     # should be clean

# Diff two workbooks → DiffResult JSON on stdout
go run ./cmd/argus-diff \
  engine/testdata/atlas_v1_base.xlsx \
  engine/testdata/atlas_v2_exit_multiple.xlsx

# Inspect the grounded narration prompt (no model call):
go run ./cmd/argus-diff --prompt-only  A.xlsx B.xlsx
# Fill summary.narrative via a live `claude -p` call (~3s, needs network):
go run ./cmd/argus-diff --narrate      A.xlsx B.xlsx
```

Test fixtures live in `engine/testdata/`; the full set (linear commit chain
`c01–c07`, all `v*` variants, `commit-history.json`) is in
`~/Downloads/argus-files/test-workbooks/`. The canonical demo pair is
`atlas_v1_base.xlsx → atlas_v2_exit_multiple.xlsx` (1 authored input, 4 computed
ripples).

## Run the desktop app

```sh
cd desktop
wails dev       # hot-reloading dev app (frontend deps auto-install on first run)
# or
wails build     # packages build/bin/argus.app
```

`wails dev` is the loop for UI work: edit anything under
`desktop/frontend/src/` and it hot-reloads. If `wails dev` misbehaves, run
`wails build` and open `build/bin/argus.app`.

## The dev workflow — rapid iteration

The engine is **not a server** — it's a one-shot CLI (or an in-process call
inside Wails). So most UI work needs nothing but the browser. Three tiers:

**Tier 1 — Pure UI, in the browser (90% of the work).**
The whole current UI renders from bundled engine output (the `c01→c07` commit
chain) and calls no Wails APIs, so it runs in a plain browser with instant
hot-reload and full DevTools:

```sh
cd desktop/frontend
npm run dev          # → http://localhost:5173
```

Edit anything under `desktop/frontend/src/` and it live-reloads. No engine, no
second terminal.

**Tier 2 — Regenerate the bundled diffs from the engine.**
The commit-chain diffs the UI renders live in `desktop/frontend/src/data/history/`
and are generated straight from `argus-diff` — never hand-edited. To refresh them
after an engine change (a one-shot command, not a running process), then let Vite
HMR pick them up:

```sh
make fixtures        # regenerates history/c0*.json for the whole chain
```

It diffs each consecutive `atlas_c0N → atlas_c0(N+1)` pair and bakes in a live
`claude -p` narrative for the single-input commits. Point it at a different
workbook set with `ARGUS_WORKBOOKS=/path make fixtures`.

**Tier 3 — Full native app (periodic + before shipping).**
Verify the real WKWebView rendering and the live engine binding:

```sh
cd desktop
wails dev            # real webview + live App.Diff; or `wails build` to package
```

Rule of thumb: **live in Tier 1**, run **Tier 2** after an engine change, and
run **Tier 3** to confirm it looks and behaves right in the actual app.

## Engine ⇄ UI integration state

Today the frontend renders **bundled engine output** — the `c01→c07` diffs in
`desktop/frontend/src/data/history/`, mapped commit→diff by
`desktop/frontend/src/data/diffs.ts`. This lets the UI be built and polished
against real, stable data with no backend running.

The Go backend already exposes the engine to the frontend:
`desktop/app.go` binds `App.Diff(pathA, pathB) → DiffResult`, and Wails
auto-generates the JS binding at `desktop/frontend/wailsjs/go/main/App.js`
(with the matching TS types in `wailsjs/go/models.ts`). To switch from bundled
diffs to live engine output, `diffs.ts` resolves a commit to its file pair and
calls `window.go.main.App.Diff(...)` — nothing else in the UI changes.

## Iterating on the UI

The intended loop: run `wails dev`, edit `desktop/frontend/src/`, and compare the
result against the target design. The three-pane layout, the authored↔cascade
toggle, per-cell `ƒ`/⚠ markers, hover cards, and the cell-detail panel all live
under `desktop/frontend/src/components/`; shared formatting/ref helpers are in
`desktop/frontend/src/data/`.
