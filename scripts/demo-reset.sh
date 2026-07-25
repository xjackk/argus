#!/usr/bin/env bash
# Reset Argus to a clean slate for a live demo.
#
# THE THING THAT BITES YOU: argusd holds the commit history in MEMORY and
# rewrites history.json wholesale on every capture. Deleting the store while the
# daemon is running does nothing — the next save restores every old commit from
# memory and appends to it. The daemon must be stopped FIRST. This script does
# that in the right order.
#
# Usage:
#   scripts/demo-reset.sh              # wipe store, keep watched workbooks
#   scripts/demo-reset.sh --workbooks  # also reset workbooks to pristine samples
#   scripts/demo-reset.sh --no-start   # wipe but don't relaunch the daemon
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

STORE="desktop/frontend/public/store"
FOLDER="${ARGUS_FOLDER:-$HOME/ArgusDropbox}"
AUTHOR="${ARGUS_AUTHOR:-Avery (Analyst)}"
STAGE_DIR="${ARGUS_STAGE:-$HOME/ArgusDemoWorkbooks}"
RESET_WORKBOOKS=0
START=1
for a in "$@"; do
  case "$a" in
    --workbooks) RESET_WORKBOOKS=1 ;;
    --no-start)  START=0 ;;
    *) echo "unknown flag: $a" >&2; exit 2 ;;
  esac
done

say() { printf '  %s\n' "$*"; }

echo "── 1. Stop the daemon ───────────────────────────────────"
# Must happen before the wipe, or in-memory history is written straight back.
if pgrep -f '[a]rgusd' >/dev/null 2>&1; then
  pkill -f '[a]rgusd' || true
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    pgrep -f '[a]rgusd' >/dev/null 2>&1 || break
    sleep 0.3
  done
  if pgrep -f '[a]rgusd' >/dev/null 2>&1; then
    echo "  ✗ argusd would not stop — kill it by hand and rerun"; exit 1
  fi
  say "✓ stopped"
else
  say "✓ not running"
fi

echo
echo "── 2. Wipe the store ────────────────────────────────────"
rm -rf "$STORE"
mkdir -p "$STORE/diffs" "$STORE/versions"
# An EMPTY commits array is treated by the client exactly like "no daemon"
# (store.ts returns null on a zero-length list), so the app shows the bundled
# Atlas chain and labels itself "Offline — showing saved history". That is the
# intended cold-open: it flips to live the moment the first save is captured.
printf '{"commits":[]}\n' > "$STORE/history.json"
say "✓ $STORE emptied"

echo
echo "── 3. Watched folder ────────────────────────────────────"
mkdir -p "$FOLDER"
# LibreOffice lock files confuse a clean start and linger after a crash.
find "$FOLDER" -maxdepth 1 -name '.~lock.*#' -delete 2>/dev/null || true
say "✓ cleared editor lock files"

# argusd SCANS the folder at startup and files every workbook already sitting
# there as a base commit. So "empty store + populated folder" is not a blank
# slate: restart alone gives you N base commits and a dead "Initial version"
# pane. A true cold open needs the folder empty too.
if [ "$RESET_WORKBOOKS" = "1" ]; then
  mkdir -p "$STAGE_DIR"
  for f in "$FOLDER"/*.xlsx; do [ -e "$f" ] || continue; mv "$f" "$STAGE_DIR/"; done
  [ -f samples/income_statement_v1.xlsx ] && cp -f samples/income_statement_v1.xlsx "$STAGE_DIR/Income_Statement.xlsx"
  say "✓ folder emptied — TRUE cold open (no commits at all)"
  say "  workbooks moved to: $STAGE_DIR"
  say "  drag one in on stage → base commit → edit + save → cascade"
else
  n=$(find "$FOLDER" -maxdepth 1 -name '*.xlsx' | wc -l | tr -d ' ')
  say "kept $n workbook(s) in $FOLDER"
  if [ "$n" -gt 0 ]; then
    say "  ⚠ argusd files each of these as a base commit on start, so the app"
    say "    opens on an \"Initial version\" pane, NOT the Atlas fallback."
    say "    Use --workbooks for a true blank slate."
  fi
fi

echo
echo "── 4. Restart the daemon ────────────────────────────────"
if [ "$START" = "0" ]; then
  say "skipped (--no-start). Start it with:"
  say "  go run ./cmd/argusd -store $STORE -author \"$AUTHOR\""
else
  go build -o /tmp/argusd-demo ./cmd/argusd
  nohup /tmp/argusd-demo -store "$STORE" -folder "$FOLDER" -author "$AUTHOR" \
    >/tmp/argusd-demo.log 2>&1 &
  sleep 1
  if pgrep -f '[a]rgusd-demo' >/dev/null 2>&1; then
    say "✓ running — watching $FOLDER as \"$AUTHOR\""
    say "  log: /tmp/argusd-demo.log"
  else
    echo "  ✗ failed to start — see /tmp/argusd-demo.log"; tail -5 /tmp/argusd-demo.log; exit 1
  fi
fi

echo
echo "── Ready ────────────────────────────────────────────────"
if [ "$RESET_WORKBOOKS" = "1" ]; then
  say "Cold open: the app shows the bundled Atlas history, badged \"Offline\"."
  say "Drag a workbook from $STAGE_DIR into $FOLDER"
  say "→ base commit appears (live). Edit + save → cascade."
else
  say "The app opens on the existing workbooks' base commits — live, but with"
  say "no diffs yet. Save one in $FOLDER to produce a cascade."
fi
say "Hard-reload the browser (Cmd+Shift+R) so no stale state carries over."
