package engine

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/efp"
	"github.com/xuri/excelize/v2"
)

// cellData is the engine's minimal view of one populated cell.
type cellData struct {
	rawValue string // raw (unformatted) stored value; "" if the cell is empty
	formula  string // formula string incl. leading "="; "" for a constant
	format   string // Excel number-format code (displayFormat); "General" fallback
}

// workbook is a parsed workbook: sheet order plus sheet -> coord -> cell.
type workbook struct {
	sheetOrder []string
	cells      map[string]map[string]cellData
}

// Diff parses two workbooks and produces a deterministic DiffResult: the
// positional cell diff, the cross-sheet dependency graph, authored/computed
// classification, and the cascade (blast radius) from each authored edit.
func Diff(pathA, pathB string) (DiffResult, error) {
	a, err := parseWorkbook(pathA)
	if err != nil {
		return DiffResult{}, fmt.Errorf("parsing %s: %w", pathA, err)
	}
	b, err := parseWorkbook(pathB)
	if err != nil {
		return DiffResult{}, fmt.Errorf("parsing %s: %w", pathB, err)
	}

	// --- Dependency graph (built from the NEW workbook's formulas) ---
	// deps: cell -> precedents it references. dependents: the inverted graph.
	deps := map[string][]string{}
	dependents := map[string][]string{}
	for sheet, cm := range b.cells {
		for coord, cd := range cm {
			if cd.formula == "" {
				continue
			}
			key := qualify(sheet, coord)
			pre := precedents(sheet, cd.formula)
			deps[key] = pre
			for _, p := range pre {
				dependents[p] = append(dependents[p], key)
			}
		}
	}
	for k := range dependents {
		dependents[k] = sortedUnique(dependents[k])
	}

	// --- Positional diff ---
	sheetOrder := unionSheetOrder(a, b)
	var sheetDiffs []SheetDiff
	changeByKey := map[string]*CellChange{} // qualified ref -> change record
	var authoredKeys []string
	authoredCount, computedCount := 0, 0

	for _, sheet := range sheetOrder {
		var changes []CellChange
		for _, coord := range unionCoords(a.cells[sheet], b.cells[sheet]) {
			ad, aOK := a.cells[sheet][coord]
			bd, bOK := b.cells[sheet][coord]
			if !aOK && !bOK {
				continue
			}
			if ad.rawValue == bd.rawValue && ad.formula == bd.formula {
				continue // unchanged
			}

			col, row, _ := excelize.CellNameToCoordinates(coord)
			key := qualify(sheet, coord)
			class := classify(ad.formula, bd.formula)

			cc := CellChange{
				Coord:          coord,
				Row:            row,
				Col:            col,
				Label:          strPtr(displaySide(a, b, sheet, "A"+strconv.Itoa(row))),
				Classification: class,
				OldValue:       typedValue(ad.rawValue),
				NewValue:       typedValue(bd.rawValue),
				OldFormula:     strPtr(ad.formula),
				NewFormula:     strPtr(bd.formula),
				DisplayFormat:  chooseFormat(bd, ad),
				DependsOn:      nonNil(deps[key]),
				Dependents:     nonNil(dependents[key]),
				CausedBy:       []string{},
				Magnitude:      magnitude(ad.rawValue, bd.rawValue),
			}
			changes = append(changes, cc)

			if class == "authored" {
				authoredCount++
				authoredKeys = append(authoredKeys, key)
			} else {
				computedCount++
			}
		}
		if len(changes) == 0 {
			continue
		}
		sort.Slice(changes, func(i, j int) bool {
			if changes[i].Row != changes[j].Row {
				return changes[i].Row < changes[j].Row
			}
			return changes[i].Col < changes[j].Col
		})
		sd := SheetDiff{
			Name:         sheet,
			Changes:      changes,
			RowsInserted: []int{},
			RowsDeleted:  []int{},
		}
		sheetDiffs = append(sheetDiffs, sd)
	}

	// Index every change by its qualified ref (pointers into sheetDiffs).
	for i := range sheetDiffs {
		for j := range sheetDiffs[i].Changes {
			cc := &sheetDiffs[i].Changes[j]
			changeByKey[qualify(sheetDiffs[i].Name, cc.Coord)] = cc
		}
	}

	// --- Cascade: BFS from each authored edit through the dependents graph ---
	sort.Strings(authoredKeys)
	var cascades []Cascade
	for _, origin := range authoredKeys {
		affected := bfsAffected(origin, dependents)
		sort.Strings(affected)

		// Trace causation: every reachable computed change was caused by this edit.
		for _, ref := range affected {
			if cc, ok := changeByKey[ref]; ok && cc.Classification == "computed" {
				cc.CausedBy = sortedUnique(append(cc.CausedBy, origin))
			}
		}

		originCC := changeByKey[origin]
		cascades = append(cascades, Cascade{
			Origin:        origin,
			OriginLabel:   originLabel(originCC),
			OldValue:      originValue(originCC, true),
			NewValue:      originValue(originCC, false),
			AffectedCount: len(affected),
			Affected:      nonNil(affected),
			TopMovers:     topMovers(affected, changeByKey),
		})
	}

	// --- Anomalies (rule-based) ---
	anomalies := detectAnomalies(sheetDiffs)

	// --- Summary ---
	var sheetsAffected []string
	for _, sd := range sheetDiffs {
		sheetsAffected = append(sheetsAffected, sd.Name)
	}

	return DiffResult{
		SchemaVersion: 1,
		From:          versionMeta(pathA),
		To:            versionMeta(pathB),
		Summary: Summary{
			AuthoredCount:  authoredCount,
			ComputedCount:  computedCount,
			SheetsAffected: nonNil(sheetsAffected),
			Narrative:      nil, // AI step adds this later; engine is correct with nil.
		},
		Sheets:    nonNil(sheetDiffs),
		Cascades:  nonNil(cascades),
		Anomalies: nonNil(anomalies),
	}, nil
}

