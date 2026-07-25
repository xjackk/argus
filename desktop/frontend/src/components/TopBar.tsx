import { useState } from "react";
import { Onboarding } from "./Onboarding";

const WORKBOOKS = [
  "Project Atlas — LBO",
  "Atlas — Ops Model",
  "Meridian — Credit Model",
];

// Top bar: a workbook picker, sync status, and Commit. Deliberately NOT a
// git-branch bar — finance reviewers think in "which model" + "what changed
// over time", so there's no branch/scenario concept. Dropdown and Commit/Sync
// are mocked (no persistence) but fully interactive so the chrome feels real.
export function TopBar() {
  const [workbook, setWorkbook] = useState(WORKBOOKS[0]);
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
          <div className="v">{workbook}</div>
        </div>
        <span className="caret">▾</span>
        {open && (
          <div className="menu" onClick={(e) => e.stopPropagation()}>
            {WORKBOOKS.map((w) => (
              <div
                key={w}
                className={"menu-item" + (w === workbook ? " on" : "")}
                onClick={() => {
                  setWorkbook(w);
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
        <div className="synced" onClick={doSync} title="Pull latest">
          {syncing ? "Syncing…" : syncedLabel}
          <br />
          via SharePoint
        </div>
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
              Snapshots the current state of {workbook}.
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
