export interface Theme {
  id: string;
  name: string;
  mode: "dark" | "light";
}

// Two dark + two light. Default is Midnight (the base :root palette); the others
// override CSS variables under :root[data-theme="…"] in styles.css.
export const THEMES: Theme[] = [
  { id: "midnight", name: "Midnight", mode: "dark" },
  { id: "tokyo", name: "Tokyo Night", mode: "dark" },
  { id: "github-light", name: "GitHub Light", mode: "light" },
  { id: "solarized-light", name: "Solarized Light", mode: "light" },
];

const KEY = "argus-theme";

export function getTheme(): string {
  return localStorage.getItem(KEY) || "midnight";
}

export function applyTheme(id: string): void {
  document.documentElement.dataset.theme = id;
  localStorage.setItem(KEY, id);
}
