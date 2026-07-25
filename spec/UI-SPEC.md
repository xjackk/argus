# UI-SPEC.md — desktop app

> **Reference:** `wireframes/ui-mockups.html` (open in a browser) — five states, dark theme, built from the real GitHub Desktop screenshots. `wireframes/shot-ui-mockups.png` is the flat render.
>
> **Governing principle:** copy GitHub Desktop's *chrome* faithfully; the diff pane is the one place that must diverge, because a spreadsheet is 2D and a code diff is 1D.

---

## Corrected layout — it's THREE panes, not two

An earlier draft of this spec described a two-pane body. The real GitHub Desktop (verified from screenshots) is:

```
┌─ workbook ▾ ─┬─ scenario ▾ ─┬─ sync ────────────────────────────┐
├──────────────┴──────────────┴───────────────────────────────────┤
│ Changes │ History │  commit title + description + author + stat │
├──────────────┬──────────────────┬───────────────────────────────┤
│ version list │ changed sheets   │  spreadsheet diff grid        │
│ (210px)      │ (170px)          │  (flex)                       │
└──────────────┴──────────────────┴───────────────────────────────┘
```

The middle pane is the important discovery. GitHub's "10 changed files" becomes **"2 changed sheets"** with per-sheet change counts. Click a sheet → its grid renders on the right. This replaces the sheet-tabs idea from earlier drafts and is a direct structural borrow.

> **"Assumptions / P&L / Debt / Returns" are not features.** They're the sheet tabs inside the actual `.xlsx`. Every workbook has them; a 30-sheet model needs navigation. This pane is read from the file, not designed.

## Mapping

| GitHub Desktop | This app |
|---|---|
| Current repository ▾ | Current workbook ▾ |
| Current branch ▾ | Current scenario ▾ (fork) |
| Fetch origin | Sync SharePoint / Commit version |
| Commit list (left rail) | Version history (c01…c07) |
| "10 changed files" | "2 changed sheets" |
| Diff pane | **Spreadsheet grid with cascade** |
| `+127 −0` | `1 authored · 4 computed` |
| Click a line | **Click a cell → dependency chain + revision history** |
| Commit description panel | Reviewer rationale (feeds AI narration) |

---

## Component tree

```
<App>
├── <TopBar>
│   ├── <WorkbookPicker>        dropdown, filter list
│   ├── <ScenarioPicker>        dropdown (branch equivalent)
│   └── <SyncButton>            contextual: Sync / Commit / Pull
├── <Body>
│   ├── <VersionRail>                        210px, --bg2
│   │   ├── <Tabs>                           Changes | History
│   │   ├── <VersionFilter>                  text input
│   │   └── <VersionRow>[]                   from commit-history.json
│   │       ├── avatar · summary · author · relative time
│   │       ├── <ChangeCounts>               "1 authored · 4 computed"
│   │       └── <SignOffBadge?>  <AnomalyBadge?>
│   └── <DiffColumn>
│       ├── <CommitHeader>
│       │   ├── title · <DescriptionPanel> (expandable)
│       │   └── author · id · <DiffStat>
│       └── <SplitPane>
│           ├── <SheetList>                  170px
│           │   ├── "N changed sheets"
│           │   ├── <SheetRow>[]             name + change count; unchanged = muted
│           │   └── <AiSummaryBox>           narrative from DiffResult.summary
│           └── <GridPane>
│               ├── <GridHeader>             sheet name + <CascadeToggle>  ← THE control
│               ├── <Legend>                 authored / computed / ƒ
│               ├── <VirtualGrid>            ← MUST virtualize
│               │   └── <DiffCell>[]         old struck-through + new below
│               └── <CellDetail>             on click → <DependencyChain> + <RevisionTimeline>
└── <FirstRun>                               when no workbooks tracked
    ├── <DetectedFolders>                    OneDrive / SharePoint auto-detect
    ├── <StartOption>[]                      Watch folder (primary) | Add workbook | Join team
    └── <FolderSetup>                        checkbox list + "good fit" / "no formulas" badges
```

### Data sources
- `<VersionRail>`, `<CommitHeader>` → `test-workbooks/commit-history.json`
- `<SheetList>`, `<VirtualGrid>`, `<CellDetail>`, `<AiSummaryBox>` → `DiffResult` (see `DATA-CONTRACT.md`); fixture at `test-workbooks/fixture-v1-to-v2.json`

