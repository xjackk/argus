import { useState } from "react";
import { Onboarding } from "./Onboarding";

const WORKBOOKS = [
  "Project Atlas — LBO",
  "Atlas — Ops Model",
  "Meridian — Credit Model",
];
const SCENARIOS = [
  "deal-team/base",
  "deal-team/upside",
  "deal-team/downside",
  "modeling/returns",
];

// GitHub-Desktop-style top bar: workbook (repo) ▾, scenario (branch) ▾, sync
// status, and a primary action. Dropdowns and Commit/Sync are mocked (no
// persistence) but fully interactive so the chrome feels real.
export function TopBar() {
  const [workbook, setWorkbook] = useState(WORKBOOKS[0]);
  const [scenario, setScenario] = useState(SCENARIOS[0]);
  const [open, setOpen] = useState<null | "wb" | "sc">(null);
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
      <div
        className="tb picker wb"
        onClick={() => setOpen(open === "wb" ? null : "wb")}
      >
        <i className="ico">▤</i>
        <div className="tb-mid">
          <div className="l">Current workbook</div>
          <div className="v">{workbook}</div>
        </div>
        <span className="caret">▾</span>
        {open === "wb" && (
          <div className="menu" onClick={(e) => e.stopPropagation()}>
            {WORKBOOKS.map((w) => (
              <div
                key={w}
                className={"menu-item" + (w === workbook ? " on" : "")}
                onClick={() => {
                  setWorkbook(w);
                  setOpen(null);
                }}
              >
                {w}
              </div>
            ))}
            <div className="menu-sep" />
            <div
              className="menu-item add"
              onClick={() => {
                setOpen(null);
                setShowOnboarding(true);
              }}
            >
              ＋ Watch a new folder…
            </div>
          </div>
        )}
      </div>

      {/* Scenario picker */}
      <div
        className="tb picker sc"
        onClick={() => setOpen(open === "sc" ? null : "sc")}
      >
        <i className="ico">⑂</i>
        <div className="tb-mid">
          <div className="l">Current scenario</div>
          <div className="v">{scenario}</div>
        </div>
        <span className="caret">▾</span>
        {open === "sc" && (
          <div className="menu" onClick={(e) => e.stopPropagation()}>
            <div className="menu-head">Switch fork</div>
            {SCENARIOS.map((s) => (
              <div
                key={s}
                className={"menu-item mono" + (s === scenario ? " on" : "")}
                onClick={() => {
                  setScenario(s);
                  setOpen(null);
                }}
              >
                ⑂ {s}
              </div>
            ))}
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

      {/* Backdrop closes any open dropdown */}
      {open && <div className="menu-backdrop" onClick={() => setOpen(null)} />}

      {/* Commit modal (mocked) */}
      {showCommit && (
        <div className="modal-backdrop" onClick={() => setShowCommit(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-title">Commit a new version</div>
            <div className="modal-sub">
              Snapshots the current workbook to {workbook} · {scenario}.
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
