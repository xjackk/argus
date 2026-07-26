#!/usr/bin/env bash
#
# run-daemon.sh — start the Argus capture daemon for a live demo.
#
# Watches ~/ArgusDropbox and writes the store the client reads, using an ABSOLUTE
# -store path so each diff's file paths stay openable (the Wails "Open in Excel"
# button). Resumes the existing timeline on restart. Runs in the foreground —
# you'll see every capture logged; Ctrl-C stops it.
#
# Usage:
#   scripts/run-daemon.sh                          # author "Avery (Analyst)"
#   scripts/run-daemon.sh "S. Patel (VP)"          # a different author
#   scripts/run-daemon.sh "Avery (Analyst)" -http :7777   # + the HTTP API
#   scripts/run-daemon.sh -http :7777              # default author + extra flags
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STORE="$ROOT/desktop/frontend/public/store"

# First arg is the author, UNLESS it looks like a flag (starts with "-"), in
# which case we keep the default author and pass everything through to argusd.
AUTHOR="Avery (Analyst)"
if [ "${1:-}" != "" ] && [ "${1#-}" = "${1:-}" ]; then
  AUTHOR="$1"
  shift
fi

# Refuse to start a second daemon over the same folder — two watchers would
# fight over the store.
if pgrep -f 'argusd -store' >/dev/null 2>&1; then
  echo "! A daemon already seems to be running. Stop it first:" >&2
  echo "    pkill -f argusd" >&2
  exit 1
fi

echo "── Argus daemon ─────────────────────────────────"
echo "  watching : ~/ArgusDropbox   (created if missing)"
echo "  store    : $STORE"
echo "  author   : $AUTHOR"
[ "$#" -gt 0 ] && echo "  extra    : $*"
echo "  Ctrl-C to stop."
echo

cd "$ROOT"
exec go run ./cmd/argusd -store "$STORE" -author "$AUTHOR" "$@"
