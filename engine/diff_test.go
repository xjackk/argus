package engine

import (
	"math"
	"reflect"
	"testing"
)

// testWorkbookDir points at fixtures committed under engine/testdata so the
// suite runs on any machine / CI (no absolute home path).
const testWorkbookDir = "testdata/"

// TestAcceptanceV1toV2 encodes the DATA-CONTRACT acceptance test for
// atlas_v1_base.xlsx -> atlas_v2_exit_multiple.xlsx. It is the regression net
// for the whole engine.
func TestAcceptanceV1toV2(t *testing.T) {
	res, err := Diff(
		testWorkbookDir+"atlas_v1_base.xlsx",
		testWorkbookDir+"atlas_v2_exit_multiple.xlsx",
	)
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	// summary.authoredCount == 1
	if res.Summary.AuthoredCount != 1 {
		t.Errorf("authoredCount = %d, want 1", res.Summary.AuthoredCount)
	}

	// Assumptions sheet changes contain coord "B5", classification "authored",
	// oldValue 10.5, newValue 9.5.
	b5 := findChange(res, "Assumptions", "B5")
	if b5 == nil {
		t.Fatal("expected a change at Assumptions!B5, found none")
	}
	if b5.Classification != "authored" {
		t.Errorf("Assumptions!B5 classification = %q, want %q", b5.Classification, "authored")
	}
	if !floatEquals(b5.OldValue, 10.5, 1e-9) {
		t.Errorf("Assumptions!B5 oldValue = %v, want 10.5", b5.OldValue)
	}
	if !floatEquals(b5.NewValue, 9.5, 1e-9) {
		t.Errorf("Assumptions!B5 newValue = %v, want 9.5", b5.NewValue)
	}

	// exactly 1 cascade, origin "Assumptions!B5"
	if len(res.Cascades) != 1 {
		t.Fatalf("len(cascades) = %d, want 1", len(res.Cascades))
	}
	casc := res.Cascades[0]
	if casc.Origin != "Assumptions!B5" {
		t.Errorf("cascade origin = %q, want %q", casc.Origin, "Assumptions!B5")
	}

	// that cascade's affected includes Returns!B9, B11, B13, B14
	for _, want := range []string{"Returns!B9", "Returns!B11", "Returns!B13", "Returns!B14"} {
		if !contains(casc.Affected, want) {
			t.Errorf("cascade.affected missing %q (got %v)", want, casc.Affected)
		}
	}

	// Returns!B14 (IRR): classification "computed", oldValue ~= 0.2746,
	// newValue ~= 0.2454, causedBy ["Assumptions!B5"].
	b14 := findChange(res, "Returns", "B14")
	if b14 == nil {
		t.Fatal("expected a change at Returns!B14, found none")
	}
	if b14.Classification != "computed" {
		t.Errorf("Returns!B14 classification = %q, want %q", b14.Classification, "computed")
	}
	if !floatEquals(b14.OldValue, 0.2746, 1e-3) {
		t.Errorf("Returns!B14 oldValue = %v, want ~0.2746", b14.OldValue)
	}
	if !floatEquals(b14.NewValue, 0.2454, 1e-3) {
		t.Errorf("Returns!B14 newValue = %v, want ~0.2454", b14.NewValue)
	}
	if !reflect.DeepEqual(b14.CausedBy, []string{"Assumptions!B5"}) {
		t.Errorf("Returns!B14 causedBy = %v, want [\"Assumptions!B5\"]", b14.CausedBy)
	}
}

// TestHardcodeAnomalyV5 covers the formula_replaced_by_constant detector
// (atlas_v5: Returns!B9 Exit EV formula overwritten with a hardcoded 2100).
func TestHardcodeAnomalyV5(t *testing.T) {
	res, err := Diff(testWorkbookDir+"atlas_v1_base.xlsx", testWorkbookDir+"atlas_v5_hardcode_override.xlsx")
	if err != nil {
		t.Fatalf("Diff error: %v", err)
	}
	var found *Anomaly
	for i := range res.Anomalies {
		if res.Anomalies[i].Type == "formula_replaced_by_constant" && res.Anomalies[i].Ref == "Returns!B9" {
			found = &res.Anomalies[i]
		}
	}
	if found == nil {
		t.Fatalf("expected formula_replaced_by_constant on Returns!B9, got %+v", res.Anomalies)
	}
	if found.Severity != "high" {
		t.Errorf("severity = %q, want high", found.Severity)
	}
	if !floatEquals(found.NewValue, 2100, 1e-9) {
		t.Errorf("anomaly newValue = %v, want 2100", found.NewValue)
	}
}

