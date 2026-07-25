# ROADMAP — self-hosted Argus

> **Status:** decided 2026-07-25. This document is the current plan for how Argus is
> deployed and how the pieces connect.
>
> **It supersedes `PROJECT.md` §5, §8, and §13 wherever they conflict** — specifically the
> deployment tier ordering (SaaS → SharePoint → self-host) and the phase plan that
> defers self-hosting to Phase 3. Everything else in `PROJECT.md` still holds: the
> cascade is still the one differentiator, the engine is still the source of truth, the
> AI still narrates and never judges.
>
> `PROJECT.md` and the rest of the context package live in `~/Downloads/argus-files/`
> and are **not** version controlled. This file is written to stand alone.

---

## 1. The decision

**Self-host first. SaaS deferred, possibly indefinitely.**

`PROJECT.md` §8 ranked deployment as SaaS cloud (1) → SharePoint adapter (2) →
self-hosted (3), with self-host "deferred until a real deal requires it," and §5 called
self-host "table stakes, not an edge." That ordering is inverted here.

**Why:**

- The objection §8 itself predicts for SaaS — *"our models can't leave our walls"* — is the
  **first** thing a finance firm says, not the third. Shipping self-host first deletes it
  rather than deferring it.
- Cost asymmetry. SaaS-first means running infrastructure, answering security
  questionnaires, handling data residency, on-call, and billing *before deal one*.
  Self-host-first means shipping a binary and a signed license file.
- It deletes real work from the critical path: multi-tenancy, tenant isolation,
  domain-based auto-join, signup flows, and SSO all exist to route strangers to the
  correct tenant. **A self-hosted server has exactly one tenant by construction.**
- `PROJECT.md` §9 already named "single static binary for self-host" as one of the **two
  decisive reasons Go was chosen** over TypeScript. The stack finally matches the plan.
- Install-story edge: xltrail's on-prem ships as a Helm/Kubernetes chart. "Copy one binary
  onto a box and run it" is materially better for a three-person IT team. This is not the
  wedge — the cascade is still the wedge — but it shortens the buying process.

**What it costs, on the record:**

- No telemetry. Flying blind on usage exactly when signal matters most.
- Update distribution becomes the customer's problem; client↔server version skew is
  permanent and must be designed for (see §6.6).
- Support debugging without access to the instance.
- Offline licensing required if this is ever commercial.
- **Argus becomes a file store.** "We watch your folder" is a small promise. "We hold your
  models and hand them back" means backup, retention, and disk growth are now our problem.

---

## 2. Architecture: one binary, two modes

`argusd` is built as a **library first, daemon second**. The same code runs both ways:

- **Mode A — solo, zero config.** The Wails app starts `argusd` in-process. SQLite under
  the OS app-support dir. Watches a local folder. No server, no account, no IT ticket.
  This preserves `PROJECT.md` §8's bottom-up requirement: *"one person must get full value
  before any of that exists."*
- **Mode B — team, self-hosted.** IT runs the same binary as a service against a shared
  store and the firm's shared folder. Clients get a base URL.

**The client cannot tell the difference.** It speaks HTTP to a base URL that defaults to an
in-process loopback listener.

### The seam — change this early

`desktop/app.go` binds `App.Diff(pathA, pathB) → DiffResult`, and the comment block at the
top of `desktop/frontend/src/data/diffs.ts` declares that in-process Go call "THE ENGINE
INTEGRATION POINT." **That is the wrong seam.** It should be an HTTP API
(`GET /api/commits`, `GET /api/diff/{from}/{to}`), with the Wails binding as a thin HTTP
client.

Convenient timing: commit `dd91ca9` deleted `loadDiff.ts` and `fixture.json`, so `App.Diff`
currently has **no caller at all** — the frontend renders entirely from bundled JSON in
`data/diffs.ts`. There is nothing to unwind; the HTTP seam can go in clean.

This is a small change now and a rewrite later. Do it before wiring more UI, so the
frontend is built against the real boundary.

### Topology

```
The firm's existing file world — Argus READS, never writes
  \\fileserver\Deals\Atlas\Atlas_LBO.xlsx     (SMB / SharePoint / OneDrive / Dropbox / none)
        │
        │  read-only polling scan  (NOT fsnotify — see §5.3)
        ▼
  argusd — one binary on a box in their VPC    ← this IS the org
    ├── commit store: entry-addressed blobs, commits, diffs, cascades
    ├── ingester: save → commit, attributed via docProps.LastModifiedBy
    ├── HTTP API + SSE
    └── drops .argus/server.json into the share for client discovery
        ▲
        │  HTTPS: history, diffs, cascades, sign-offs. Blobs on request.
        │
  Wails clients on analysts' laptops
```