// parseWorkbook reads every populated cell into raw value + formula + format.
func parseWorkbook(path string) (*workbook, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	wb := &workbook{
		sheetOrder: f.GetSheetList(),
		cells:      map[string]map[string]cellData{},
	}
	for _, sheet := range wb.sheetOrder {
		rows, err := f.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("reading sheet %q: %w", sheet, err)
		}
		cm := map[string]cellData{}
		for r := 1; r <= len(rows); r++ {
			for c := 1; c <= len(rows[r-1]); c++ {
				coord, _ := excelize.CoordinatesToCellName(c, r)
				raw, _ := f.GetCellValue(sheet, coord, excelize.Options{RawCellValue: true})
				formula, _ := f.GetCellFormula(sheet, coord)
				if raw == "" && formula == "" {
					continue
				}
				cm[coord] = cellData{
					rawValue: raw,
					formula:  normalizeFormula(formula),
					format:   displayFormat(f, sheet, coord),
				}
			}
		}
		wb.cells[sheet] = cm
	}
	return wb, nil
}

// normalizeFormula ensures a non-empty formula carries a single leading "=",
// so old/new comparison and formula<->constant detection are consistent.
func normalizeFormula(formula string) string {
	if formula == "" {
		return ""
	}
	return "=" + strings.TrimPrefix(formula, "=")
}

// displayFormat returns the cell's number-format code, or "General". It handles
// both custom format strings (e.g. openpyxl-authored "0.0%") and Excel's
// built-in format IDs (e.g. 9="0%", 44=currency) that files formatted with
// Excel's toolbar buttons use — without the built-in table, those would fall
// back to "General" and the UI/narrator would render raw floats.
func displayFormat(f *excelize.File, sheet, coord string) string {
	styleID, err := f.GetCellStyle(sheet, coord)
	if err != nil {
		return "General"
	}
	style, err := f.GetStyle(styleID)
	if err != nil || style == nil {
		return "General"
	}
	if style.CustomNumFmt != nil {
		return *style.CustomNumFmt
	}
	if code, ok := builtinNumFmt[style.NumFmt]; ok {
		return code
	}
	return "General"
}