---

## Tokens

Defined at the top of `ui-mockups.html`. **Use CSS variables from day one** — a light theme then becomes a variable swap rather than a rewrite. Retrofitting theming is tedious; setting it up costs nothing now.

| Purpose | Var | Dark value |
|---|---|---|
| App bg / panel / raised | `--bg` `--bg2` `--bg3` | `#1c2128` `#22272e` `#2d333b` |
| Borders | `--line` `--line2` | `#373e47` `#2d333b` |
| Text primary / muted / faint | `--tx` `--tx2` `--tx3` | `#adbac7` `#768390` `#545d68` |
| Accent (selection, primary) | `--blue` | `#316dca` |
| **Authored** change | `--green` / `--greenbg` | `#57ab5a` / `#1b4721` |
| **Computed** (cascade) | `--purple` / `--purplebg` | `#986ee2` / `#2f2b52` |
| Old value / decrease | `--red` | `#e5534b` |
| Anomaly | `--amber` / `--amberbg` | `#e5a83b` / `#3d2113` |

Numbers and cell refs use `--mono`. Everything else system sans.

> **Color carries meaning:** green = a human typed it, purple = a formula recomputed it. That distinction *is* the product. Don't reuse these hues for anything else.

---

## The five states

**1 · Main window** — three panes, cascade view active. The `<CascadeToggle>` in the grid header is the single most important control in the app; toggling it is the demo moment (1 cell → 5 cells).

**2 · Cell detail** — click a cell → formula, before/after, **dependency chain** (how the change reached this cell), and **revision timeline** (every time it moved, who caused it). The closing line — "IRR has never been directly edited" — is computed from the classification field and is exactly the reassurance a reviewer wants. This is "git log for a cell."

**3 · First run** — "Watch a folder" is the primary action, with the OneDrive/SharePoint folder **pre-detected** so the user sees "14 workbooks found" before doing anything. `No account needed` supports bottom-up adoption: one person gets value alone before any admin is involved.

**4 · Folder setup** — checkbox list of detected workbooks. The **"no formulas" badge** filters out contact lists and trackers where the cascade does nothing, so users only track workbooks the product is good at — and it teaches what the tool is *for* without a tutorial.

**5a · Summary-first** — **recommended default for non-technical users.** Lead with the plain-English sentence, back it with three metric cards, make the grid a deliberate "See what changed" click-through. A reviewer with 90 seconds gets everything without touching a cell; the power user clicks in. Same product, two depths.

**5b · Anomaly** — formula replaced by a typed constant, with the reasoning spelled out ("IRR rose 5.2 points, but no input assumption changed to justify it"). Uses the real `c06→c07` test data. Strongest demo moment after the cascade toggle.

---

## Design decisions worth preserving

**The grid must diverge from GitHub.** A code diff stacks removed-above-added as separate rows because it's a 1D list of lines. A spreadsheet cell has *one position and two values*. So: new value prominent, old value struck-through beneath (or on hover). Forcing stacked red/green rows onto a 2D grid fights the medium.

**Virtualize from the start.** Render only visible rows (react-window / TanStack Virtual). A non-virtualized grid on a real workbook freezes — and "snappy native app" is part of the pitch. This is not a "add it later" item.

**Density is a real risk.** GitHub Desktop is designed for developers who live in diffs. A PE associate opening a four-pane interface with colored cells and monospace numbers may find it intimidating. Mitigation: state 5a as the entry point, grid as drill-down. Watch for this in user testing.

**Honest caveat:** these are mockups, and mockups always look better than the built thing. Real workbooks have 30 sheets, wide columns, long labels, and merged cells that will make the grid messier than what's drawn. Budget for that gap.

---

## Hackathon build order

1. `<VirtualGrid>` + `<DiffCell>` against `fixture-v1-to-v2.json` — the demo lives or dies here
2. `<CascadeToggle>` — the money shot
3. `<VersionRail>` from `commit-history.json` — makes it feel real
4. `<AiSummaryBox>` — cheap, high impact
5. `<CellDetail>` — if time permits
6. First-run / folder-setup states — **skip entirely for the demo** (you'll open two files from disk)
