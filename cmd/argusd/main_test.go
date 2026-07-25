package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// --- unit tests for the pure helpers ---

func TestFileKey(t *testing.T) {
	// Deterministic: same name -> same key.
	if fileKey("/a/b/atlas.xlsx") != fileKey("/somewhere/else/atlas.xlsx") {
		t.Fatal("fileKey must depend only on the base name")
	}
	if fileKey("atlas.xlsx") != fileKey("atlas.xlsx") {
		t.Fatal("fileKey must be deterministic")
	}
	// Distinct per filename.
	if fileKey("atlas.xlsx") == fileKey("model.xlsx") {
		t.Fatal("distinct filenames must produce distinct keys")
	}
	if got := fileKey("atlas.xlsx"); len(got) != 12 { // 6 bytes hex-encoded
		t.Fatalf("unexpected key length %d for %q", len(got), got)
	}
}

func TestIsXlsx(t *testing.T) {
	cases := map[string]bool{
		"model.xlsx":     true,
		"MODEL.XLSX":     true,
		"a.b.xlsx":       true,
		"model.xls":      false,
		"model.csv":      false,
		"model":          false,
		"model.xlsx.tmp": false,
	}
	for name, want := range cases {
		if got := isXlsx(name); got != want {
			t.Errorf("isXlsx(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIsTemp(t *testing.T) {
	cases := map[string]bool{
		"~$model.xlsx":     true,  // Excel lock file
		".~lock.model.xlsx#": true, // LibreOffice lock file
		".hidden.xlsx":     true,  // dotfile
		".DS_Store":        true,
		"model.xlsx":       false,
		"atlas_v1.xlsx":    false,
	}
	for name, want := range cases {
		if got := isTemp(name); got != want {
			t.Errorf("isTemp(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestHashAndSameContent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.bin")
	b := filepath.Join(dir, "b.bin")
	c := filepath.Join(dir, "c.bin")
	mustWrite(t, a, []byte("hello world"))
	mustWrite(t, b, []byte("hello world"))
	mustWrite(t, c, []byte("different"))

	ha, err := hashFile(a)
	if err != nil {
		t.Fatalf("hashFile: %v", err)
	}
	if ha == "" {
		t.Fatal("hashFile returned empty hash")
	}
	if !sameContent(a, b) {
		t.Error("identical content should be sameContent")
	}
	if sameContent(a, c) {
		t.Error("different content should not be sameContent")
	}
	// Missing files must not panic and must be reported unequal.
	if sameContent(a, filepath.Join(dir, "nope.bin")) {
		t.Error("missing file should not be sameContent")
	}
	if _, err := hashFile(filepath.Join(dir, "nope.bin")); err == nil {
		t.Error("hashFile of missing file should error")
	}
}

// --- integration test: drive capture + resume directly ---

const testdata = "../../engine/testdata"

func TestCaptureAndResume(t *testing.T) {
	folder := t.TempDir()
	store := t.TempDir()
	watched := filepath.Join(folder, "atlas.xlsx")

	d := newDaemon(folder, store, "alice")
	if err := d.ensureDirs(); err != nil {
		t.Fatalf("ensureDirs: %v", err)
	}

	// 1) base version
	copyInto(t, filepath.Join(testdata, "atlas_v1_base.xlsx"), watched)
	d.capture(watched)
	// 2) an authored edit + cascade
	copyInto(t, filepath.Join(testdata, "atlas_v2_exit_multiple.xlsx"), watched)
	d.capture(watched)
	// 3) a hardcode override that trips an anomaly
	copyInto(t, filepath.Join(testdata, "atlas_v5_hardcode_override.xlsx"), watched)
	d.capture(watched)
	d.writeHistory()

	if got := len(d.history.Commits); got != 3 {
		t.Fatalf("expected 3 commits, got %d", got)
	}

	c1, c2, c3 := d.history.Commits[0], d.history.Commits[1], d.history.Commits[2]

	if !c1.Base || c1.ID != "c001" {
		t.Errorf("commit 1 should be base c001, got id=%q base=%v", c1.ID, c1.Base)
	}
	if c1.Author != "alice" {
		t.Errorf("commit 1 author = %q, want alice", c1.Author)
	}
	if c2.AuthoredCount != 1 || c2.ComputedCount != 4 {
		t.Errorf("commit 2 counts = %d authored / %d computed, want 1 / 4", c2.AuthoredCount, c2.ComputedCount)
	}
	if c2.Parent != c1.ID {
		t.Errorf("commit 2 parent = %q, want %q", c2.Parent, c1.ID)
	}
	if !c3.Anomaly {
		t.Errorf("commit 3 should flag an anomaly")
	}
	if c3.Parent != c2.ID {
		t.Errorf("commit 3 parent = %q, want %q", c3.Parent, c2.ID)
	}

	// diff files exist for the two non-base commits (not for the base)
	assertFile(t, filepath.Join(store, "diffs", "c002.json"))
	assertFile(t, filepath.Join(store, "diffs", "c003.json"))
	if _, err := os.Stat(filepath.Join(store, "diffs", "c001.json")); err == nil {
		t.Error("base commit should not have a diff file")
	}
	// history.json is on disk and parses
	var onDisk History
	b, err := os.ReadFile(filepath.Join(store, "history.json"))
	if err != nil {
		t.Fatalf("reading history.json: %v", err)
	}
	if err := json.Unmarshal(b, &onDisk); err != nil {
		t.Fatalf("history.json does not parse: %v", err)
	}
	if len(onDisk.Commits) != 3 {
		t.Fatalf("history.json has %d commits, want 3", len(onDisk.Commits))
	}

	// --- RESUME: a new daemon on the same store, different author ---
	d2 := newDaemon(folder, store, "bob")
	if err := d2.ensureDirs(); err != nil {
		t.Fatalf("ensureDirs (resume): %v", err)
	}
	if err := d2.resume(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if len(d2.history.Commits) != 3 {
		t.Fatalf("resume loaded %d commits, want 3", len(d2.history.Commits))
	}
	key := fileKey("atlas.xlsx")
	if d2.lastCommit[key] != "c003" {
		t.Errorf("resume lastCommit = %q, want c003", d2.lastCommit[key])
	}
	if d2.lastSnapshot[key] == "" {
		t.Error("resume did not rebuild lastSnapshot")
	}
	if d2.seq < 3 {
		t.Errorf("resume seq = %d, want >= 3", d2.seq)
	}

	// An unchanged file (still v5 in the folder) must be a no-op, not a dup.
	d2.capture(watched)
	if len(d2.history.Commits) != 3 {
		t.Fatalf("resume+unchanged capture created a duplicate commit: %d", len(d2.history.Commits))
	}

	// A real edit continues the id sequence (c004) and is attributed to bob.
	copyInto(t, filepath.Join(testdata, "atlas_v3_downside.xlsx"), watched)
	d2.capture(watched)
	if len(d2.history.Commits) != 4 {
		t.Fatalf("expected 4 commits after resume edit, got %d", len(d2.history.Commits))
	}
	c4 := d2.history.Commits[3]
	if c4.ID != "c004" {
		t.Errorf("next commit id = %q, want c004 (sequence must continue, not restart)", c4.ID)
	}
	if c4.Author != "bob" {
		t.Errorf("resumed commit author = %q, want bob", c4.Author)
	}
	if c4.Parent != "c003" {
		t.Errorf("resumed commit parent = %q, want c003", c4.Parent)
	}
	if c4.Base {
		t.Error("resumed edit must not be a base commit")
	}
}

// A corrupt (non-xlsx) file must not crash capture — it records a save and
// logs, exercising the diff-error and panic-safe paths.
func TestCaptureCorruptFileDoesNotCrash(t *testing.T) {
	folder := t.TempDir()
	store := t.TempDir()
	watched := filepath.Join(folder, "model.xlsx")

	d := newDaemon(folder, store, "alice")
	if err := d.ensureDirs(); err != nil {
		t.Fatalf("ensureDirs: %v", err)
	}
	mustWrite(t, watched, []byte("this is not a real xlsx"))
	d.capture(watched) // base — no diff, must not crash
	mustWrite(t, watched, []byte("still not a real xlsx, but different"))
	d.capture(watched) // diff attempt fails — must log and continue, no panic

	if len(d.history.Commits) != 2 {
		t.Fatalf("expected 2 commits from corrupt-file captures, got %d", len(d.history.Commits))
	}
}

// A file that vanishes before capture must be skipped cleanly (no commit).
func TestCaptureMissingFileIsSkipped(t *testing.T) {
	folder := t.TempDir()
	store := t.TempDir()
	d := newDaemon(folder, store, "alice")
	if err := d.ensureDirs(); err != nil {
		t.Fatalf("ensureDirs: %v", err)
	}
	d.capture(filepath.Join(folder, "ghost.xlsx")) // never existed
	if len(d.history.Commits) != 0 {
		t.Fatalf("missing file must not create a commit, got %d", len(d.history.Commits))
	}
	if d.seq != 0 {
		t.Errorf("seq must be rolled back on skip, got %d", d.seq)
	}
}

// --- helpers ---

func mustWrite(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func copyInto(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatalf("copy %s -> %s: %v", src, dst, err)
	}
}

func assertFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file %s: %v", path, err)
	}
}
