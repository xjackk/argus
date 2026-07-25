import { useEffect, useState } from "react";

// App-wide zoom: a native-app affordance for "make everything bigger so the room
// can read it." Uses CSS `zoom` on the document root (what browser zoom does),
// which reflows the whole layout uniformly — grid, panes, text — instead of just
// scaling one font. Persisted so it survives refresh and the Wails relaunch.
const MIN = 0.8;
const MAX = 1.6;
const STEP = 0.1;

function clamp(z: number): number {
  return Math.min(MAX, Math.max(MIN, Math.round(z * 100) / 100));
}

export function ZoomControl() {
  const [zoom, setZoom] = useState(() => {
    const v = parseFloat(localStorage.getItem("argus.zoom") || "1");
    return isNaN(v) ? 1 : clamp(v);
  });

  useEffect(() => {
    // `zoom` isn't in the typed CSSStyleDeclaration but every Chromium-based
    // engine (including the Wails webview) honors it.
    (document.documentElement.style as unknown as { zoom: string }).zoom =
      String(zoom);
    localStorage.setItem("argus.zoom", String(zoom));
  }, [zoom]);

  // ⌘/Ctrl +/-/0 — the shortcuts people already expect for zoom.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (!(e.metaKey || e.ctrlKey)) return;
      if (e.key === "=" || e.key === "+") {
        e.preventDefault();
        setZoom((z) => clamp(z + STEP));
      } else if (e.key === "-") {
        e.preventDefault();
        setZoom((z) => clamp(z - STEP));
      } else if (e.key === "0") {
        e.preventDefault();
        setZoom(1);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <div className="zoomctl" title="Scale the app  (⌘ + / − / 0)">
      <button
        className="zc-btn"
        onClick={() => setZoom((z) => clamp(z - STEP))}
        disabled={zoom <= MIN}
        aria-label="Zoom out"
      >
        −
      </button>
      <button
        className="zc-val"
        onClick={() => setZoom(1)}
        title="Reset to 100%"
      >
        {Math.round(zoom * 100)}%
      </button>
      <button
        className="zc-btn"
        onClick={() => setZoom((z) => clamp(z + STEP))}
        disabled={zoom >= MAX}
        aria-label="Zoom in"
      >
        +
      </button>
    </div>
  );
}
