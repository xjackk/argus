package spreadsheet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

const fixture = "../engine/testdata/atlas_c06_exit_multiple.xlsx"

// activeSheetName reports which tab a workbook opens on.
func activeSheetName(t *testing.T, path string) string {
	t.Helper()
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	list := f.GetSheetList()
	idx := f.GetActiveSheetIndex()
	if idx < 0 || idx >= len(list) {
		t.Fatalf("active index %d out of range for %v", idx, list)
	}
	return list[idx]
}

// The whole point: the copy opens on the sheet the user clicked.
func TestOpenAtSelectsSheet(t *testing.T) {
	var gotName string
	var gotArgs []string
	o := Opener{
		TempDir: t.TempDir(),
		Run: func(name string, args ...string) error {
			gotName, gotArgs = name, args
			return nil
		},
	}

	app, err := o.OpenAt(fixture, "Debt", "Atlas LBO @ c06")
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	if app == "" {
		t.Error("no application name returned")
	}
	if gotName == "" || len(gotArgs) == 0 {
		t.Fatalf("no launch command issued (%q %v)", gotName, gotArgs)
	}

	opened := gotArgs[len(gotArgs)-1] // the file path is always last
	if got := activeSheetName(t, opened); got != "Debt" {
		t.Errorf("active sheet = %q, want %q", got, "Debt")
	}
}

// The original must never be touched — it is an immutable historical version,
// and setting the active tab is a write.
func TestOpenAtLeavesSourceUnmodified(t *testing.T) {
	before, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	o := Opener{TempDir: t.TempDir(), Run: func(string, ...string) error { return nil }}
	if _, err := o.OpenAt(fixture, "Assumptions", "snap"); err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	after, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("source workbook was modified")
	}
}

// The copy is chmod 0444 so the app opens it read-only and a stray Cmd+S
// cannot write edits into a snapshot.
func TestCopyIsReadOnly(t *testing.T) {
	dir := t.TempDir()
	o := Opener{TempDir: dir, Run: func(string, ...string) error { return nil }}
	if _, err := o.OpenAt(fixture, "Returns", "snap"); err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 copy, got %d", len(entries))
	}
	info, err := entries[0].Info()
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o444 {
		t.Errorf("copy mode = %o, want 444", perm)
	}
}

// Opening twice must not fail on the read-only leftover from the first run.
func TestOpenAtTwiceOverwrites(t *testing.T) {
	o := Opener{TempDir: t.TempDir(), Run: func(string, ...string) error { return nil }}
	for i, sheet := range []string{"Debt", "Returns"} {
		if _, err := o.OpenAt(fixture, sheet, "same label"); err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
	}
}

// An unknown sheet is not worth failing over: still open the workbook.
func TestOpenAtUnknownSheetStillOpens(t *testing.T) {
	called := false
	o := Opener{TempDir: t.TempDir(), Run: func(string, ...string) error { called = true; return nil }}
	if _, err := o.OpenAt(fixture, "NoSuchSheet", "snap"); err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	if !called {
		t.Error("workbook was not opened")
	}
}

func TestDetectAlwaysOffersAFallback(t *testing.T) {
	apps := Detect()
	if len(apps) == 0 {
		t.Fatal("Detect returned nothing; the OS default must always be present")
	}
	last := apps[len(apps)-1]
	if last.Name != "default application" || len(last.argv) == 0 {
		t.Errorf("last entry should be the OS default handler, got %+v", last)
	}
	for _, a := range apps {
		if a.Name == "" || len(a.argv) == 0 {
			t.Errorf("malformed app entry: %+v", a)
		}
	}
}

func TestSafeName(t *testing.T) {
	cases := map[string]string{
		"Atlas LBO @ c06":      "Atlas LBO @ c06",
		"Deal/Model:2026":      "Deal-Model-2026",
		"  ":                   "workbook",
		"":                     "workbook",
		"a<b>c|d?e*f\"g":       "a-b-c-d-e-f-g",
		strings.Repeat("x", 200): strings.Repeat("x", 80),
	}
	for in, want := range cases {
		if got := safeName(in); got != want {
			t.Errorf("safeName(%q) = %q, want %q", in, got, want)
		}
	}
}

// The copy must land in its own directory, never beside the source — a watched
// folder would otherwise ingest it as a new save (ROADMAP §7).
func TestCopyIsNotBesideSource(t *testing.T) {
	dir := t.TempDir()
	o := Opener{TempDir: dir, Run: func(string, ...string) error { return nil }}
	if _, err := o.OpenAt(fixture, "Debt", "snap"); err != nil {
		t.Fatal(err)
	}
	srcDir := filepath.Dir(fixture)
	entries, _ := os.ReadDir(srcDir)
	for _, e := range entries {
		if strings.Contains(e.Name(), "snap") {
			t.Errorf("copy leaked into the source directory: %s", e.Name())
		}
	}
}