// builtinNumFmt maps the ECMA-376 reserved built-in number-format IDs (0–49) to
// their format codes. Only the IDs finance models actually use are included
// (number, percent, currency, accounting); date/time built-ins are deliberately
// omitted since these models don't diff dates, and anything absent falls back to
// "General".
var builtinNumFmt = map[int]string{
	0:  "General",
	1:  "0",
	2:  "0.00",
	3:  "#,##0",
	4:  "#,##0.00",
	5:  "$#,##0_);($#,##0)",
	6:  "$#,##0_);[Red]($#,##0)",
	7:  "$#,##0.00_);($#,##0.00)",
	8:  "$#,##0.00_);[Red]($#,##0.00)",
	9:  "0%",
	10: "0.00%",
	11: "0.00E+00",
	37: "#,##0_);(#,##0)",
	38: "#,##0_);[Red](#,##0)",
	39: "#,##0.00_);(#,##0.00)",
	40: "#,##0.00_);[Red](#,##0.00)",
	41: `_(* #,##0_);_(* \(#,##0\);_(* "-"_);_(@_)`,
	42: `_($* #,##0_);_($* \(#,##0\);_($* "-"_);_(@_)`,
	43: `_(* #,##0.00_);_(* \(#,##0.00\);_(* "-"??_);_(@_)`,
	44: `_($* #,##0.00_);_($* \(#,##0.00\);_($* "-"??_);_(@_)`,
	49: "@",
}

// classify decides authored vs computed from the old/new formula strings.
//
//	authored  — formula string changed, a formula became a constant (or vice
//	            versa), or a constant's value changed (both sides constant).
//	computed  — formula identical and non-empty; the value moved because an
//	            upstream dependency moved.
func classify(oldFormula, newFormula string) string {
	if oldFormula != newFormula {
		return "authored"
	}
	if oldFormula == "" {
		return "authored" // both constants, value differs -> a human typed it
	}
	return "computed" // same formula, value moved
}

// precedents parses a formula's referenced cells, fully qualified and
// canonicalized. Same-sheet refs get the current sheet; absolute refs are
// normalized ($B$11 -> B11); ranges are expanded to individual cells.
func precedents(currentSheet, formula string) []string {
	if formula == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(rawRef string) {
		for _, ref := range canonRefs(currentSheet, rawRef) {
			if !seen[ref] {
				seen[ref] = true
				out = append(out, ref)
			}
		}
	}

	// Primary: efp tokenization (handles same-sheet refs and function args).
	parser := efp.ExcelParser()
	for _, t := range parser.Parse(strings.TrimPrefix(formula, "=")) {
		if t.TType == efp.TokenTypeOperand && t.TSubType == efp.TokenSubTypeRange {
			add(t.TValue)
		}
	}
	// Fallback: regex catches cross-sheet refs efp may split (esp. quoted names).
	for _, m := range crossRefRegex.FindAllString(formula, -1) {
		add(m)
	}

	sort.Strings(out)
	return out
}

// crossRefRegex matches Sheet!A1, 'Sheet Name'!A1, and range forms, with
// optional absolute markers.
var crossRefRegex = regexp.MustCompile(`(?:'[^']+'|[A-Za-z_][A-Za-z0-9_.]*)!\$?[A-Z]+\$?[0-9]+(?::\$?[A-Z]+\$?[0-9]+)?`)

var singleCellRe = regexp.MustCompile(`^[A-Z]+[0-9]+$`)

