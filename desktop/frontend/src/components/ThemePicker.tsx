import { useState } from "react";
import { THEMES, getTheme, applyTheme } from "../data/theme";

// Compact theme picker for the top bar: two dark + two light schemes,
// persisted to localStorage.
export function ThemePicker() {
  const [theme, setTheme] = useState(getTheme());
  const [open, setOpen] = useState(false);

  function pick(id: string) {
    applyTheme(id);
    setTheme(id);
    setOpen(false);
  }

  const dark = THEMES.filter((t) => t.mode === "dark");
  const light = THEMES.filter((t) => t.mode === "light");

  return (
    <div className="theme-picker" onClick={(e) => e.stopPropagation()}>
      <button
        className="theme-btn"
        onClick={() => setOpen((o) => !o)}
        title="Theme"
      >
        ◐
      </button>
      {open && (
        <>
          <div className="menu-backdrop" onClick={() => setOpen(false)} />
          <div className="menu theme-menu">
            <div className="menu-head">Dark</div>
            {dark.map((t) => (
              <div
                key={t.id}
                className={"menu-item" + (t.id === theme ? " on" : "")}
                onClick={() => pick(t.id)}
              >
                {t.name}
                {t.id === theme && <span className="menu-check">✓</span>}
              </div>
            ))}
            <div className="menu-head">Light</div>
            {light.map((t) => (
              <div
                key={t.id}
                className={"menu-item" + (t.id === theme ? " on" : "")}
                onClick={() => pick(t.id)}
              >
                {t.name}
                {t.id === theme && <span className="menu-check">✓</span>}
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
