> ⚠️ **RECONCILIATION NOTE — added when this file was ported into the repo (2026-07-25).**
>
> This is the original context document and its reasoning is preserved verbatim. Two parts
> are now out of date:
>
> - **§5, §8, §13 — deployment tiers and phase plan.** The SaaS → SharePoint → self-host
>   ordering is **superseded by `/ROADMAP.md`**, which inverts it to self-host-first.
>   Where the two disagree, `/ROADMAP.md` wins.
> - **§12 Risk #1 — "[LOAD-BEARING — VERIFY FIRST]"** — this was **verified and passed**.
>   `excelize` + `efp` expose cached values, formula strings, cross-sheet references, and
>   shared-formula expansion. No raw-XML fallback is needed. Do not re-run this probe.
>
> Everything else stands: the cascade is still the one differentiator, the engine is still
> the source of truth, the AI still narrates and never judges.

---

# Excel VCS — "GitHub Desktop for Finance"
### Project context & get-started guide

> **Purpose of this file.** This is the context-injection / kickoff document for the project. It captures the vision, the market reality, the architecture decisions and their reasoning, the open risks, and a phased build plan. It is written to be fed to Claude Code (or any collaborator) as the single source of truth for "what are we building and why." It is deliberately opinionated where decisions are made and deliberately honest where they are not.

---

## 1. One-line vision

A version-control and diff tool for Excel workbooks — **GitHub Desktop, but for finance teams** — whose defining feature is not version history (incumbents have that) but the **cascade / blast-radius view**: showing not just *what* changed in a model, but *what those changes broke or moved downstream*, with every change attributed to a person.

## 2. The problem (why this exists)

Finance teams — private equity, investment banking, FP&A, corporate finance — live in Excel. Models move by email and SharePoint as files named `Model_v7_FINAL_JK_v2.xlsx`. Review is manual: someone opens two versions side by side and eyeballs which numbers moved, then tries to reason about *why*. This wastes enormous amounts of senior time and is error-prone — a single missed formula change can flow into a materially wrong output (a valuation, a loan decision, an IC memo).

The specific pains, ranked by time wasted:

1. **"What changed since I last looked?"** — the eyeballing problem. (Cell-level diff solves this.)
2. **"Why did this number change?"** — the cascade problem. (Authored-vs-computed + dependency graph solves this. **This is our differentiator.**)
3. **"Which version is canonical / who signed off?"** — the provenance problem. (Commit history + attribution solves this.)
4. **"Two people edited in parallel — reconcile them."** — the merge problem. (2D structural diff sets this up; hardest, long-term moat.)

## 3. What the product does

Turn the pile of emailed `_FINAL_v7` files into a **reviewable history where every change has an author, a cause, and a blast radius.**

Core capabilities, in priority order:

- **Structural-aware cell diff** — compares two workbook versions and reports what changed at the cell level, correctly handling inserted/deleted rows and columns (see Risk #2 below — naive positional diff is the classic failure).
- **Authored vs. computed classification** — for every changed cell, decide: did a human edit the formula/input (authored), or did the value move because an upstream dependency changed (computed)? Present authored changes as causes; let the user expand into the cascade.
- **Cascade / blast-radius view** — "this one WACC assumption changed → here are the 340 downstream cells it rippled into," colored by magnitude. **Nobody in the competitive set does this. It is the reason to build.**
- **Commit-style version history** with per-cell history over time and per-commit attribution ("whoever committed version N owns the delta from N-1").
- **Sign-off workflow** — reviewers can approve a version; sign-off rides on commit metadata (maps to the social sign-off ritual finance already has).
- **GitHub-Desktop-style desktop app** — repo/workbook list, history, diff pane; see §7.

## 4. Competitive landscape (know this cold)

The idea is **not original** — but that validates the market rather than closing it. Three tiers:

**Tier 1 — Pure diff tools (no versioning):** Microsoft Spreadsheet Compare (via Inquire add-in; Windows-only, Office Pro Plus / M365 enterprise only), xlCompare, Synkronizer, DiffEngineX, SheetCompare. These are point-in-time two-file comparisons — no history, no team layer, no cross-time attribution. They are the *manual step we replace*, not a real competitor.

**Tier 2 — The direct competitor: xltrail (by Zoomer Analytics).** This is substantially our idea, shipping today. Web-based, GitHub-like version history for Excel; red/green diffs across cells and VBA; audit trail of who/what/when/why; per-sheet and per-VBA-module history; cloud + on-prem/self-hosted; integrates with Git providers (GitHub, Bitbucket, GitLab, Azure DevOps). Explicitly markets to the financial-modeling industry.
  - **They are active, not sleepy.** (The public testimonials page is stale ~2018, but that's "found PMF, stopped tending the marketing wall," not abandonment.) Parent company Zoomer Analytics also ships **xlwings** — a popular Python-Excel library with frequent 2026 releases — which gives them an adjacent audience of technical-finance users feeding xltrail. Treat them as a focused, technically strong incumbent with a distribution advantage.
  - **⚠️ CORRECTION — what they already have (verified from their own docs).** An earlier draft of this doc claimed a "no-Git on-ramp for finance" as a differentiator. **That was wrong and has been removed.** xltrail's docs confirm a **drag-and-drop web flow explicitly for users with no Git knowledge** — you drop the `.xlsx` on a web page, type a change message, and get version history; Git is fully hidden behind the server. They *also* offer a separate Git-provider integration for technical users, but the finance-facing path already abstracts Git away. They also already have: **branching** (create/switch/commit branches — so "scenario forks" is not novel to them), **share-a-cell-range URLs**, **workbook vs. component history**, **malware scanning**, **password-protected workbook support**, and a **mature self-hosted Helm/Kubernetes deployment**. Treat drag-and-drop onboarding, self-host, and branching as **table stakes we must also meet — not differentiators.**

**Tier 3 — Excel-native FP&A platforms:** Datarails and similar (Series C funded). Heavier — centralized financial data, consolidation, dashboards, approvals, audit trails. Different, bigger category. Not a direct competitor now, but they own the enterprise "Excel governance" budget we'd eventually bump into.

### Where the opening is (the ONE real differentiator)

After verifying xltrail's actual capabilities, the honest picture is **one** load-bearing differentiator, not two — but it's the hard-to-copy one:

1. **The cascade / blast-radius view — unclaimed.** xltrail's comparison view shows a **flat list of changed cells** (value/formula/VBA diffs) — it tells you *that* cells changed. Nothing in their public docs describes **authored-vs-computed classification or dependency-graph cascades** — i.e. "this one input change rippled into these 340 downstream cells and moved your IRR from 24.1% to 19.8%." That distinction requires the dependency-graph engine that is the hard part of our build, which is exactly why it's defensible: it's real engineering they'd have to add, not a UI reskin. **This is the wedge. Everything else is parity.**
2. **Ambient capture vs. manual upload — verified.** Their *only* ingestion paths are a manual browser upload (five actions per version, opt-in, gaps when forgotten) or a Git remote (assumes the team already runs Git). There is no file-sync integration of any kind. A watched-folder daemon — hit Ctrl+S in Excel, a version exists — is a fundamentally different interaction model, not a marginal improvement. **Ambient vs. ritual.** This is now evidence-backed, not inferred.
3. **A genuinely snappy native desktop experience (softer, UX-only edge).** Their finance flow is web drag-and-drop; a GitHub-Desktop-style *native app* with a watched folder can feel faster and more integrated. Real UX preference difference, **not** a capability gap — do not over-claim. Caution: UI polish is the *cheapest* thing for an incumbent to fix. Never lead with it.

> **Strategic discipline:** "Same thing but newer" loses — a strong incumbent out-executes you on their home turf. Do **not** pitch "no-Git for finance" (they have it) or self-host/branching (they have it). Compete on the one thing their public product doesn't do: **blast-radius / cascade review, grounded in the formula dependency graph, with AI narration on top.** Lead every pitch with the cascade; treat all version-history features as table stakes to reach parity on, not reasons to switch.

### Positioning line

> *"xltrail shows you what changed. We show you what it broke — the blast radius of every change through your model's formulas, narrated in plain English."*

### ✅ Verified in the trial (July 2026 — hands-on, not inference)

Confirmed by using the product and reading their integrations page:

- **Ingestion is manual upload or Git — nothing else.** Their integrations page (`xltrail.com/integrations`) is titled "Git Integration" and lists exactly one category: Git providers (GitHub, BitBucket, GitLab, Azure DevOps). **No SharePoint, no OneDrive, no file-sync of any kind.** For a finance team on SharePoint, the only path is the manual browser upload. This is structural to their ingestion model, not a UI shortcoming.
- **The upload ritual is five deliberate actions per version:** open browser → navigate to repo → Add → drag file → type message → Commit. It is *opt-in*, so a forgotten upload leaves a hole in the history — and a version-control tool with gaps can't be trusted.
- **Their diff surfaces structure, not causality.** The comparison view reports counts like "12 rows added, 7 columns added / Added Columns A-G / Added Rows 1-12" and displays raw formula strings as changed text. Nothing resembling authored-vs-computed classification or dependency traversal.
- **They store cross-sheet references** (`='P&L'!B6`, `=Assumptions!B5`, `=Debt!B4` are visible in their cell view) — so they have the raw material for a dependency graph and haven't built one.
- **Presentation is a repository browser** (workbook → sheet → cells), not a review tool oriented around "what changed and why."

### ⚠️ Still to verify (one run, five minutes)
The trial so far uploaded workbooks as **separate files**, so their true *version-to-version* diff hasn't been seen yet. To test: upload a changed workbook **under the same filename** (e.g. rename `atlas_c06_exit_multiple.xlsx` → `atlas_c01_initial.xlsx` and commit). Then confirm: when an input feeds formulas, does their diff distinguish the *input* change from the *downstream recalculations*, or list all changed cells flat? Everything observed suggests flat — but confirm before building the pitch on it. (An earlier draft of this doc asserted a "no-Git" gap from inference and was wrong; don't repeat that pattern.)

## 5. Market & GTM

- **Beachhead: PE / IB.** Narrow beats broad for a wedge. High-stakes models, a formal review/sign-off ritual to design against, and errors expensive enough to justify paying. Gives a concrete reference-customer profile.
- **Expansion: any finance team doing Excel back-and-forth** — FP&A, accounting/audit firms, insurance/actuarial, lenders reviewing borrower models, consultancies. Real, but dilutes the pitch if led with. Widen *after* a PE reference.
- **Source-of-truth / deployment tiers** (see §8): SaaS cloud first; SharePoint-sync adapter second (adoption unlock); self-hosted/hybrid third. **Note: self-host is table stakes, not an edge** — xltrail already ships a mature Helm/K8s on-prem option, and finance firms that "don't trust the cloud" will expect it from us too. Plan for it, don't market it as differentiation.
- **The validation question that actually decides viability** (a customer-conversation question, not a research one): *Given xltrail already does drag-and-drop version history for finance, will teams switch/pay for the cascade + native-app experience specifically — or is xltrail (or "good enough" SharePoint) already enough?* **Five conversations with real PE associates / FP&A analysts will tell you more than any further competitor analysis.** Do this before committing months.

## 6. Architecture overview — three parts

The product is **three distinct workloads with different needs.** Do not pick one language reflexively for all three; understand each.

1. **The engine** — parse two `.xlsx`, align rows/cols, build the dependency graph, classify authored-vs-computed, compute cascade. Algorithm-heavy, correctness-critical. *This is the whole company; prove it in isolation first, before any UI.*
2. **The daemon / server** — auth, storage, SharePoint sync, file-watching, serving the desktop app. I/O-bound, concurrency-friendly, boring on purpose.
3. **The desktop shell** — window, sidebar, history list, virtualized diff grid. Rendering-bound.

### How a version flows in

```
1 Edit      Analyst edits model in Excel, saves normally
2 Capture   Daemon sees the save (watched folder)  OR  user clicks "Commit version"
3 Parse+diff Engine aligns rows, classifies authored vs computed, builds cascade
4 Store     Commit + blobs + diff land in canonical history (cloud or self-host)
5 Review    Any permitted team opens desktop app, sees diff, comments, signs off
```

None of this requires hooking into Excel's UI. The watched-folder save (or explicit commit) is the integration point. See the topology wireframe (`wireframes/wireframe-arch.html`).

## 7. Desktop UX — model directly on GitHub Desktop

Users should feel fluent on first open. Borrow GitHub Desktop's structure faithfully; diverge only where a spreadsheet forces it. (See `wireframes/wireframe-main.html`.)

GitHub Desktop anatomy → our mapping:

- **Repo dropdown → Workbook/Model dropdown.**
- **Branch dropdown → Scenario/Fork** (base / upside / modeling-return). Finance teams fork models constantly; this maps perfectly.
- **Primary action button → "Commit version" / "Push to team" / "Pull latest."**
- **Left sidebar "Changes" tab → uncommitted cell changes**, grouped by sheet (e.g. `Sheet1!B12`), commit box pinned at bottom.
- **Left sidebar "History" tab → version history**, each row = author, summary, when, N cells changed, sign-off badge.
- **Right diff pane → virtualized spreadsheet grid** (the one place the metaphor must diverge — see below).

**Where to put the finance "bells and whistles"** — overload GitHub's existing slots, don't invent new chrome:

- **Cascade toggle** (Authored-only ↔ Show-cascade) lives in the diff-pane header where GitHub puts the file path. This is the single most differentiating control; give it the prominent spot.
- **Sign-off** = a badge on commit rows in History (it's just commit metadata).
- **Authored/computed counts** ("1 authored · 4 computed") in the commit-row subtitle where GitHub shows file-change counts.
- **Changed-sheets list** in a dedicated middle pane (GitHub's "10 changed files" → "2 changed sheets"), each row showing the sheet name and its change count. **Superseded an earlier "sheet tabs above the grid" idea** — the three-pane layout verified from real GitHub Desktop screenshots is a better fit. See `UI-SPEC.md`.
- **Per-cell history / attribution** = hover/click card, not always-on chrome, so the grid stays readable.

**The one deliberate divergence from GitHub:** GitHub's diff is a 1D list of lines and can stack removed-above-added. A spreadsheet cell is *spatial* — one position, two values (before/after). So render: new value in green, old value struck-through beneath (or in a side panel), plus author/timestamp/authored-vs-computed on hover. An `ƒ` marker distinguishes formula-driven (computed) cells from hand-typed inputs. **Do not force stacked red/green rows onto a 2D grid** — it fights the spatial nature of a sheet. Everything *around* the diff (chrome, sidebar, history, commit flow) copies GitHub faithfully.

## 8. Source of truth & SharePoint — the key product insight

Everyone in an org can "see each other's sheets and full history" only if there is **one place** holding the commits. That's non-negotiable.

- **Org onboarding — the Slack model, bottom-up.** Sign in with company SSO (Microsoft Entra is the natural first target: finance teams are already a Microsoft shop, and it's the same identity that later grants Graph API access for SharePoint). **Domain-based auto-join** — anyone with an `@company.com` address lands in the right org automatically, exactly like Slack. Invite links for edge cases (contractors, cross-firm deal teams).
  - **Critical: one person must get full value before any of that exists.** Install → point at a folder → see diffs. No account, no admin, no IT ticket. "Join your team" is then an *upgrade they want*, not a *gate they must clear*. This is how Slack and Notion actually spread inside companies, and it's the opposite of an enterprise-sale-first motion — which matters because an admin-first rollout means a security review before a single user gets value.
  - **Permissions are workbook-scoped, not org-scoped.** "I'm sharing Atlas LBO with you," not "you have access to Deal Team A's files." A deal team's model should not be firm-visible by default.
- **Cross-team visibility is a permission on a workbook, not a copy.** Deal Team owns `Atlas LBO`; Modeling gets read+comment; IC gets read at sign-off checkpoints. One workbook, many viewers — the *only* way "everyone sees the same history" is actually true. The moment you have copies, you're back to the `_v7_FINAL` problem.
- **Canonical history lives in your cloud tenant** (or the same server self-hosted in their VPC for the compliance-anxious — deferred).

**The mental model that resolves "can it merge with our SharePoint?":**
> **SharePoint is a filing cabinet; you are the version-control brain.** You don't merge two systems that both want to own the file. The daemon watches their existing SharePoint/OneDrive *synced folder* and ingests each saved version *as a commit* into your canonical history. SharePoint stays the file store; you own the history/diff/cascade layer. SharePoint sync is one **ingestion source** (the other being the explicit "Commit version" button) — **not** an alternative product to the SaaS server.

**What if the company doesn't use SharePoint?** Nothing breaks. The server is *always* the canonical history; SharePoint is one optional front door. A team without it uses the other paths — watch any local folder, or commit explicitly — and history, diffs, cascades, attribution, and sharing all work identically. **No part of the product depends on Microsoft.** Ingestion sources are pluggable: SharePoint/OneDrive sync folder, any watched local folder, explicit commit, drag-and-drop, and later Dropbox/Google Drive/Box sync folders (all use the same local-folder mechanism).

> **Do NOT build push/pull of workbook files over a synced folder.** If SharePoint is already syncing the file and your daemon also pushes/pulls versions of it, two systems have competing opinions about the current state — a conflict surface you didn't create and can't fix. The sync client moves *files* (Microsoft's job, they're good at it). You record *history*. What syncs through your system is commits, diffs, attribution, and cascades — never the workbook bytes. A GitHub-Desktop-style sync button is still right, but it pulls **history** so you see teammates' commits, not workbooks.

> Framing SaaS and SharePoint as either/or leads to two half-products. Frame the SaaS server as *always* the brain, and SharePoint sync as one of several front doors into it.

Deployment tiers, in build order:
1. **SaaS cloud** — source of truth; simplest; cross-team history "just works" (one DB); SSO/SAML. Objection to expect: "our models can't leave our walls."
2. **SharePoint sync adapter** — meets teams where files already are; zero behavior change; easiest adoption.
3. **Self-hosted / hybrid** — answers the compliance objection; Kamal + their infra; heavier sales/support; defer until a real deal requires it.

## 9. Language & stack decision — full tradeoff

**The user's real constraint:** the practical choice is **Go + Wails** *or* **TypeScript + Electron**. Rust + Tauri is included below for completeness and honesty, but it is not a path the user intends to take. The LLM writes the code, so "which is easier for a human to learn" is not the deciding axis; "which produces a reliable, snappy, shippable product with the fewest seams" is.

### Recommendation
**Go for all three core layers (engine + daemon + Wails desktop).** Optionally a Python side-worker *later* for phase-3 quant/simulation features, wired out-of-process (the `claude -p`-style pattern). **No gRPC/Python service needed to ship the core.**

> **DECIDED — and the strongest reasons are not the performance ones.** Two arguments settle this beyond the analysis below:
> 1. **Single static binary for self-host.** The install story is "IT drops one Go binary on a server and it just works" — no runtime, no dependency tree, no container orchestration required. (Contrast: xltrail's on-prem ships as a Helm/Kubernetes chart. Self-host is table stakes we must match — but a *simpler* install is a legitimate selling point to a small IT team.)
> 2. **Many external integrations ahead.** SharePoint/Graph API, OneDrive, SSO/SAML, Slack, notification hooks, Git providers. That's a lot of HTTP clients, OAuth flows, polling, and concurrent I/O — Go's core strength and the least interesting thing to argue about. Every one of these is a well-supported, boring Go library away.
>
> **Caution on a Rails cloud backend:** pairing a Go daemon/desktop with a Rails SaaS server reintroduces exactly the language seam this section argues against — two codebases, two sets of models, a serialization contract to keep in sync. It's workable (desktop talks HTTP to Rails), but since the engine must be Go anyway and Go serves HTTP well, keep the server Go unless Rails buys something specific. Familiarity is not worth re-adding the seam.

### Why — reasoned, not by habit

**Performance is a red herring at this data scale.** A large PE model is ~50–200k populated cells. A deliberately oversized **240k-cell, 30-sheet model is a 1.4 MB file.** Measured on that file *in Python* (the slowest candidate):
- Full XML parse of every cell: ~1.4 s (Go ~150–300 ms; Rust ~100–200 ms — this is I/O/byte-shuffling, not compute)
- Full diff of 240k cells: ~144 ms
- Dependency graph over ~108k formula cells: ~349 ms
- Cascade BFS from a 500-cell changeset: ~33 ms

Everything except parsing is *already sub-350 ms in Python*; in Go it's tens of ms. Rust shaves milliseconds off operations already below human-perceptible thresholds — and those actions are gated by network/disk round-trips that dwarf the difference. **You will not be CPU-bound. Rust buys performance you cannot feel here.**

**The Python "finance owns it" instinct is a trap for *this* product.** Python's `xlcalculator`/`formulas` lead is in formula *evaluation* (re-implementing Excel's function library). **We never evaluate formulas** — the `.xlsx` already contains Excel's cached computed values, so the diff compares stored values. We only need to *parse* formulas (to build the dependency graph + classify authored-vs-computed). Parsing is the easy half, and **Go has it via `efp` (Excel formula tokenizer/AST, same author as `excelize`).** Python's advantage collapses. It only re-earns a seat for *actual recomputation* — what-if re-simulation, Monte Carlo, anomaly detection — which is a phase-3 module, not the foundation.

**Desktop grid perf is a virtualization question, not a shell question.** Electron, Tauri, and Wails all render UI in a webview. A 50k-cell sheet feels identical across them *if you virtualize* (react-window / TanStack Virtual — render only ~200 visible cells) and janks in all of them if you don't. The shell's language does not touch the render loop.

**The decisive Wails property: zero language seam.** Wails = Go backend + web frontend, so engine, daemon, and desktop app are **one Go codebase sharing types** — no FFI, no IPC schema drift. Tauri forces a Go-engine↔Rust-shell seam (or a full Rust rewrite of the engine). Electron forces a Go-engine↔Node-shell seam (or a TS engine). Wails is the only option with no seam between the three parts.

**Reliability of LLM-generated code (an honest axis, since the LLM writes it):** Go's mistakes tend to be logic bugs that surface at runtime with clear stack traces. Non-trivial Rust mistakes are often borrow-checker/lifetime errors — worst in **async Rust**, which is exactly what the daemon needs (concurrent file-watch + HTTP + DB + SharePoint poll). Go's goroutine model is generated correctly far more consistently. Go's Excel libraries (`excelize` + `efp`) are more widely used and better-documented than Rust's (`calamine` read, `umya-spreadsheet` read-write), so first-draft correctness is higher in Go.

### The Go + Wails vs TS + Electron call (the user's real fork)

| Axis | Go + Wails | TS + Electron |
|---|---|---|
| Engine libraries | `excelize` + `efp` (mature, formula AST) | `exceljs` (richest single lib) but **no dependency graph for free**; you build the formula-graph either way |
| Language seam | **None** — engine/daemon/app all Go | Engine in TS too (one language) *or* Go-engine↔Node seam |
| Daemon/concurrency | Go's core strength; single static binary | Node works; heavier runtime, bundled Chromium |
| Desktop footprint | Lighter (OS webview, small binary) | Heavier (bundled Chromium, higher idle memory) |
| Packaging/updater | `wails build` → native bundles; auto-update less turnkey (wire a lib) | Very mature ecosystem (electron-builder, electron-updater) |
| Grid perf | Same (virtualization decides it) | Same (virtualization decides it) |
| Frontend familiarity | React + TanStack (known) | React + TanStack (known) |

- If you go **TS/Electron**, the clean move is a **single-language TS product** (engine in TS with `exceljs` + hand-built formula-graph, Electron shell) to avoid a seam — trading Go's daemon/footprint strengths for Electron's packaging maturity and one JS toolchain.
- If you go **Go/Wails**, you get the no-seam property, lighter footprint, and the daemon strengths, at the cost of a less turnkey auto-updater (a phase-2 concern).

**Rust + Tauri (for completeness, not a chosen path):** genuinely better only if you later parse *live keystroke-rate edits* and recompute full models in the hot path, or handle files 50–100× this size, or do heavy numerics inline. None are current requirements. If they ever become central, drop a Rust (or Python) worker behind a clean interface without rewriting the rest.

### Packaging & deploy (both viable paths)
- **Server:** single static binary; Go cross-compile (`GOOS`/`GOARCH`) or Node build; **Kamal → Hetzner** (user's existing workflow).
- **Desktop:** the hard part is **code-signing + notarization** (Apple notarization, Windows Authenticode) — OS-specific and *identical annoyance regardless of language*. You realistically build the Mac app on macOS and the Windows app on Windows (or per-OS CI runners) in *any* stack. Tauri/Electron have slightly more mature bundlers/updaters than Wails, but that gap is (a) partly neutralized by signing being language-agnostic and (b) a phase-2 concern (matters when you have users to update).

## 10. Data model (not yet specced — next artifact)

Deliberately **not** specced yet. The next foundational artifact is the contract everything binds to:
- **Engine output JSON:** per-cell diff records (coord, old/new value, old/new formula, authored|computed classification), dependency edges, cascade sets.
- **Postgres schema:** commits, content-addressed blobs (raw workbook + parsed representation), computed diffs, and org/team/workbook/permission tables that make the topology in §8 real.
- **Storage model:** don't literally use Git (byte-diffing zipped XML is useless), but steal its content model — immutable commits, content-addressed blobs, a history DAG, branches/forks. Postgres holds raw workbook per commit + parsed representation + computed diff; content-hash the parsed representation for dedup. Sets up the eventual **merge** feature (the long-term moat).

## 11. AI-assisted layer

**Core principle: the AI reasons *over* the deterministic diff; it never becomes the source of truth.** The engine (§6) produces a structured, grounded representation — dependency graph, authored-vs-computed classification, per-change diff. That structure is exactly what LLMs are unreliable at extracting themselves but excellent at reasoning over once handed to them. **The moat is not the LLM — it's the deterministic work that makes the LLM reliable instead of hallucinatory.** Every AI feature below is grounded on the engine's output, so it explains and triages; it does not compute or judge.

> **The hard guardrail — AI narrates and flags, never judges.** Finance users will not (and should not) trust an LLM asserting that a valuation is correct or making a financial judgment. Keep the AI on *explaining* and *surfacing*, grounded in the deterministic diff, and off *evaluating whether the numbers are right*. The credibility of the entire product rests on the engine being the source of truth and the AI being a narration/triage layer on top. Invert that and finance users bounce instantly.

### Why this strengthens the product (and the investor story)
The AI layer is where "quality of life and flow" genuinely lives, and it's featureful *without diluting the wedge* — because it all hangs off the same grounded structure rather than adding new surface area. It compounds on the core instead of bloating around it. This is the right place to put scale-ambition; the core diff is not (the core must be ruthlessly focused and correct).

### Features, ordered by value × defensibility

1. **Plain-English change narration (build first).** Engine knows "B10 exit multiple 10.5x→9.5x, authored, rippled to 340 cells incl. IRR 24.1%→19.8%." LLM turns that into: *"Exit multiple lowered 10.5x→9.5x, dropping projected IRR ~4.3pts and MOIC 2.6x→2.2x."* Auto-generated commit summaries and review notes, **grounded in the diff so it can't fabricate.** High-value, low-risk, demos beautifully.
2. **Anomaly / "does this smell wrong" flagging (most differentiated).** Because the cascade + history exist, an LLM (or sometimes plain heuristics) surfaces: "a hardcoded number replaced a formula in a cell that used to compute — often a broken calc," or "this input moved 10× beyond its historical range." Finance reviewers live in fear of exactly these silent errors. Framed right: *"an AI reviewer that catches the mistakes humans miss when eyeballing."* A painkiller, not a vitamin.
3. **Conversational query over model history (nice-to-have).** "When did WACC last change and who changed it?" Rides on the same grounded structure. A feature, not a wedge.

### Explicitly out of scope for the AI layer
Anything where the LLM evaluates whether numbers are *correct* or makes financial judgments. That's the one thing that destroys trust. The deterministic engine owns truth; the AI owns explanation and triage.

### Sequencing (critical)
**This is a Phase 3+ layer, not a Phase 0 distraction.** The AI is worthless until the dependency graph underneath it is correct — narration and anomaly detection are only as good as the grounded structure they read. The trap is building the fun AI narration before the boring dependency graph works. **Deterministic engine first; AI as the layer that makes it feel magic later.** Wiring follows the out-of-process pattern (an LLM worker the Go daemon calls), so it slots in without disturbing the core.

## 12. Open risks — verify before betting

1. **[LOAD-BEARING — VERIFY FIRST] Do `excelize` + `efp` actually expose enough to build the cascade?** The entire "Go is sufficient" conclusion rests on this single unverified assumption. Specifically, can we read, richly enough: **shared formulas** (Excel stores one master formula shared across a range — must expand correctly), **cross-sheet references** (`Sheet2!C4`), **named ranges** (`=WACC` resolving to a cell), and **cached values** (the stored computed result per cell, which is what lets us diff without recomputing)? **Action: write a small Go probe against a real formula-heavy PE-style workbook and confirm all four are accessible before committing to the Go engine.** If a gap exists, it's a Go-specific problem worth knowing *now* — it could force a Python parse-worker or change the language call.
2. **Structural alignment (2D diff) is the make-or-break algorithm.** Naive `(row,col)→value` comparison reports the whole sheet as changed the moment a row is inserted → product looks broken. Need row/column alignment (keyed LCS/Myers: match on a stable key — header/ID column or row fingerprint hash — then diff aligned rows; positional fallback only when no key exists). Spend the first weeks here and nowhere else. This is a from-scratch algorithm in any language.
3. **Cascade product decision:** one input change → hundreds of downstream recomputes. Default to showing **authored** changes as causes, let users expand the cascade (colored by magnitude). Don't dump the full computed set by default. (Compact test model: 4 computed. A real PE model: hundreds.)
4. **Attribution granularity:** `.xlsx` carries no reliable per-cell authorship. Commit-level attribution (committer of N owns the N-1→N delta) is honest, cheap, and ~90% of the value. True live per-cell authorship needs real-time edit capture via an Office add-in — a much bigger commitment; defer.
5. **In-Excel colored overlays are NOT the day-one integration.** Rendering red/green *inside the live Excel grid* means an Office Add-in (JS API, sandboxed, can set cell fills/comments but limited UI; Windows/Office-version fragile) or VSTO/COM (Windows-only, heavy). A true non-destructive floating overlay basically doesn't exist cleanly. **Build the diff in our own desktop app first** (cross-platform, Office-version-independent); treat the Excel-native overlay as an optional phase-3 layer on the same pipeline.
6. **Competitive:** a focused incumbent (xltrail) with an adjacent hit (xlwings) is a *harder* setup than an empty market — and they already have more than an early draft of this doc assumed (drag-and-drop no-Git onboarding, branching, self-host). Response is discipline, not speed: win the **one** wedge they don't have (cascade / blast-radius + AI narration); reach parity on the rest; don't rebuild the 80% they already do well.

## 13. Build phases

**Phase 0 — De-risk the engine (do this before anything else).**
- Verify Risk #1 (`excelize`+`efp` probe).
- Build the diff engine as a pure library (no UI, no server): two `.xlsx` in → structured diff JSON out.
- Nail structural alignment (Risk #2) on real messy models. Success test: insert a row + change 3 inputs → engine reports "3 authored + N computed," not "800 changed."
- Implement authored-vs-computed classification + dependency graph + cascade BFS.

**Phase 1 — Single-user desktop app over a local server.**
- Wails (or Electron) shell modeled on GitHub Desktop (§7).
- Virtualized diff grid with red/green + `ƒ` marker + hover attribution.
- Cascade toggle. Explicit "Commit version" flow. Local Postgres, commit/blob/diff model (§10).
- Success test: the diff pane feels *better* than opening two Excel windows side by side.

**Phase 2 — Teams & cloud.**
- SSO/SAML org sign-on; org→team→workbook hierarchy; workbook-level permissions (§8).
- SaaS cloud as canonical history. Sign-off workflow. Per-cell history over time.

**Phase 3 — Adoption & depth.**
- SharePoint/OneDrive watched-folder ingestion adapter.
- Self-hosted/hybrid deployment (when a deal requires it).
- **AI-assisted layer (§11)** — grounded on the diff: plain-English narration first, then anomaly flagging, then conversational history query. LLM worker called out-of-process by the daemon. *Only after the deterministic engine is correct — the AI is worthless before then.*
- *Then* consider: Office add-in overlays, live edit capture, merge/conflict resolution, Python quant/simulation worker (what-if re-simulation).

## 14. Immediate next steps

1. **Verify Risk #1** — the `excelize`+`efp` probe against a real formula-heavy workbook. Everything downstream depends on it.
2. **Spec the data model** (§10) — the engine-output JSON + Postgres schema. The contract the engine, server, and app all bind to.
3. **Prototype structural alignment** (Risk #2) — the hardest algorithm; prove it on messy real models early.
4. **Run 5 customer-validation conversations** (§5) — given xltrail already does drag-and-drop version history for finance, will teams switch/pay for the cascade + native-app experience specifically, or is xltrail / "good enough" SharePoint already enough?

---

### Appendix — wireframes in this package
- `wireframes/wireframe-main.html` — main window (GitHub Desktop structure + finance additions annotated in orange). PNG: `wireframes/shot-main.png`.
- `wireframes/wireframe-arch.html` — topology: org→team→workbook hierarchy, source-of-truth options, version-flow pipeline. PNG: `wireframes/shot-arch.png`.