// canonRefs canonicalizes one raw reference token into one or more fully
// qualified cell refs (a range expands to many). Returns nil if not a ref.
func canonRefs(currentSheet, raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	sheetPart := currentSheet
	cellPart := raw
	if i := strings.LastIndex(raw, "!"); i >= 0 {
		sheetPart = strings.TrimSpace(raw[:i])
		cellPart = raw[i+1:]
		if len(sheetPart) >= 2 && strings.HasPrefix(sheetPart, "'") && strings.HasSuffix(sheetPart, "'") {
			sheetPart = strings.ReplaceAll(sheetPart[1:len(sheetPart)-1], "''", "'")
		}
	}
	cellPart = strings.ToUpper(strings.ReplaceAll(cellPart, "$", ""))

	if strings.Contains(cellPart, ":") {
		cells := expandRange(cellPart)
		out := make([]string, 0, len(cells))
		for _, cc := range cells {
			out = append(out, canonSheet(sheetPart)+"!"+cc)
		}
		return out
	}
	if !singleCellRe.MatchString(cellPart) {
		return nil
	}
	return []string{canonSheet(sheetPart) + "!" + cellPart}
}

// canonSheet quotes a sheet name only if it isn't a bare identifier (matching
// the contract: Assumptions, Returns stay bare; 'P&L' is quoted).
func canonSheet(name string) string {
	needsQuote := name == "" || (name[0] >= '0' && name[0] <= '9')
	for _, r := range name {
		if !(r == '_' || r == '.' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			needsQuote = true
			break
		}
	}
	if needsQuote {
		return "'" + strings.ReplaceAll(name, "'", "''") + "'"
	}
	return name
}

// expandRange turns "B4:B9" into individual coords.
func expandRange(rng string) []string {
	parts := strings.SplitN(rng, ":", 2)
	if len(parts) != 2 {
		return nil
	}
	c1, r1, err1 := excelize.CellNameToCoordinates(parts[0])
	c2, r2, err2 := excelize.CellNameToCoordinates(parts[1])
	if err1 != nil || err2 != nil {
		return nil
	}
	if c1 > c2 {
		c1, c2 = c2, c1
	}
	if r1 > r2 {
		r1, r2 = r2, r1
	}
	var out []string
	for c := c1; c <= c2; c++ {
		for r := r1; r <= r2; r++ {
			n, _ := excelize.CoordinatesToCellName(c, r)
			out = append(out, n)
		}
	}
	return out
}

// bfsAffected returns every cell reachable from origin through the dependents
// graph (the full downstream blast radius), excluding origin itself.
func bfsAffected(origin string, dependents map[string][]string) []string {
	visited := map[string]bool{}
	var out []string
	queue := append([]string(nil), dependents[origin]...)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] || cur == origin {
			continue
		}
		visited[cur] = true
		out = append(out, cur)
		queue = append(queue, dependents[cur]...)
	}
	return out
}

// topMovers returns the affected changes with the largest |magnitude|, capped
// at 10 (the AI narrates / the UI headlines these).
func topMovers(affected []string, changeByKey map[string]*CellChange) []Mover {
	var movers []Mover
	for _, ref := range affected {
		cc, ok := changeByKey[ref]
		if !ok || cc.Magnitude == nil {
			continue
		}
		movers = append(movers, Mover{
			Ref:       ref,
			Label:     cc.Label,
			OldValue:  cc.OldValue,
			NewValue:  cc.NewValue,
			Magnitude: cc.Magnitude,
		})
	}
	sort.Slice(movers, func(i, j int) bool {
		mi, mj := math.Abs(*movers[i].Magnitude), math.Abs(*movers[j].Magnitude)
		if mi != mj {
			return mi > mj
		}
		return movers[i].Ref < movers[j].Ref
	})
	if len(movers) > 10 {
		movers = movers[:10]
	}
	if movers == nil {
		return []Mover{}
	}
	return movers
}

