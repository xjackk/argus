package engine

import (
	"math"
	"reflect"
	"testing"
)

const testWorkbookDir = "/Users/emo/Downloads/argus-files/test-workbooks/"

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
