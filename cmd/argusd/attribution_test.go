package main

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

// makeWorkbook writes a real .xlsx at path with lastModifiedBy stamped into
// docProps/core.xml (empty string = leave the property unset, like the
// machine-generated fixtures).
func makeWorkbook(t *testing.T, path, lastModifiedBy, cell string) {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetCellValue("Sheet1", "A1", cell); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	if lastModifiedBy != "" {
		if err := f.SetDocProps(&excelize.DocProperties{LastModifiedBy: lastModifiedBy}); err != nil {
			t.Fatalf("SetDocProps: %v", err)
		}
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("SaveAs %s: %v", path, err)
	}
}

func TestWorkbookLastModifiedBy(t *testing.T) {
	dir := t.TempDir()

	// A workbook saved by a person — the case that matters.
	named := filepath.Join(dir, "named.xlsx")
	makeWorkbook(t, named, "Deivison de Oliveira", "x")
	if got := workbookLastModifiedBy(named); got != "Deivison de Oliveira" {
		t.Errorf("workbookLastModifiedBy = %q, want %q", got, "Deivison de Oliveira")
	}

	// Whitespace-only is as good as absent — it must not become the author.
	blanky := filepath.Join(dir, "blanky.xlsx")
	makeWorkbook(t, blanky, "   \t ", "x")
	if got := workbookLastModifiedBy(blanky); got != "" {
		t.Errorf("whitespace-only property should read as empty, got %q", got)
	}

	// Surrounding whitespace is trimmed, not preserved.
	padded := filepath.Join(dir, "padded.xlsx")
	makeWorkbook(t, padded, "  M. Rivera  ", "x")
	if got := workbookLastModifiedBy(padded); got != "M. Rivera" {
		t.Errorf("property should be trimmed, got %q", got)
	}
}

// The bundled fixtures were written by openpyxl and carry no author — this is
// the exact case the fallback chain exists for (ROADMAP §5.1).
func TestWorkbookLastModifiedByOnMachineWrittenFixture(t *testing.T) {
	got := workbookLastModifiedBy(filepath.Join(testdata, "atlas_v1_base.xlsx"))
	if got != "" {
		// Not a failure of the code — but the whole fallback design assumes it,
		// so if a fixture ever gains an author we want to hear about it.
		t.Errorf("expected no author on machine-written fixture, got %q", got)
	}
}

// Nothing about a bad file may escape as an error or a panic: attribution is
// best-effort on the capture path.
func TestWorkbookLastModifiedByBadInputIsEmpty(t *testing.T) {
	dir := t.TempDir()

	junk := filepath.Join(dir, "junk.xlsx")
	mustWrite(t, junk, []byte("this is not a real xlsx"))

	// A zip that is not a workbook — gets further into excelize than raw junk.
	emptyZip := filepath.Join(dir, "empty.xlsx")
	mustWrite(t, emptyZip, []byte("PK\x05\x06\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"))

	for _, path := range []string{
		junk,
		emptyZip,
		filepath.Join(dir, "does-not-exist.xlsx"),
		dir, // a directory
		"",
	} {
		if got := workbookLastModifiedBy(path); got != "" {
			t.Errorf("workbookLastModifiedBy(%q) = %q, want empty", path, got)
		}
	}
}

// The fallback chain: workbook → -author → (main wires -author to the OS user).
func TestAuthorForFallbackChain(t *testing.T) {
	dir := t.TempDir()
	named := filepath.Join(dir, "named.xlsx")
	makeWorkbook(t, named, "Priya Raman", "x")
	anon := filepath.Join(dir, "anon.xlsx")
	makeWorkbook(t, anon, "", "x")

	// Mode off — the flag author wins even when the workbook names someone.
	// This is what keeps the pre-existing no-flag path byte-identical.
	off := newDaemon(dir, dir, "flag-author")
	if got := off.authorFor(named); got != "flag-author" {
		t.Errorf("attribution off: authorFor = %q, want flag-author", got)
	}

	on := newDaemon(dir, dir, "flag-author")
	on.attributeFromFile = true
	if got := on.authorFor(named); got != "Priya Raman" {
		t.Errorf("attribution on: authorFor = %q, want Priya Raman", got)
	}
	if got := on.authorFor(anon); got != "flag-author" {
		t.Errorf("attribution on, no property: authorFor = %q, want flag-author", got)
	}
	if got := on.authorFor(filepath.Join(dir, "gone.xlsx")); got != "flag-author" {
		t.Errorf("attribution on, missing file: authorFor = %q, want flag-author", got)
	}
}

// End to end through capture: two different people saving into the same
// watched folder must produce two differently-attributed commits from ONE
// daemon — the bug that a single process-level -author cannot express.
func TestCaptureAttributesPerSave(t *testing.T) {
	folder := t.TempDir()
	store := t.TempDir()
	watched := filepath.Join(folder, "model.xlsx")

	d := newDaemon(folder, store, "the-service-account")
	d.attributeFromFile = true
	if err := d.ensureDirs(); err != nil {
		t.Fatalf("ensureDirs: %v", err)
	}

	makeWorkbook(t, watched, "Priya Raman", "first")
	d.capture(watched)
	makeWorkbook(t, watched, "Tom Okafor", "second")
	d.capture(watched)
	makeWorkbook(t, watched, "", "third") // a machine-written save
	d.capture(watched)

	if len(d.history.Commits) != 3 {
		t.Fatalf("expected 3 commits, got %d", len(d.history.Commits))
	}
	want := []string{"Priya Raman", "Tom Okafor", "the-service-account"}
	for i, w := range want {
		if got := d.history.Commits[i].Author; got != w {
			t.Errorf("commit %d author = %q, want %q", i+1, got, w)
		}
	}
}

// With the mode off, the same sequence is attributed the old way — every
// commit gets the -author flag, regardless of what the workbooks say.
func TestCaptureAttributionOffKeepsFlagAuthor(t *testing.T) {
	folder := t.TempDir()
	store := t.TempDir()
	watched := filepath.Join(folder, "model.xlsx")

	d := newDaemon(folder, store, "alice") // attributeFromFile stays false
	if err := d.ensureDirs(); err != nil {
		t.Fatalf("ensureDirs: %v", err)
	}
	makeWorkbook(t, watched, "Priya Raman", "first")
	d.capture(watched)
	makeWorkbook(t, watched, "Tom Okafor", "second")
	d.capture(watched)

	for i, c := range d.history.Commits {
		if c.Author != "alice" {
			t.Errorf("commit %d author = %q, want alice (attribution is off)", i+1, c.Author)
		}
	}
}