// detectAnomalies runs the cheap rule-based smell detectors.
func detectAnomalies(sheets []SheetDiff) []Anomaly {
	var out []Anomaly
	for _, sd := range sheets {
		for i := range sd.Changes {
			cc := sd.Changes[i]
			ref := qualify(sd.Name, cc.Coord)

			// formula_replaced_by_constant: a live calc overwritten with a typed
			// number. Require NewValue != nil so a *deleted* formula cell (or a
			// positional-diff artifact from an inserted/shifted row) doesn't
			// spuriously flag as "replaced with the hardcoded value null/0".
			if cc.OldFormula != nil && cc.NewFormula == nil && cc.NewValue != nil {
				out = append(out, Anomaly{
					Type:       "formula_replaced_by_constant",
					Ref:        ref,
					Label:      cc.Label,
					Severity:   "high",
					Message:    fmt.Sprintf("Formula '%s' was replaced with the hardcoded value %s.", *cc.OldFormula, valueString(cc.NewValue)),
					OldFormula: cc.OldFormula,
					NewValue:   cc.NewValue,
				})
			}

			// large_magnitude_change: a big swing on a computed cell.
			if cc.Classification == "computed" && cc.Magnitude != nil && math.Abs(*cc.Magnitude) > 0.5 {
				out = append(out, Anomaly{
					Type:     "large_magnitude_change",
					Ref:      ref,
					Label:    cc.Label,
					Severity: "medium",
					Message:  fmt.Sprintf("Computed value moved %.1f%% (%s -> %s).", *cc.Magnitude*100, valueString(cc.OldValue), valueString(cc.NewValue)),
				})
			}
		}
	}
	return out
}

// --- small helpers ---

// qualify builds a fully-qualified ref key, quoting the sheet name as needed.
func qualify(sheet, coord string) string {
	return canonSheet(sheet) + "!" + coord
}

// typedValue converts a raw stored string into a typed JSON value.
func typedValue(raw string) any {
	if raw == "" {
		return nil
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f
	}
	// Booleans arrive as raw "1"/"0" under RawCellValue and are parsed as floats
	// above, so there's no "TRUE"/"FALSE" case to handle here.
	return raw
}

// strPtr wraps a non-empty string in a pointer, or nil for the empty string.
// Used for optional contract fields (formula strings, row labels).
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// magnitude computes (new-old)/abs(old) for numeric cells, rounded to 4dp.
// Returns nil for non-numeric values or when old == 0.
func magnitude(oldRaw, newRaw string) *float64 {
	oldF, err1 := strconv.ParseFloat(oldRaw, 64)
	newF, err2 := strconv.ParseFloat(newRaw, 64)
	if err1 != nil || err2 != nil || oldF == 0 {
		return nil
	}
	m := round4((newF - oldF) / math.Abs(oldF))
	return &m
}

func round4(x float64) float64 { return math.Round(x*1e4) / 1e4 }

// chooseFormat prefers the new cell's format, falling back to the old.
func chooseFormat(newCell, oldCell cellData) string {
	if newCell.format != "" {
		return newCell.format
	}
	if oldCell.format != "" {
		return oldCell.format
	}
	return "General"
}

// displaySide returns the raw value of a helper cell (e.g. a row label), from
// the new workbook if present, else the old.
func displaySide(a, b *workbook, sheet, coord string) string {
	if cd, ok := b.cells[sheet][coord]; ok && cd.rawValue != "" {
		return cd.rawValue
	}
	if cd, ok := a.cells[sheet][coord]; ok {
		return cd.rawValue
	}
	return ""
}

func originLabel(cc *CellChange) *string {
	if cc == nil {
		return nil
	}
	return cc.Label
}

func originValue(cc *CellChange, old bool) any {
	if cc == nil {
		return nil
	}
	if old {
		return cc.OldValue
	}
	return cc.NewValue
}

// valueString renders a typed value for anomaly messages.
func valueString(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

// versionMeta builds a basic VersionMeta from a path (a manifest fills real
// label/author/committedAt later).
func versionMeta(path string) VersionMeta {
	return VersionMeta{
		Label:       filepath.Base(path),
		Path:        path,
		CommittedAt: "",
		Author:      "",
	}
}

func unionSheetOrder(a, b *workbook) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range b.sheetOrder {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range a.sheetOrder {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func unionCoords(a, b map[string]cellData) []string {
	seen := map[string]bool{}
	var out []string
	for c := range a {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	for c := range b {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

func sortedUnique(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

// nonNil coerces a nil slice to an empty one so it marshals to a JSON []
// rather than null — the contract requires empty arrays, not null.
func nonNil[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return xs
}
