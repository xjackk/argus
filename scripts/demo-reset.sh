#!/usr/bin/env bash
# Reset Argus to a known-good starting state for a live demo, and open the
# workbooks in LibreOffice ready to edit.
#
# TWO THINGS THAT BITE YOU, both handled here:
#
# 1. argusd holds the commit history in MEMORY and rewrites history.json
#    wholesale on every capture. Deleting the store while the daemon runs is a
#    no-op — the next save writes every old commit straight back and appends to
#    it, and leaves history.json referencing diff files that were removed (those
#    rows then load as "No diff"). The daemon must be stopped FIRST.
#
# 2. argusd SCANS the watched folder at startup and files every workbook already
#    there as a base commit. That is what we want here: restore all three
#    workbooks, restart, and you get three clean base commits ready to edit.
#
# Usage:
#   scripts/demo-reset.sh              # restore all workbooks + open in LibreOffice
#   scripts/demo-reset.sh --no-open    # same, but don't launch LibreOffice
#   scripts/demo-reset.sh --empty      # true cold open: no workbooks, no commits
#   scripts/demo-reset.sh --no-start   # wipe without relaunching the daemon
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

ROOT="$(pwd)"
STORE="desktop/frontend/public/store"
FOLDER="${ARGUS_FOLDER:-$HOME/ArgusDropbox}"
AUTHOR="${ARGUS_AUTHOR:-Avery (Analyst)}"
PRISTINE="$ROOT/demo-workbooks"     # committed, so the demo is reproducible
EMPTY=0; START=1; OPEN=1
for a in "$@"; do
  case "$a" in
    --empty)    EMPTY=1; OPEN=0 ;;
    --no-open)  OPEN=0 ;;
    --no-start) START=0 ;;
    *) echo "unknown flag: $a" >&2; exit 2 ;;
  esac
done

say() { printf '  %s\n' "$*"; }

echo "── 1. Stop the daemon ───────────────────────────────────"
if pgrep -f '[a]rgusd' >/dev/null 2>&1; then
  pkill -f '[a]rgusd' || true
  for _ in $(seq 10); do pgrep -f '[a]rgusd' >/dev/null 2>&1 || break; sleep 0.3; done
  pgrep -f '[a]rgusd' >/dev/null 2>&1 && { echo "  ✗ argusd would not stop"; exit 1; }
  say "✓ stopped"
else
  say "✓ not running"
fi

echo
echo "── 2. Wipe the store ────────────────────────────────────"
rm -rf "$STORE"
mkdir -p "$STORE/diffs" "$STORE/versions"
printf '{"commits":[]}\n' > "$STORE/history.json"
say "✓ $STORE emptied"

echo
echo "── 3. Workbooks ─────────────────────────────────────────"
mkdir -p "$FOLDER"
# LibreOffice lock files linger after a crash and clutter a clean start.
find "$FOLDER" -maxdepth 1 -name '.~lock.*#' -delete 2>/dev/null || true
rm -f "$FOLDER"/*.xlsx

if [ "$EMPTY" = "1" ]; then
  say "✓ folder emptied — TRUE cold open, no commits at all"
  say "  the app shows the bundled Atlas history badged \"Offline\";"
  say "  drag a workbook from $PRISTINE to go live"
else
  if [ ! -d "$PRISTINE" ] || [ -z "$(ls -A "$PRISTINE"/*.xlsx 2>/dev/null)" ]; then
    echo "  ✗ no pristine workbooks in $PRISTINE"; exit 1
  fi
  cp "$PRISTINE"/*.xlsx "$FOLDER"/
  n=$(find "$FOLDER" -maxdepth 1 -name '*.xlsx' | wc -l | tr -d ' ')
  say "✓ restored $n pristine workbook(s) from demo-workbooks/"
  for f in "$FOLDER"/*.xlsx; do say "    $(basename "$f")"; done
fi

echo
echo "── 4. Restart the daemon ────────────────────────────────"
if [ "$START" = "0" ]; then
  say "skipped (--no-start). Start it with:"
  say "  go run ./cmd/argusd -store $STORE -folder $FOLDER -author \"$AUTHOR\""
else
  go build -o /tmp/argusd-demo ./cmd/argusd
  nohup /tmp/argusd-demo -store "$ROOT/$STORE" -folder "$FOLDER" -author "$AUTHOR" \
    >/tmp/argusd-demo.log 2>&1 &
  # Wait for the startup scan to file the base commits before reporting.
  for _ in $(seq 20); do
    c=$(python3 -c "import json;print(len(json.load(open('$STORE/history.json'))['commits']))" 2>/dev/null || echo 0)
    [ "$c" != "0" ] && break
    [ "$EMPTY" = "1" ] && break
    sleep 0.4
  done
  pgrep -f '[a]rgusd-demo' >/dev/null 2>&1 || { echo "  ✗ failed to start"; tail -5 /tmp/argusd-demo.log; exit 1; }
  say "✓ running — watching $FOLDER as \"$AUTHOR\""
  python3 - <<PY
import json
h=json.load(open("$STORE/history.json"))
for c in h["commits"]:
    print(f"    {c['id']}  {c['message']}")
print(f"  → {len(h['commits'])} base commit(s) ready")
PY
fi

echo
echo "── 5. Open in LibreOffice ───────────────────────────────"
if [ "$OPEN" = "0" ]; then
  say "skipped"
elif [ -d "/Applications/LibreOffice.app" ]; then
  # Open all workbooks in one call so LibreOffice starts once, not three times.
  open -a "/Applications/LibreOffice.app" "$FOLDER"/*.xlsx
  say "✓ opened $(find "$FOLDER" -maxdepth 1 -name '*.xlsx' | wc -l | tr -d ' ') workbook(s)"
  say "  give it a few seconds to finish launching before you edit"
else
  say "⚠ LibreOffice not found at /Applications/LibreOffice.app — open them yourself"
fi

echo
echo "── Ready ────────────────────────────────────────────────"
say "1. Launch the app:  scripts/demo-app.sh"
say "2. Edit a value in LibreOffice and save (⌘S)"
say "3. Toast fires within ~3s — THEN CLICK THE TOP ROW in the rail"
say "   (the pane does not auto-advance)"
say "Change a real value each time; identical content is deduped and ignored."
