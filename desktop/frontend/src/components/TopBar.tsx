import { useState } from "react";
import { ThemePicker } from "./ThemePicker";
import { ZoomControl } from "./ZoomControl";

interface TopBarProps {
  live: boolean; // true when connected to a running capture daemon
  workbooks: string[]; // the files tracked in the folder
  selected: string; // the currently viewed workbook
  onSelect: (w: string) => void;
}

// Top bar: a workbook (file) picker and connectivity status. Deliberately NOT a
// git-branch bar — finance reviewers think in "which file" + "what changed over
// time". There is NO manual "commit" button by design: Argus captures a version
// automatically on every save (the daemon is the single watcher/writer), so
// committing *is* saving. Likewise the watched folder is daemon-side config, not
// something the client sets — the client only reads the store the daemon writes.
export function TopBar({ live, workbooks, selected, onSelect }: TopBarProps) {
  const [open, setOpen] = useState(false);

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
            {/* Read-only status, not an action: the watched folder is set on the
                daemon (a laptop user at launch, or an admin on the box), so the
                client has nothing to configure here. */}
            <div
              className="menu-cap"
              style={{ padding: "8px 12px", fontSize: 11, opacity: 0.55 }}
            >
              Versions captured automatically on save
            </div>
          </div>
        )}
      </div>

      <div className="spacer" />

      <div className="tb action">
        <ZoomControl />
        <ThemePicker />
        {live ? (
          <div className="conn live" title="Connected to the capture daemon">
            <span className="conn-dot" />
            <span className="conn-text">
              <span className="conn-l">Live</span>
              <span className="conn-s">watching for saves</span>
            </span>
          </div>
        ) : (
          // Honest offline state — no daemon connected. Editing still works; the
          // app just shows the last saved history. (No fake "syncing" here: you
          // don't need to be connected to anything to review your own files.)
          <div className="conn" title="No daemon connected — showing saved history">
            <span className="conn-text">
              <span className="conn-l">Offline</span>
              <span className="conn-s">showing saved history</span>
            </span>
          </div>
        )}
      </div>

      {/* Backdrop closes the open dropdown */}
      {open && <div className="menu-backdrop" onClick={() => setOpen(false)} />}
    </div>
  );
}
