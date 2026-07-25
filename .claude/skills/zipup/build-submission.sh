#!/usr/bin/env bash
# Package Argus for a hackathon/demo submission upload.
#
# Produces argus-submission.zip in the repo root, structured judge-first:
#
#   README.md          one page: what it is, why it matters, how to run
#   demo.mp4           the walkthrough (also demo.gif for previewers that
#                      won't inline video)
#   screenshots/       numbered stills
#   source/            every tracked file, via `git archive`
#
# Deliberately EXCLUDED, and why:
#   node_modules/      ~110 MB, trivially reinstalled with `npm install`
#   desktop/build/     the compiled .app is unsigned, so Gatekeeper blocks it
#                      with a scary dialog — a judge bails rather than
#                      right-click-opening, and it is macOS-only anyway
#   .git/              ~10 MB of history nobody unzips; the repo is public
#
# Usage: .claude/skills/zipup/build-submission.sh [--limit-mb N]
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

LIMIT_MB=35
[ "${1:-}" = "--limit-mb" ] && LIMIT_MB="${2:-35}"

SKILL_DIR=".claude/skills/zipup"
OUT="argus-submission.zip"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

say() { printf '  %s\n' "$*"; }

echo "── Preflight ────────────────────────────────────────────"

# `git archive` exports HEAD, so anything uncommitted silently will not ship.
DIRTY="$(git status --porcelain | grep -v '^?? \.claude/' || true)"
if [ -n "$DIRTY" ]; then
  echo "  ⚠ UNCOMMITTED CHANGES — these will NOT be in the zip:"
  printf '%s\n' "$DIRTY" | sed 's/^/      /'
  echo "      Commit them first, or accept that the source/ tree is behind."
  echo
else
  say "✓ working tree clean — source/ will match your HEAD"
fi

say "commit: $(git rev-parse --short HEAD) on $(git branch --show-current)"

# A submission that doesn't build is worse than a late one.
if go build ./... 2>/dev/null; then say "✓ go build"; else echo "  ✗ go build FAILED — fix before submitting"; exit 1; fi
if go test ./engine/... ./narrator/... ./spreadsheet/... >/dev/null 2>&1; then
  say "✓ go test"
else
  echo "  ✗ go test FAILED — fix before submitting"; exit 1
fi

echo
echo "── Staging ──────────────────────────────────────────────"

PKG="$STAGE/argus-submission"
mkdir -p "$PKG"

# 1. Source: tracked files only, at HEAD.
mkdir -p "$PKG/source"
git archive HEAD | tar -x -C "$PKG/source"
say "source/        $(find "$PKG/source" -type f | wc -l | tr -d ' ') files"

# 2. The demo — the single most-watched artifact. Top level, obvious name.
if [ -f docs/assets/argus-demo.mp4 ]; then
  cp docs/assets/argus-demo.mp4 "$PKG/demo.mp4"; say "demo.mp4       $(du -h docs/assets/argus-demo.mp4 | cut -f1)"
fi
if [ -f screenshots/argus-demo.gif ]; then
  cp screenshots/argus-demo.gif "$PKG/demo.gif"; say "demo.gif       $(du -h screenshots/argus-demo.gif | cut -f1)  (for previewers that won't play mp4)"
fi

# 3. Stills.
if [ -d screenshots ]; then
  mkdir -p "$PKG/screenshots"
  find screenshots -maxdepth 1 -type f -name '*.png' -exec cp {} "$PKG/screenshots/" \;
  say "screenshots/   $(find "$PKG/screenshots" -type f | wc -l | tr -d ' ') images"
fi

# 4. Hero image, next to the README that references it — the submission README
# sits at the package root and cannot reach docs/assets/.
if [ -f docs/assets/argus-panoptes.jpg ]; then
  cp docs/assets/argus-panoptes.jpg "$PKG/argus-panoptes.jpg"; say "argus-panoptes.jpg  $(du -h docs/assets/argus-panoptes.jpg | cut -f1)"
fi

# 5. Judge-facing README. Falls back to the repo's own if the template is gone.
if [ -f "$SKILL_DIR/SUBMISSION-README.md" ]; then
  cp "$SKILL_DIR/SUBMISSION-README.md" "$PKG/README.md"; say "README.md      submission template"
else
  cp README.md "$PKG/README.md"; say "README.md      repo README (template missing)"
fi

echo
echo "── Packaging ────────────────────────────────────────────"
rm -f "$OUT"
( cd "$STAGE" && zip -qr "argus-submission.zip" "argus-submission" )
mv "$STAGE/argus-submission.zip" "$OUT"

BYTES=$(wc -c < "$OUT" | tr -d ' ')
MB=$(( BYTES / 1048576 ))
say "$OUT — $(du -h "$OUT" | cut -f1)"

echo
if [ "$MB" -ge "$LIMIT_MB" ]; then
  echo "  ✗ OVER the ${LIMIT_MB} MB limit. Drop demo.gif (largest single item) and rerun."
  exit 1
fi
echo "  ✓ under the ${LIMIT_MB} MB limit (${MB} MB used, $(( LIMIT_MB - MB )) MB spare)"
echo
echo "  Upload:  $(pwd)/$OUT"
echo "  Inspect: unzip -l $OUT | head -30"
