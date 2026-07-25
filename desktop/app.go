package main

import (
	"context"

	"argus/engine"
	"argus/spreadsheet"
)

// App is the Wails application backend. It exposes the deterministic diff
// engine to the frontend.
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved so we can call
// the Wails runtime methods later.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Diff computes the structured diff between two .xlsx workbooks and returns the
// DiffResult the frontend renders. This is the single binding the UI calls to
// replace the static fixture (see frontend/src/data/loadDiff.ts).
func (a *App) Diff(pathA, pathB string) (engine.DiffResult, error) {
	return engine.Diff(pathA, pathB)
}

// OpenInSpreadsheet opens a workbook version in Excel or LibreOffice, at the
// given sheet, and returns the name of the application it used.
//
// Argus stores diffs rather than whole sheets, so this is the door out to the
// real tool when a reviewer wants to browse the full model — or look at a sheet
// this commit never touched. It always opens a read-only COPY, never the
// original: a historical version is immutable, and selecting a tab is a write.
// See package spreadsheet for the details.
func (a *App) OpenInSpreadsheet(path, sheet, label string) (string, error) {
	return spreadsheet.Opener{}.OpenAt(path, sheet, label)
}

// SpreadsheetApps lists the spreadsheet applications available on this machine,
// best first. The UI uses the first name to label its button ("Open in
// Microsoft Excel") instead of guessing what the user has installed.
func (a *App) SpreadsheetApps() []string {
	apps := spreadsheet.Detect()
	names := make([]string, 0, len(apps))
	for _, app := range apps {
		names = append(names, app.Name)
	}
	return names
}
