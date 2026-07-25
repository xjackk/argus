package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestWriteFileAtomicBasics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	if err := writeFileAtomic(path, []byte("first"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "first" {
		t.Fatalf("read back %q, %v", b, err)
	}

	// Overwrite with something shorter — the classic truncate hazard.
	if err := writeFileAtomic(path, []byte("2"), 0o644); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "2" {
		t.Errorf("overwrite left %q", b)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("perm = %v, want 0644", fi.Mode().Perm())
	}

	// No temp droppings left in the directory — this dir is served to the
	// browser as /store in the default configuration.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "history.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory should hold only history.json, has %v", names)
	}
}

func TestWriteFileAtomicErrorLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	// A destination directory that does not exist: CreateTemp fails, and
	// nothing may be created anywhere.
	err := writeFileAtomic(filepath.Join(dir, "nope", "history.json"), []byte("x"), 0o644)
	if err == nil {
		t.Fatal("expected an error writing into a missing directory")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("failed write left files behind: %v", entries)
	}
}

// The property that matters: a reader polling the file while it is rewritten
// over and over must never observe a partial write. With os.WriteFile this
// test fails; with rename it cannot.
func TestWriteFileAtomicNeverExposesAPartialFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	// Alternate between a large and a tiny payload so any truncate-in-place
	// window would be wide and obvious.
	big, _ := json.MarshalIndent(History{Commits: makeCommits(400)}, "", "  ")
	small, _ := json.MarshalIndent(History{Commits: makeCommits(1)}, "", "  ")
	if err := writeFileAtomic(path, big, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() { // the writer — the daemon
		defer wg.Done()
		for i := 0; i < 300; i++ {
			payload := big
			if i%2 == 0 {
				payload = small
			}
			if err := writeFileAtomic(path, payload, 0o644); err != nil {
				t.Errorf("writeFileAtomic: %v", err)
				break
			}
		}
		close(stop)
	}()

	for r := 0; r < 4; r++ { // the readers — HTTP clients / the browser
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				b, err := os.ReadFile(path)
				if err != nil {
					// The file must never vanish either — rename replaces it
					// in one step.
					t.Errorf("history.json disappeared mid-write: %v", err)
					return
				}
				var h History
				if err := json.Unmarshal(b, &h); err != nil {
					t.Errorf("read a partial history.json (%d bytes): %v", len(b), err)
					return
				}
				if n := len(h.Commits); n != 1 && n != 400 {
					t.Errorf("read a torn history.json: %d commits", n)
					return
				}
			}
		}()
	}
	wg.Wait()

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func makeCommits(n int) []Commit {
	out := make([]Commit, n)
	for i := range out {
		out[i] = Commit{
			ID:        "c001",
			File:      "atlas.xlsx",
			Author:    "alice",
			Message:   "Updated atlas.xlsx — padding padding padding padding",
			Timestamp: "2026-07-25T21:00:00Z",
		}
	}
	return out
}

// The daemon's own writers must go through the atomic path.
func TestStoreWritesAreAtomic(t *testing.T) {
	folder := t.TempDir()
	store := t.TempDir()
	watched := filepath.Join(folder, "atlas.xlsx")

	d := newDaemon(folder, store, "alice")
	if err := d.ensureDirs(); err != nil {
		t.Fatalf("ensureDirs: %v", err)
	}
	copyInto(t, filepath.Join(testdata, "atlas_v1_base.xlsx"), watched)
	d.capture(watched)
	copyInto(t, filepath.Join(testdata, "atlas_v2_exit_multiple.xlsx"), watched)
	d.capture(watched)
	d.writeHistory()
	d.writeHistory() // rewriting must not accumulate temp files

	for _, dir := range []string{store, filepath.Join(store, "diffs")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("readdir %s: %v", dir, err)
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") || strings.Contains(e.Name(), ".tmp") {
				t.Errorf("%s left a temp file: %s", dir, e.Name())
			}
		}
	}
}