---

## 3. Storage independence — no SharePoint dependency

`PROJECT.md` §8 already states *"No part of the product depends on Microsoft"* and lists
ingestion sources as pluggable. But it **assumes a sync fabric already exists** — every
source it names is "a folder something else keeps in sync." A firm with no SharePoint, no
OneDrive, and no Dropbox has nothing moving files between people. That case is unaddressed
in the original docs and is handled here.

Three supported shapes, in priority order:

1. **Firm has a network share** (`S:\Deals\` — very common at mid-market PE firms, no
   SharePoint required). `argusd` polls it read-only. Files already reach everyone.
2. **Firm has SharePoint/OneDrive/Dropbox.** Same as above, or Graph API delta queries for
   SharePoint (see §5.3 — do *not* run a sync client on the server).
3. **Firm has no shared storage at all.** Argus provides the "magic folder" itself — see §4.

### Invariant: Argus is read-only on shared folders

Straight from `PROJECT.md` §8's warning about two systems with competing opinions about
current state. Argus reads saves and records history. **It never writes a workbook back to
a watched folder.** `.argus/server.json` is the single exception — one file, written once.

This one rule eliminates the entire sync-conflict surface.

---

## 4. The magic folder (for firms with no shared storage)

Target the **Dropbox UX**, not the Dropbox implementation. The distinction matters because
the commit store *already* holds workbook bytes (`PROJECT.md` §10: "raw workbook per
commit + parsed representation + computed diff"). Serving a file is one endpoint over data
already stored — not a sync engine.

The v1 loop:

- Client has a local `~/Argus/<Workbook>/` folder.
- Pull → server streams the blob → file appears. *(one endpoint)*
- Analyst edits in Excel, saves → the Wails app is already running on that machine and
  watches the folder → uploads as a commit. *(fsnotify is correct here — local disk)*
- Other clients receive the commit over SSE and can pull.

### Concurrency policy — do NOT build merge

Two people edit offline and both save. Dropbox's answer is
`Model (Jack's conflicted copy).xlsx` — **literally the `_v7_FINAL` disease Argus exists to
cure.** Argus has a better answer available because it is a version control system:
divergence becomes two commits, a fork, and a diff showing exactly what moved, cascade
included.

For v1, optimistic concurrency is sufficient (~50 lines):

- Client commits with a declared parent (`parent: c07`).
- If the server's head has moved, **accept the commit as a fork** and flag divergence in
  the UI.
- Never attempt an automatic merge.

Optional and cheap: soft advisory locks ("Sarah has this open since 9:40"). Matches what
finance teams already expect from a shared drive — Excel itself does this with `~$` lock
files.

---

## 5. Verified findings

Everything in this section was tested on 2026-07-25. Probes were scratch harnesses under
the gitignored `probe/` directory. **Do not re-derive or contradict these without
re-testing.**

### 5.1 Attribution for ambient capture is solved and free — VERIFIED

`PROJECT.md` §12 Risk #4 concludes `.xlsx` carries no reliable authorship and settles for
"the committer of N owns the N−1→N delta." **But ambient capture has no committer** — the
server just observes a file change.

`excelize.GetDocProps().LastModifiedBy` returns the real human display name on a workbook
actually saved by a person in an Excel-family app. Verified against a real-world file:
returned `"Deivison de Oliveira"`.

It is **blank** on machine-generated files — the `atlas_*` test fixtures were written by
openpyxl and return `""`. So a fallback chain is required:

```
docProps.LastModifiedBy  →  SharePoint Graph lastModifiedBy  →  file owner  →  unattributed (claim in UI)
```

⚠️ **Not yet verified end-to-end:** the bundled fixtures cannot exercise this. Needs one
real Excel save to confirm.

### 5.2 Stale cached values fail SILENTLY — VERIFIED, and this is the sharpest edge

The engine never recomputes; it reads Excel's cached values. A workbook written by
something that did not recalculate produces a diff with the input change and **zero
downstream ripples** — an empty cascade. Nothing errors. It just looks like the product is
broken.

Reproduced by setting `Assumptions!B5 = 9.5` with a non-recalculating writer:

```
Assumptions!B5=9.5   Returns!B9=1928.49   B13=3.3645   B14=0.2746    ← all still v1 values
```

Expected (real `atlas_v2`): `B9=1744.83  B13=2.9935  B14=0.2452`.

**Required ingestion sanity check:** after parsing, verify that at least one dependent of
each changed input actually moved. If none did, flag the commit as suspect at capture time.

### 5.3 `CalcID` is NOT a reliable staleness signal — CORRECTION

An earlier analysis in conversation proposed `CalcID == 0` as a staleness guard. **That is
wrong.** The `atlas_c01–c07` chain all carry `calcID=0` yet have perfectly valid computed
values — which is why `desktop/frontend/src/data/diffs.ts` shows real ripple counts (40,
35, …). `CalcID` records which calc engine last stamped the file, not whether values are
fresh.

Use the dependency-graph check in §5.2 instead. It costs real work and is the only
reliable answer.

### 5.4 fsnotify does not work on network mounts — DESIGN CONSTRAINT

- Linux `inotify` is a no-op on NFS/CIFS mounts.
- macOS FSEvents does not fire for SMB volumes.
- Only Windows `ReadDirectoryChangesW` receives SMB2 change notifications.

**Therefore:** server-side ingestion must be a **polling scanner** (mtime + size → hash),
not fsnotify. fsnotify is correct *only* for Mode A's local folder and the client-side
watcher in §4.

Corollary: do not put a SharePoint-*synced* folder on the server — that would require
running the OneDrive client as a service account. Use Graph delta queries instead.

### 5.5 LibreOffice recalculation — VERIFIED, gives the daemon a repair path

Default headless `soffice --convert-to xlsx` **does not recalculate on load**; stale values
pass straight through. With a user profile setting `OOXMLRecalcMode=0`, it recalculates and
the output matches the ground-truth fixture to the last digit:

```
Assumptions!B5=9.5   Returns!B9=1744.8270912   B13=2.99346221454546   B14=0.24518751092498
```

This gives `argusd` a **repair path** for degraded inputs. Self-host makes it acceptable:
the server is a box IT controls, so a LibreOffice dependency there is fine — requiring it
on every analyst laptop would not be.

**Rule:** trust Excel's cached values when present. Only recalculate when §5.2's check
detects staleness. LibreOffice's calc engine is not Excel's and exotic functions can
differ.

⚠️ **Not yet verified:** the GUI path (open in Excel/LibreOffice, type a value, save).
Both auto-calculate as you type, so this *should* write fresh values, but it is the path
live usage actually depends on. **Test by hand before relying on it.**

### 5.6 Store zip entries, not whole files — VERIFIED, affects schema

`.xlsx` is a zip, so a one-cell edit rewrites the archive and **whole-file content
addressing dedups to exactly zero between commits.** Per-entry it is a different story.
Measured on `atlas_c01 → atlas_c02`:

```
14 entries | 10 identical | 4 changed (sheet1, sheet2, sheet4, docProps/core.xml)
61% of uncompressed bytes reusable at entry level
```

61% on a 4-sheet toy workbook. On a 30-sheet PE model where a commit touches two sheets,
expect 90%+.

**Therefore:** store a commit as a **manifest of content-addressed zip entries** — git's
tree model. This turns "every commit costs a full 50MB workbook" into "every commit costs
the sheets that changed." At 7 commits/day on a real model that is the difference between
tens of GB per year and a couple.

---

## 6. Build order

### 6.1 Commit store

- Content-addressed **zip entries** + a per-commit manifest (§5.6). Plus parsed
  representation and computed diff, per `PROJECT.md` §10.
- Commits table, refs/branches, `user_id` on every commit from day one (see §7).
- **SQLite via `modernc.org/sqlite` (pure Go). NOT `mattn/go-sqlite3` (CGO).** CGO breaks
  cross-compilation and the CGO-free static binary that is the entire premise of this plan.
  This dependency choice is load-bearing.
- Postgres as an optional driver later for large installs.

### 6.2 `argusd` HTTP API

Do this **before** wiring more UI, so the frontend binds to the real seam.

```
GET  /api/commits                     list, with authored/computed counts
GET  /api/diff/{from}/{to}            → DiffResult (engine/types.go, unchanged)
GET  /api/cell-history/{ref}          per-cell revision timeline
GET  /api/blob/{hash}                 materialize a workbook version
POST /api/commits                     client-side commit (declares parent, §4)
GET  /api/events                      SSE — new commits, ingestion flags
GET  /api/version                     API major/minor for the handshake (§6.6)
```

Use **SSE, not WebSocket** — one-way is all that's needed, ~20 lines in Go, and
`EventSource` reconnects automatically.

Frontend changes: `data/diffs.ts` and `data/history.ts` swap bundled JSON for `fetch`
(`loadDiff.ts` no longer exists — deleted in `dd91ca9`). Mostly relocating data that
already exists. Keep the `asDiff()` contract validation in `diffs.ts` — it becomes *more*
valuable against a live server than against bundled JSON.

### 6.3 Ingester

- Polling scanner over configured roots (§5.4), content-hash dedupe.
- Settle-delay for partial writes; **skip Excel's `~$` lock files**.
- Attribution chain (§5.1); staleness check (§5.2); optional LibreOffice repair (§5.5).
- Dedupe across sources: with both a server scanner and client watchers, the same logical
  save can arrive twice. Content-hash at commit time and drop exact-duplicate parents.

### 6.4 Identity, minimum viable

**Identity is required; authentication mostly is not.** Attribution *is* the product — a
commit reading "someone lowered the exit multiple" is worthless.

- Identity: OS username on explicit commits + `docProps.LastModifiedBy` for ambient
  captures. **No login screen anywhere.**
- Authentication: a join token printed at first boot, exchanged for a per-device key. That
  is proportionate for a server inside a firm's own network.
- **Skip SSO/SAML entirely for now.** It is a SaaS requirement — it exists to prove which
  tenant a stranger belongs to, and a self-hosted box has one tenant.

### 6.5 Discovery

- `argusd` drops `.argus/server.json` into the share: `{url, fingerprint, orgName}`.
- A client that can already see the share reads it and joins. No URL typing, no per-user IT
  ticket.
- mDNS on the LAN as a fallback.

### 6.6 Version handshake

Self-host makes client↔server skew permanent. Negotiate API major on connect; refuse a
mismatch with a clear "update your client" rather than misbehaving subtly.

### 6.7 Offline licensing (only if commercial)

Ed25519-signed license file carrying seats, expiry, and org. No phone-home.

---

## 7. Invariants — do not violate

1. **Never recompute a historical commit.** Commit `c06` must show what its author actually
   saw and signed off on. For an audit-trail product sold to finance, an audit trail that is
   silently rewritten is worse than none. If a commit's values were stale at capture, that
   is a *fact about that commit* — flag it (⚠ "values may not reflect formulas as of
   capture"), never repair it. Recomputation belongs at **ingestion**, never at view time.
2. **Never write a workbook back to a watched folder** (§3).
3. **Never sync workbook bytes as a transport over a folder something else already syncs**
   (`PROJECT.md` §8). Argus *archives* bytes in its own store; it is not a file-sync layer.
4. **Materialized historical files must not live in a watched path.** "Open in Excel" on
   commit `c06` extracts a blob to a temp file — if that directory is watched, the analyst
   edits the snapshot, saves, and Argus ingests a commit branching off ancient history.
   Name it unmistakably (`Atlas LBO @ c06 (read-only).xlsx`) and keep it out of watch roots.
5. **The engine stays pure and deterministic.** No AI imports, no network. The narrator is
   strictly post-processing (`PROJECT.md` §11).
6. **Permissions are workbook-scoped, not org-scoped** (`PROJECT.md` §8) — see §8 below for
   why this is not optional long-term.

---

## 8. Open questions and risks

| # | Item | Status |
|---|---|---|
| 1 | **GUI save recalculation** (§5.5) — does a real Excel/LibreOffice edit-and-save write fresh cached values? | **Untested. Test first — it fails silently and everything depends on it.** |
| 2 | **The GTM bet.** Is IT standing up a binary genuinely easier than a SaaS security review, or does self-host just relocate the enterprise sale? | Unvalidated. This is what `PROJECT.md` §5's five customer conversations should settle. |
| 3 | **Information barriers.** PE/IB firms have compliance walls — deal teams are legally not permitted to see each other's live deals. "Everyone who can reach the server reads every model" is something a compliance officer kills on sight. | v2, but it is what eventually drags real auth back in. Keep `user_id` on commits from day one. |
| 4 | **Structural alignment** (`PROJECT.md` §12 Risk #2) — positional diff still reports spurious changes on row inserts (`atlas_v4`). | Unchanged by this roadmap. Still the hardest algorithm and still deferred. |
| 5 | **Retention and disk growth** now that Argus is a file store. §5.6 helps a great deal; a retention policy is still needed. | Undesigned. |

---

## 9. Explicitly out of scope

Do not build these, in this order of temptation:

- **Merge / conflict resolution.** Fork and flag (§4). `PROJECT.md` calls merge the
  long-term moat; it is not v1.
- **A file sync engine.** Blob endpoint + folder watcher only (§4).
- **SSO/SAML, multi-tenancy, domain auto-join, billing.** All SaaS mechanisms (§6.4).
- **Office add-in / in-Excel colored overlays** (`PROJECT.md` §12 Risk #5).
- **Any AI feature that evaluates whether numbers are correct** (`PROJECT.md` §11). The AI
  narrates and flags. It never judges.
