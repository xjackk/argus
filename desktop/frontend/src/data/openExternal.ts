// Opening a workbook version in Excel / LibreOffice.
//
// This has to run on the CLIENT machine, so it goes through the Wails binding
// rather than the daemon — in a self-hosted setup the daemon may be on a server
// in another building, where launching a spreadsheet app helps nobody.
//
// In a plain browser (`npm run dev`) the binding is absent. Callers get
// available() === false and should render the action disabled rather than
// failing on click.

interface WailsApp {
  OpenInSpreadsheet?(path: string, sheet: string, label: string): Promise<string>;
  SpreadsheetApps?(): Promise<string[]>;
}

function wailsApp(): WailsApp | undefined {
  return (window as unknown as { go?: { main?: { App?: WailsApp } } }).go?.main?.App;
}

/** True when this build can actually launch a spreadsheet application. */
export function available(): boolean {
  return !!wailsApp()?.OpenInSpreadsheet;
}

/**
 * The app a click would open, e.g. "Microsoft Excel" — used to label the
 * button honestly instead of assuming Excel is installed. Null in the browser.
 */
export async function preferredApp(): Promise<string | null> {
  const app = wailsApp();
  if (!app?.SpreadsheetApps) return null;
  try {
    const names = await app.SpreadsheetApps();
    return names[0] ?? null;
  } catch {
    return null;
  }
}

/**
 * Open `path` at `sheet` in the user's spreadsheet application. Resolves to the
 * application's name. The Go side opens a read-only copy, never the original.
 */
export async function openInSpreadsheet(
  path: string,
  sheet: string,
  label: string
): Promise<string> {
  const app = wailsApp();
  if (!app?.OpenInSpreadsheet) {
    throw new Error("not running in the desktop app");
  }
  return app.OpenInSpreadsheet(path, sheet, label);
}
