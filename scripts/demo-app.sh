#!/usr/bin/env bash
# Launch the packaged Argus desktop app for a demo.
#
# WHY THE PACKAGED BUILD AND NOT `npm run dev`:
#   - No browser chrome. The pitch is "GitHub Desktop for finance"; presenting it
#     in a tab with a localhost URL bar undercuts that in the first two seconds.
#   - No dev server to die. Vite restarts on every file touch and can drop
#     entirely — it went down twice during QA. A packaged binary has no such
#     failure mode mid-demo.
#   - Open-in-Excel only works here. It needs the Wails binding, so in a browser
#     the button is permanently disabled.
#
# THE TRAP THIS SCRIPT HANDLES: the frontend is `go:embed`-ed, so
# desktop/frontend/public/store is baked into the binary AT BUILD TIME. Without
# ARGUS_STORE the app would show a frozen snapshot and never update when the
# daemon captures a save — the live demo would silently do nothing. Setting
# ARGUS_STORE routes /store/* at a real directory (see liveStoreMiddleware in
# desktop/main.go).
#
# Usage:
#   scripts/demo-app.sh              # launch (build only if missing)
#   scripts/demo-app.sh --rebuild    # force a fresh build first
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

ROOT="$(pwd)"
STORE="${ARGUS_STORE:-$ROOT/desktop/frontend/public/store}"
APP="desktop/build/bin/argus.app"
BIN="$APP/Contents/MacOS/argus"
REBUILD=0
[ "${1:-}" = "--rebuild" ] && REBUILD=1

say() { printf '  %s\n' "$*"; }

echo "── 1. Capture daemon ────────────────────────────────────"
if pgrep -f '[a]rgusd' >/dev/null 2>&1; then
  say "✓ argusd running — live capture will work"
else
  say "⚠ argusd is NOT running. The app will open on the bundled Atlas history"
  say "  and never go live. Start it first:"
  say "      scripts/demo-reset.sh --workbooks"
fi

echo
echo "── 2. Build ─────────────────────────────────────────────"
if [ "$REBUILD" = "1" ] || [ ! -x "$BIN" ]; then
  say "building (this takes a minute — do it BEFORE you're on stage)…"
  # Wails builds the frontend itself; the app bundle lands in desktop/build/bin.
  ( cd desktop && wails build ) >/tmp/argus-wails-build.log 2>&1 || {
    echo "  ✗ build failed — tail of /tmp/argus-wails-build.log:"; tail -20 /tmp/argus-wails-build.log; exit 1; }
  say "✓ built $APP"
else
  say "✓ using existing build ($(date -r "$BIN" '+%Y-%m-%d %H:%M'))"
  say "  (pass --rebuild after any code change)"
fi

echo
echo "── 3. Launch ────────────────────────────────────────────"
# Kill a previous instance so demo windows don't stack.
pkill -f "$BIN" 2>/dev/null && say "closed a previous instance" || true

if [ ! -d "$STORE" ]; then
  say "⚠ store not found at $STORE — creating an empty one"
  mkdir -p "$STORE/diffs" "$STORE/versions"
  printf '{"commits":[]}\n' > "$STORE/history.json"
fi

say "ARGUS_STORE=$STORE"
# Run the binary directly rather than `open -a`: `open` does not forward env
# vars to the bundle, so the app would silently fall back to the embedded
# snapshot — exactly the failure this script exists to prevent.
ARGUS_STORE="$STORE" "$BIN" >/tmp/argus-app.log 2>&1 &
sleep 2

if pgrep -f "$BIN" >/dev/null 2>&1; then
  say "✓ Argus is up (native window, no browser chrome)"
  say "  log: /tmp/argus-app.log"
else
  echo "  ✗ failed to start — see /tmp/argus-app.log"; tail -10 /tmp/argus-app.log; exit 1
fi

echo
echo "── Demo notes ───────────────────────────────────────────"
say "• Save a workbook in ~/ArgusDropbox → toast fires within ~3s"
say "• THEN CLICK THE TOP ROW in the rail — the pane does not auto-advance"
say "• Change a real value each save; identical content is deduped and ignored"
say "• Open-in-Excel works in this build (it is disabled in the browser)"
