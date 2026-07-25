package narrator

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"argus/engine"
)

// stubNarrator returns a canned result, standing in for a live adapter.
type stubNarrator struct {
	out string
	err error
}

func (s stubNarrator) Narrate(context.Context, engine.DiffResult) (string, error) {
	return s.out, s.err
}

// sampleDiff is a minimal DiffResult with one authored change, so BuildPrompt
// produces a non-empty grounded prompt to record.
func sampleDiff() engine.DiffResult {
	label := "Exit EV / EBITDA"
	return engine.DiffResult{
		Sheets: []engine.SheetDiff{{
			Name: "Assumptions",
			Changes: []engine.CellChange{{
				Coord: "B5", Label: &label, Classification: "authored",
				OldValue: 10.5, NewValue: 9.5, DisplayFormat: `0.0\x`,
			}},
		}},
	}
}

func readPairs(t *testing.T, path string) []NarrationPair {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []NarrationPair
	for line := range strings.SplitSeq(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var p NarrationPair
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			t.Fatalf("bad JSONL line %q: %v", line, err)
		}
		out = append(out, p)
	}
	return out
}

func TestRecordingAppendsPair(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pairs.jsonl")
	r := Recording{
		Inner:  stubNarrator{out: "Exit multiple lowered 10.5x to 9.5x."},
		Path:   path,
		Source: "test",
	}

	// Two calls must append two lines, not overwrite.
	for range 2 {
		got, err := r.Narrate(context.Background(), sampleDiff())
		if err != nil {
			t.Fatalf("Narrate: %v", err)
		}
		if got != "Exit multiple lowered 10.5x to 9.5x." {
			t.Errorf("narration not passed through: %q", got)
		}
	}

	pairs := readPairs(t, path)
	if len(pairs) != 2 {
		t.Fatalf("got %d recorded pairs, want 2", len(pairs))
	}
	if pairs[0].Source != "test" {
		t.Errorf("Source = %q, want %q", pairs[0].Source, "test")
	}
	if pairs[0].At == "" {
		t.Error("At timestamp is empty")
	}
	// The recorded prompt must be the real grounded prompt, since that is what a
	// fine-tune would train against.
	if !strings.Contains(pairs[0].Prompt, "10.5x → 9.5x") {
		t.Errorf("recorded prompt is not the grounded prompt:\n%s", pairs[0].Prompt)
	}
	if pairs[0].Narration != "Exit multiple lowered 10.5x to 9.5x." {
		t.Errorf("recorded narration = %q", pairs[0].Narration)
	}
}

// A failed or empty narration is not training data — recording it would teach a
// model to emit nothing.
func TestRecordingSkipsFailuresAndEmpty(t *testing.T) {
	cases := map[string]stubNarrator{
		"error": {err: errors.New("boom")},
		"empty": {out: ""},
	}
	for name, stub := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pairs.jsonl")
			r := Recording{Inner: stub, Path: path}
			_, _ = r.Narrate(context.Background(), sampleDiff())
			if pairs := readPairs(t, path); len(pairs) != 0 {
				t.Errorf("recorded %d pairs, want 0", len(pairs))
			}
		})
	}
}

// Recording is a side-channel: if the log cannot be written, narration must
// still succeed.
func TestRecordingSurvivesUnwritablePath(t *testing.T) {
	r := Recording{
		Inner: stubNarrator{out: "fine"},
		Path:  filepath.Join(t.TempDir(), "no-such-dir", "pairs.jsonl"),
	}
	got, err := r.Narrate(context.Background(), sampleDiff())
	if err != nil || got != "fine" {
		t.Fatalf("Narrate = (%q, %v), want (\"fine\", nil)", got, err)
	}
}

// An empty Path disables recording, so Recording degrades to a pass-through.
func TestRecordingEmptyPathIsPassthrough(t *testing.T) {
	r := Recording{Inner: stubNarrator{out: "fine"}}
	got, err := r.Narrate(context.Background(), sampleDiff())
	if err != nil || got != "fine" {
		t.Fatalf("Narrate = (%q, %v), want (\"fine\", nil)", got, err)
	}
}

// Composes with Fallback: the narrator that actually answered is what lands in
// the log.
func TestRecordingWrapsFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pairs.jsonl")
	r := Recording{
		Inner: Fallback{
			Primary: stubNarrator{err: errors.New("primary down")},
			Backup:  Static{Text: "backup prose"},
		},
		Path: path,
	}
	got, err := r.Narrate(context.Background(), sampleDiff())
	if err != nil {
		t.Fatalf("Narrate: %v", err)
	}
	if got != "backup prose" {
		t.Errorf("got %q, want backup prose", got)
	}
	pairs := readPairs(t, path)
	if len(pairs) != 1 || pairs[0].Narration != "backup prose" {
		t.Fatalf("expected the backup's output recorded, got %+v", pairs)
	}
}

func TestRecordingNilInner(t *testing.T) {
	r := Recording{Path: filepath.Join(t.TempDir(), "pairs.jsonl")}
	got, err := r.Narrate(context.Background(), sampleDiff())
	if err != nil || got != "" {
		t.Fatalf("Narrate = (%q, %v), want (\"\", nil)", got, err)
	}
}
