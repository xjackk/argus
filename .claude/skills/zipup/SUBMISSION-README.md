<img src="argus-panoptes.jpg" alt="Argus Panoptes, the hundred-eyed watchman of Greek myth" width="100%">

# Argus — every spreadsheet change, explained

**Version control and diff for Excel models, built for finance teams.**

Live: **https://xjackk.github.io/argus/** · Source: **https://github.com/xjackk/argus**

---

## The problem

Private equity and investment banking run on Excel. Models move by email and
SharePoint as `Model_v7_FINAL_JK_v2.xlsx`. Review is manual: open two versions
side by side and eyeball which numbers moved, then try to reason about *why*.

That burns senior time, and it's error-prone in an expensive way — one missed
formula change flows straight into a valuation, a loan decision, an IC memo.

## What nobody else does

Existing tools — including the incumbent, xltrail — show you a **flat list of
changed cells**. They tell you *that* something changed.

Argus tells you **what it broke**.

For every version, it separates the handful of inputs a human actually typed
(**authored**) from the hundreds of cells that moved as a consequence
(**computed**), and traces every ripple back to the edit that caused it. One
analyst lowers an exit multiple from 10.5x to 9.5x; Argus shows the 40 downstream
cells that moved and walks you from IRR back to the assumption that changed it.

That's the **cascade**, and it needs a real formula dependency graph underneath —
which is the hard part of the build, and the reason it isn't a UI reskin anyone
can copy in a weekend.

## Watch it (19 seconds)

`demo.mp4` — or `demo.gif` if your viewer won't play video.

| | |
|---|---|
| `screenshots/01-main-cascade.png` | the cascade view: green = a human typed it, purple = a formula recalculated |
| `screenshots/02-authored-only.png` | the same commit filtered to just the human edits |
| `screenshots/03-cell-inspector-chain.png` | click any cell → the dependency chain that produced it, plus its revision history |
| `screenshots/04-anomaly.png` | a formula silently replaced by a hardcoded number — the classic silent error |
| `screenshots/05-history-rail.png` | commit history per workbook, with sign-off |

## Run it

```sh
# Requires Go 1.25+. No Excel or LibreOffice needed — the engine reads .xlsx directly.
cd source

# 1. The engine on its own — two workbooks in, structured diff JSON out
go test ./engine/...
go run ./cmd/argus-diff \
  engine/testdata/atlas_v1_base.xlsx \
  engine/testdata/atlas_v2_exit_multiple.xlsx

# 2. The desktop app (needs Node 20+ and the Wails CLI)
cd desktop && wails dev
```

The acceptance test in `engine/diff_test.go` is the whole thesis in one
assertion: `atlas_v1 → atlas_v2` must report **1 authored change** and a computed
set including Exit EV, MOIC, and IRR — not "800 cells changed."

## How it works

**The engine never recomputes your numbers.** Excel already stored the computed
values in the file; Argus reads those, so it can never disagree with what the
analyst saw on screen. It then:

1. parses both workbooks (cached values, formula strings, number formats)
2. builds a cross-sheet dependency graph from the formulas
3. classifies each change — formula changed ⇒ **authored**, value moved but
   formula identical ⇒ **computed**
4. runs a BFS from each authored edit to get its blast radius
5. flags rule-based anomalies (a formula replaced by a typed constant, etc.)

An AI layer sits **on top** of that deterministic output and narrates it in plain
English. It never computes and never judges whether a number is right — it only
describes changes the engine already proved. Grounding it this way is what makes
it safe to show a finance reviewer.

Written in Go (engine, daemon, CLI) with a React/TypeScript front end via Wails —
one language, no serialization seam between the engine and the app, and a single
static binary for teams that need to self-host.

## What's in this archive

```
README.md          this file
demo.mp4 / .gif    19-second walkthrough
screenshots/       stills of each major view
source/            complete source at the submitted commit
```

`node_modules/` and build output are excluded — `npm install` and `go build`
regenerate them.
