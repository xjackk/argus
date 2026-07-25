package main

import (
	"context"

	"argus/engine"
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
