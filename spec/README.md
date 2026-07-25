# spec/ — design & contract reference

Ported from the original context package (`~/Downloads/argus-files`), which is **not**
version controlled. This directory is the version-controlled copy.

| File | Status |
| --- | --- |
| `DATA-CONTRACT.md` | **Current.** The frozen engine ⇄ UI JSON contract. Matches `engine/types.go`. |
| `UI-SPEC.md` | **Current.** Component tree, design tokens, the five app states. |
| `PROJECT.md` | **Current except where noted** — see the header inside; `/ROADMAP.md` supersedes §5, §8, §13. |
| `wireframes/ui-mockups.html` | **Current.** Five states, dark theme. Open in a browser. |
| `wireframes/wireframe-arch.html` | **Current.** Topology / source-of-truth diagram. |

Not ported, deliberately:

- `START-HERE.md` — its "first task before any app code" (verify `excelize`+`efp` exposes
  cached values, formulas, cross-sheet refs, shared formulas) is **done and passed**, and
  its read-order described the Downloads folder layout.
- `HACKATHON.md` — the one-day plan is spent. Its "Deliberately faking" table is still a
  live inventory of known debt; that content belongs in `/ROADMAP.md` §8.
- `wireframes/wireframe-main.html` + `shot-main.png` — `UI-SPEC.md` marks them superseded
  by `ui-mockups.html`.

Test workbooks moved to `engine/testdata/` (see the README there).
