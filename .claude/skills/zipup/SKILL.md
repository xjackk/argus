---
name: zipup
description: Package Argus for a hackathon or demo submission upload — builds argus-submission.zip with the demo video, screenshots, a judge-facing README, and the full source, verified green and under the upload size limit. Use when the user says zipup, "package for submission", "what do I upload", or is preparing a hackathon deliverable.
---

# zipup — build the submission archive

Produces `argus-submission.zip` in the repo root: **~7 MB**, judge-first, and
verified to build before it ships.

## Run it

```bash
.claude/skills/zipup/build-submission.sh
```

For a different upload cap (default 35 MB):

```bash
.claude/skills/zipup/build-submission.sh --limit-mb 50
```

## What it does

1. **Preflight** — warns about uncommitted changes (`git archive` exports HEAD,
   so anything uncommitted silently will not ship), then runs `go build ./...`
   and the test suite. **Refuses to package a broken build.**
2. **Stages** the archive:
   - `README.md` — the judge-facing one-pager from
     `.claude/skills/zipup/SUBMISSION-README.md`
   - `demo.mp4` + `demo.gif` — the walkthrough at top level
   - `screenshots/` — the numbered stills
   - `source/` — every tracked file via `git archive HEAD`
3. **Zips and size-checks**, failing loudly if it exceeds the limit.

## Why it's structured this way

A judge has minutes. The zip is a *verification* artifact; the video and README
are the *persuasive* ones — so those sit at the top level and source goes in a
subfolder.

Excluded on purpose:

- **`node_modules/`** (~110 MB) — `npm install` regenerates it.
- **`desktop/build/`** — the compiled `.app` is unsigned, so Gatekeeper shows an
  "unidentified developer" warning. Most judges bail rather than
  right-click-open, and it's macOS-only. The video does this job better.
- **`.git/`** (~10 MB) — nobody unzips history, and the repo is public.

Zipping the folder as-is would be ~167 MB, roughly 5× over a typical limit, with
two-thirds of it `node_modules`.

## After running

Report to the user:

- the final path and size, and how much headroom is left under the limit
- any uncommitted changes the preflight flagged, since those did **not** ship
- a reminder that `github.com/xjackk/argus` and `xjackk.github.io/argus/` are
  both public — many judges prefer a link to an upload, so those belong in the
  submission form's description field too

## Tailoring it

`SUBMISSION-README.md` in this skill directory is a normal file — edit it. If the
hackathon publishes judging criteria (categories, "describe your stack", a theme
to address), work them into that file before running, so the archive answers the
questions actually being scored.

The `screenshots/` captions in the template refer to `01-main-cascade.png`
through `05-history-rail.png`. If the screenshot set changes, update those rows
so the README doesn't point at files that no longer exist.
