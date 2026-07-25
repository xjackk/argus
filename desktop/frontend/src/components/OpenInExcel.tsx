import { useEffect, useState } from "react";
import { available, preferredApp, openInSpreadsheet } from "../data/openExternal";

// The door out to the real spreadsheet app. Argus stores diffs, not whole
// sheets, so anything beyond "what changed" — browsing an untouched sheet,
// checking a formula's neighbours, seeing the model whole — belongs in Excel.
//
// The label names the application actually installed rather than assuming
// Excel, and the button disables itself in browser dev mode where there is no
// binding to call.

interface Props {
  /** The workbook version to open (DiffResult.to.path). */
  path: string;
  /** Sheet to select on open. */
  sheet: string;
  /** Title-bar name for the read-only copy, e.g. "Atlas LBO @ c06". */
  label: string;
  className?: string;
}

export function OpenInExcel({ path, sheet, label, className }: Props) {
  const [appName, setAppName] = useState<string | null>(null);
  const [hasBinding, setHasBinding] = useState(available());
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const enabled = hasBinding && !!path;

  // The Wails runtime injects window.go shortly after the page loads; in a
  // production build the first render can beat it. Checking once at render used
  // to latch `false` forever (button permanently greyed). Poll briefly instead.
  useEffect(() => {
    if (hasBinding) return;
    let n = 0;
    const t = window.setInterval(() => {
      if (available()) {
        setHasBinding(true);
        window.clearInterval(t);
      } else if (++n > 25) {
        window.clearInterval(t); // ~5s — genuinely not the desktop app
      }
    }, 200);
    return () => window.clearInterval(t);
  }, [hasBinding]);

  // Resolve the installed app's name once the binding is present.
  useEffect(() => {
    if (!hasBinding) return;
    let active = true;
    preferredApp().then((n) => active && setAppName(n));
    return () => {
      active = false;
    };
  }, [hasBinding]);

  async function open() {
    if (!enabled || busy) return;
    setBusy(true);
    setError(null);
    try {
      await openInSpreadsheet(path, sheet, label);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  const text = busy ? "Opening…" : `Open in ${appName ?? "Excel"}`;

  return (
    <div className={"openx" + (className ? " " + className : "")}>
      <button
        className="openx-btn"
        onClick={open}
        disabled={!enabled || busy}
        title={
          enabled
            ? `Open a read-only copy of this version at ${sheet}`
            : "Available in the Argus desktop app"
        }
      >
        {text}
      </button>
      {error && <div className="openx-err">{error}</div>}
    </div>
  );
}
