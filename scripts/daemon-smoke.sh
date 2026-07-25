#!/usr/bin/env bash
#
# daemon-smoke.sh — end-to-end smoke test for the argusd capture daemon.
#
# Builds argusd, runs it against a throwaway folder + store, drops a few real
# .xlsx fixtures into the watched folder (with sleeps for the debounce), then
# greps the log and the store to assert that commits actually appeared. Kills
# the daemon and prints PASS/FAIL.
#
# Usage:  bash scripts/daemon-smoke.sh
set -u

# Resolve repo root regardless of where the script is called from.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT" || { echo "FAIL: cannot cd to repo root"; exit 1; }

TESTDATA="$ROOT/engine/testdata"
WORK="$(mktemp -d)"
FOLDER="$WORK/watched"
STORE="$WORK/store"
LOG="$WORK/argusd.log"
BIN="$WORK/argusd"
PID=""

cleanup() {
  if [[ -n "$PID" ]] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null
    wait "$PID" 2>/dev/null
  fi
  rm -rf "$WORK"
}
trap cleanup EXIT

fail() {
  echo "FAIL: $*"
  echo "----- daemon log -----"
  cat "$LOG" 2>/dev/null || echo "(no log)"
  exit 1
}

mkdir -p "$FOLDER" "$STORE"

echo "==> building argusd"
go build -o "$BIN" ./cmd/argusd || fail "build failed"

echo "==> starting daemon (folder=$FOLDER store=$STORE)"
"$BIN" -folder "$FOLDER" -store "$STORE" -author "smoke-tester" >"$LOG" 2>&1 &
PID=$!
sleep 1
kill -0 "$PID" 2>/dev/null || fail "daemon exited immediately"

drop() { # src-fixture  dest-name
  cp "$TESTDATA/$1" "$FOLDER/$2"
  sleep 1.5   # > 800ms debounce window
}

echo "==> dropping fixtures"
drop atlas_v1_base.xlsx           model.xlsx   # base commit
drop atlas_v2_exit_multiple.xlsx  model.xlsx   # authored edit + cascade
drop atlas_v5_hardcode_override.xlsx model.xlsx # anomaly

sleep 1

HISTORY="$STORE/history.json"
[[ -f "$HISTORY" ]] || fail "history.json was not written"

# Count commits in the store (avoid a jq dependency; count "id" keys).
COMMITS="$(grep -c '"id"' "$HISTORY")"
echo "==> store reports $COMMITS commit(s)"
[[ "$COMMITS" -ge 3 ]] || fail "expected >= 3 commits, got $COMMITS"

grep -q '"base": true'   "$HISTORY" || fail "no base commit recorded"
grep -q '"anomaly": true' "$HISTORY" || fail "no anomaly commit recorded"

[[ -f "$STORE/diffs/c002.json" ]] || fail "missing diff file c002.json"
[[ -f "$STORE/diffs/c003.json" ]] || fail "missing diff file c003.json"

grep -q 'tracked model.xlsx' "$LOG" || fail "log missing base-capture line"

# --- RESUME check: restart with a different author, drop one more edit ---
echo "==> restarting daemon to verify RESUME"
kill "$PID" 2>/dev/null; wait "$PID" 2>/dev/null; PID=""

"$BIN" -folder "$FOLDER" -store "$STORE" -author "second-user" >>"$LOG" 2>&1 &
PID=$!
sleep 1
kill -0 "$PID" 2>/dev/null || fail "daemon exited immediately on restart"
grep -q 'resumed 3 commit' "$LOG" || fail "restart did not resume existing 3 commits"

drop atlas_v3_downside.xlsx model.xlsx   # should become c004, author second-user
sleep 1

COMMITS2="$(grep -c '"id"' "$HISTORY")"
echo "==> after resume+edit, store reports $COMMITS2 commit(s)"
[[ "$COMMITS2" -eq 4 ]] || fail "resume did not continue sequence (want 4 commits, got $COMMITS2)"
grep -q '"c004"' "$HISTORY" || fail "id sequence did not continue to c004"
grep -q '"second-user"' "$HISTORY" || fail "resumed edit not attributed to new author"

echo "PASS: base, authored-edit, anomaly, and resume all captured correctly"
exit 0
