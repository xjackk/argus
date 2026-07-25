import { useState } from "react";

// Mocked onboarding — UI-SPEC States 3 (first-run) & 4 (folder setup) plus a
// fake company-SSO screen (PROJECT.md §8). Reached from the workbook menu, not
// the default view (the demo opens straight into the main window). Nothing here
// persists; it exists to show the adoption story.
interface Props {
  onClose: () => void;
  onDone: (msg: string) => void;
}

interface TrackFile {
  name: string;
  meta: string;
  fit: "good" | "none";
}

const DETECTED: TrackFile[] = [
  { name: "Project Atlas — LBO Model.xlsx", meta: "4 sheets · 312 formulas · modified 2h ago", fit: "good" },
  { name: "Atlas — Ops Model.xlsx", meta: "7 sheets · 890 formulas · modified 1d ago", fit: "good" },
  { name: "Team contact list.xlsx", meta: "1 sheet · 0 formulas", fit: "none" },
];

export function Onboarding({ onClose, onDone }: Props) {
  const [step, setStep] = useState<"welcome" | "setup" | "sso">("welcome");
  const [checked, setChecked] = useState<Record<string, boolean>>({
    "Project Atlas — LBO Model.xlsx": true,
    "Atlas — Ops Model.xlsx": true,
    "Team contact list.xlsx": false,
  });
  const trackCount = Object.values(checked).filter(Boolean).length;

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="onb" onClick={(e) => e.stopPropagation()}>
        <div className="onb-close" onClick={onClose}>
          ✕
        </div>

        {step === "welcome" && (
          <>
            <div className="onb-h">Let's get started</div>
            <div className="onb-sub">
              Track changes to your Excel models and see exactly what moved.
            </div>
            <div className="onb-split">
              <div className="onb-detected">
                <div className="onb-label">Detected on this machine</div>
                <div className="onb-folder">
                  <span className="onb-cloud">☁</span>
                  <div>
                    <div className="onb-fname">OneDrive — Atlas Partners</div>
                    <div className="onb-fmeta">14 workbooks found</div>
                  </div>
                </div>
                <div className="onb-folder">
                  <span className="onb-cloud">☁</span>
                  <div>
                    <div className="onb-fname">SharePoint — Deal Team A</div>
                    <div className="onb-fmeta">31 workbooks found</div>
                  </div>
                </div>
              </div>
              <div className="onb-options">
                <div
                  className="onb-opt primary-opt"
                  onClick={() => setStep("setup")}
                >
                  <div className="onb-opt-t">Watch a folder</div>
                  <div className="onb-opt-s">Auto-track every save — recommended</div>
                </div>
                <div className="onb-opt" onClick={() => setStep("setup")}>
                  <div className="onb-opt-t">Add a workbook</div>
                  <div className="onb-opt-s">Track a single file</div>
                </div>
                <div className="onb-opt" onClick={() => setStep("sso")}>
                  <div className="onb-opt-t">Join your team</div>
                  <div className="onb-opt-s">Sign in with company SSO</div>
                </div>
                <div className="onb-drop">Or drag an .xlsx file here</div>
              </div>
            </div>
            <div className="onb-foot">
              No account needed — everything stays on this machine until you join a team.
            </div>
          </>
        )}

        {step === "setup" && (
          <>
            <div className="onb-h">Which workbooks should we track?</div>
            <div className="onb-sub">
              We'll create a version every time one of these is saved. Turn any off later.
            </div>
            <div className="onb-path">
              <span className="onb-cloud">☁</span> ~/OneDrive/Atlas Partners/Models
            </div>
            <div className="onb-files">
              {DETECTED.map((f) => (
                <label
                  key={f.name}
                  className={"onb-file" + (f.fit === "none" ? " dim" : "")}
                >
                  <input
                    type="checkbox"
                    disabled={f.fit === "none"}
                    checked={!!checked[f.name]}
                    onChange={(e) =>
                      setChecked((c) => ({ ...c, [f.name]: e.target.checked }))
                    }
                  />
                  <div className="onb-file-main">
                    <div className="onb-fname">{f.name}</div>
                    <div className="onb-fmeta">{f.meta}</div>
                  </div>
                  <span className={"onb-fit " + f.fit}>
                    {f.fit === "good" ? "good fit" : "no formulas"}
                  </span>
                </label>
              ))}
            </div>
            <div className="onb-actions">
              <button className="ghost" onClick={() => setStep("welcome")}>
                Back
              </button>
              <button
                className="primary"
                onClick={() =>
                  onDone(`Now tracking ${trackCount} workbook${trackCount === 1 ? "" : "s"}`)
                }
              >
                Start tracking {trackCount} workbook{trackCount === 1 ? "" : "s"}
              </button>
            </div>
          </>
        )}

        {step === "sso" && (
          <>
            <div className="onb-h">Sign in to your organization</div>
            <div className="onb-sub">
              Anyone with an <b>@atlaspartners.com</b> address joins automatically —
              no admin needed.
            </div>
            <div className="onb-sso">
              <button
                className="sso-btn"
                onClick={() => onDone("Signed in as xkillilea@atlaspartners.com")}
              >
                <span className="ms-logo">⊞</span> Continue with Microsoft
              </button>
              <div className="onb-foot">
                Permissions are workbook-scoped — you only see models shared with you.
              </div>
            </div>
            <div className="onb-actions">
              <button className="ghost" onClick={() => setStep("welcome")}>
                Back
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