// TestMultiInputCausationV3 covers multiple authored inputs with overlapping
// cascades (atlas_v3: growth, margin, interest all change).
func TestMultiInputCausationV3(t *testing.T) {
	res, err := Diff(testWorkbookDir+"atlas_v1_base.xlsx", testWorkbookDir+"atlas_v3_downside.xlsx")
	if err != nil {
		t.Fatalf("Diff error: %v", err)
	}
	if res.Summary.AuthoredCount != 3 {
		t.Errorf("authoredCount = %d, want 3", res.Summary.AuthoredCount)
	}
	if len(res.Cascades) != 3 {
		t.Errorf("len(cascades) = %d, want 3", len(res.Cascades))
	}
	// IRR is computed and must trace back to at least one authored origin.
	b14 := findChange(res, "Returns", "B14")
	if b14 == nil {
		t.Fatal("expected a change at Returns!B14")
	}
	if b14.Classification != "computed" {
		t.Errorf("Returns!B14 classification = %q, want computed", b14.Classification)
	}
	if len(b14.CausedBy) == 0 {
		t.Errorf("Returns!B14 causedBy is empty, want >=1 authored origin")
	}
}

// TestAuthoredCellsHaveNoCause asserts authored cells carry an empty causedBy
// (the cause chain only applies to computed ripples).
func TestAuthoredCellsHaveNoCause(t *testing.T) {
	res, err := Diff(testWorkbookDir+"atlas_v1_base.xlsx", testWorkbookDir+"atlas_v2_exit_multiple.xlsx")
	if err != nil {
		t.Fatalf("Diff error: %v", err)
	}
	b5 := findChange(res, "Assumptions", "B5")
	if b5 == nil {
		t.Fatal("expected Assumptions!B5")
	}
	if len(b5.CausedBy) != 0 {
		t.Errorf("authored cell causedBy = %v, want empty", b5.CausedBy)
	}
}

// TestUnlabeledWorkbook proves the engine doesn't depend on the atlas structure:
// a workbook with NO column-A labels (headers in row 1, data in a matrix) still
// diffs, classifies, and cascades correctly — labels just come back nil.
func TestUnlabeledWorkbook(t *testing.T) {
	res, err := Diff(testWorkbookDir+"unlabeled_v1.xlsx", testWorkbookDir+"unlabeled_v2.xlsx")
	if err != nil {
		t.Fatalf("Diff error: %v", err)
	}
	// One input (B2) changed; every formula cell recomputed.
	if res.Summary.AuthoredCount != 1 {
		t.Errorf("authoredCount = %d, want 1", res.Summary.AuthoredCount)
	}
	if res.Summary.ComputedCount == 0 {
		t.Errorf("computedCount = 0, want > 0 (formula cells should recompute)")
	}
	// The changed input is authored with the right value move...
	b2 := findChange(res, "Sheet1", "B2")
	if b2 == nil || b2.Classification != "authored" {
		t.Fatalf("B2 = %+v, want an authored change", b2)
	}
	if !floatEquals(b2.OldValue, 1000, 1e-9) || !floatEquals(b2.NewValue, 1200, 1e-9) {
		t.Errorf("B2 %v→%v, want 1000→1200", b2.OldValue, b2.NewValue)
	}
	// ...and every changed cell has a nil label (no column-A labels), which must
	// NOT break anything — a downstream computed cell still traces its cause.
	c2 := findChange(res, "Sheet1", "C2")
	if c2 == nil {
		t.Fatal("expected a change at Sheet1!C2")
	}
	if c2.Label != nil {
		t.Errorf("C2 label = %v, want nil (unlabeled workbook)", *c2.Label)
	}
	if c2.Classification != "computed" || len(c2.CausedBy) == 0 {
		t.Errorf("C2 should be computed with a cause; got %s causedBy=%v", c2.Classification, c2.CausedBy)
	}
}

// --- helpers ---

func findChange(res DiffResult, sheet, coord string) *CellChange {
	for i := range res.Sheets {
		if res.Sheets[i].Name != sheet {
			continue
		}
		for j := range res.Sheets[i].Changes {
			if res.Sheets[i].Changes[j].Coord == coord {
				return &res.Sheets[i].Changes[j]
			}
		}
	}
	return nil
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func floatEquals(v any, want, tol float64) bool {
	f, ok := v.(float64)
	if !ok {
		return false
	}
	return math.Abs(f-want) <= tol
}
