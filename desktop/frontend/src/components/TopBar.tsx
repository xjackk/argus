import { useState } from "react";
import { Onboarding } from "./Onboarding";
import { ThemePicker } from "./ThemePicker";

interface TopBarProps {
  live: boolean; // true when connected to a running capture daemon
  workbooks: string[]; // the files tracked in the folder
  selected: string; // the currently viewed workbook
  onSelect: (w: string) => void;
}

// Top bar: a workbook (file) picker, sync/connectivity status, and Commit.
// Deliberately NOT a git-branch bar — finance reviewers think in "which file" +
// "what changed over time". When a daemon is connected, the status goes live.
export function TopBar({ live, workbooks, selected, onSelect }: TopBarProps) {
  const [open, setOpen] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [syncedLabel, setSyncedLabel] = useState("Synced 2m ago");
  const [showCommit, setShowCommit] = useState(false);
  const [commitMsg, setCommitMsg] = useState("");
  const [toast, setToast] = useState<string | null>(null);
  const [showOnboarding, setShowOnboarding] = useState(false);

  function flash(msg: string) {
    setToast(msg);
    window.setTimeout(() => setToast(null), 2200);
  }

  function doSync() {
    if (syncing) return;
    setSyncing(true);
    window.setTimeout(() => {
      setSyncing(false);
      setSyncedLabel("Synced just now");
      flash("Pulled latest from SharePoint");
    }, 1100);
  }

  function doCommit() {
    setShowCommit(false);
    const msg = commitMsg.trim();
    setCommitMsg("");
    flash(msg ? `Committed: “${msg}”` : "Version committed");
  }

  return (
    <div className="topbar">
      {/* Workbook picker */}
      <div className="tb picker wb" onClick={() => setOpen((o) => !o)}>
        <i className="ico">▤</i>
        <div className="tb-mid">
          <div className="l">Current workbook</div>
          <div className="v">{selected}</div>
        </div>
        <span className="caret">▾</span>
        {open && (
          <div className="menu" onClick={(e) => e.stopPropagation()}>
            {workbooks.map((w) => (
              <div
                key={w}
                className={"menu-item" + (w === selected ? " on" : "")}
                onClick={() => {
                  onSelect(w);
                  setOpen(false);
                }}
              >
                {w}
              </div>
            ))}
            <div className="menu-sep" />
            <div
              className="menu-item add"
              onClick={() => {
                setOpen(false);
                setShowOnboarding(true);
              }}
            >
              ＋ Watch a new folder…
            </div>
          </div>
        )}
      </div>

      <div className="spacer" />

      <div className="tb action">
        <ThemePicker />
        {live ? (
          <div className="conn live" title="Connected to the capture daemon">
            <span className="conn-dot" />
            Live
            <br />
            watching for saves
          </div>
        ) : (
          <div className="synced" onClick={doSync} title="Pull latest">
            {syncing ? "Syncing…" : syncedLabel}
            <br />
            via SharePoint
          </div>
        )}
        <button className="primary" onClick={() => setShowCommit(true)}>
          Commit version
        </button>
      </div>

      {/* Backdrop closes the open dropdown */}
      {open && <div className="menu-backdrop" onClick={() => setOpen(false)} />}

      {/* Commit modal (mocked) */}
      {showCommit && (
        <div className="modal-backdrop" onClick={() => setShowCommit(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-title">Commit a new version</div>
            <div className="modal-sub">
              Snapshots the current state of {selected}.
            </div>
            <textarea
              className="modal-input"
              placeholder="Summary of what changed…"
              value={commitMsg}
              autoFocus
              onChange={(e) => setCommitMsg(e.target.value)}
            />
            <div className="modal-actions">
              <button className="ghost" onClick={() => setShowCommit(false)}>
                Cancel
              </button>
              <button className="primary" onClick={doCommit}>
                Commit version
              </button>
            </div>
          </div>
        </div>
      )}

      {showOnboarding && (
        <Onboarding
          onClose={() => setShowOnboarding(false)}
          onDone={(msg) => {
            setShowOnboarding(false);
            flash(msg);
          }}
        />
      )}

      {toast && <div className="toast">{toast}</div>}
    </div>
  );
}
