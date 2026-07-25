// Package engine implements Argus's deterministic Excel diff engine.
//
// The engine never recomputes spreadsheet values — it reads the cached values
// Excel already stored, diffs them positionally, builds a cross-sheet
// dependency graph from the formulas, classifies each change as authored or
// computed, and traces the cascade (blast radius) from each authored edit.
//
// The types below are the frozen engine ⇄ UI contract (DATA-CONTRACT.md).
package engine

type DiffResult struct {
	SchemaVersion int         `json:"schemaVersion"`
	From          VersionMeta `json:"from"`
	To            VersionMeta `json:"to"`
	Summary       Summary     `json:"summary"`
	Sheets        []SheetDiff `json:"sheets"`
	Cascades      []Cascade   `json:"cascades"`
	Anomalies     []Anomaly   `json:"anomalies"`
}

type VersionMeta struct {
	Label       string `json:"label"`
	Path        string `json:"path"`
	CommittedAt string `json:"committedAt"`
	Author      string `json:"author"`
}

type Summary struct {
	AuthoredCount  int      `json:"authoredCount"`
	ComputedCount  int      `json:"computedCount"`
	SheetsAffected []string `json:"sheetsAffected"`
	Narrative      *string  `json:"narrative"` // nil if AI skipped — UI must tolerate
}

type SheetDiff struct {
	Name         string       `json:"name"`
	Changes      []CellChange `json:"changes"`
	RowsInserted []int        `json:"rowsInserted"`
	RowsDeleted  []int        `json:"rowsDeleted"`
}

type CellChange struct {
	Coord          string   `json:"coord"`
	Row            int      `json:"row"`
	Col            int      `json:"col"`
	Label          *string  `json:"label"`
	Classification string   `json:"classification"` // "authored" | "computed"
	OldValue       any      `json:"oldValue"`
	NewValue       any      `json:"newValue"`
	OldFormula     *string  `json:"oldFormula"`
	NewFormula     *string  `json:"newFormula"`
	DisplayFormat  string   `json:"displayFormat"`
	DependsOn      []string `json:"dependsOn"`
	Dependents     []string `json:"dependents"`
	CausedBy       []string `json:"causedBy"`
	Magnitude      *float64 `json:"magnitude"`
}

type Cascade struct {
	Origin        string   `json:"origin"`
	OriginLabel   *string  `json:"originLabel"`
	OldValue      any      `json:"oldValue"`
	NewValue      any      `json:"newValue"`
	AffectedCount int      `json:"affectedCount"`
	Affected      []string `json:"affected"`
	TopMovers     []Mover  `json:"topMovers"`
}

type Mover struct {
	Ref       string   `json:"ref"`
	Label     *string  `json:"label"`
	OldValue  any      `json:"oldValue"`
	NewValue  any      `json:"newValue"`
	Magnitude *float64 `json:"magnitude"`
}

type Anomaly struct {
	Type       string  `json:"type"`
	Ref        string  `json:"ref"`
	Label      *string `json:"label"`
	Severity   string  `json:"severity"` // "high" | "medium" | "low"
	Message    string  `json:"message"`
	OldFormula *string `json:"oldFormula,omitempty"`
	NewValue   any     `json:"newValue,omitempty"`
}
