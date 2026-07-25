package narrator

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"argus/engine"
)

// Recording wraps any Narrator and appends every successful (prompt, narration)
// pair to a JSONL file. It changes nothing about narration itself — it is a
// pure side-channel.
//
// WHY: these pairs are the training set for a future local model. The narration
// task is narrow (a grounded fact block in, 2-3 sentences out), which is exactly
// the shape a sub-1B model can be distilled onto — but only if the examples were
// captured. Every `--narrate` run and every `make fixtures` regeneration is a
// free example, and one that cannot be reconstructed after the fact.
//
// It wraps rather than living inside an adapter so it survives swapping
// ClaudeCLI for a self-hosted OpenAI-compatible endpoint later, and it composes
// with Fallback the same way (Recording{Inner: Fallback{...}} records whichever
// narrator actually answered).
type Recording struct {
	// Inner is the narrator doing the real work. A nil Inner narrates to "".
	Inner Narrator
	// Path is the JSONL file to append to. Empty disables recording entirely,
	// so a zero-value Path degrades to a plain pass-through.
	Path string
	// Source optionally tags which adapter produced the narration ("claude-cli",
	// a model name, ...), so a mixed-provenance log stays filterable at
	// fine-tuning time.
	Source string
}

// recordMu serializes appends. Narration happens roughly once per commit, so a
// single package-level lock is cheap, and it keeps Recording a plain value type
// like the other narrators in this package.
var recordMu sync.Mutex

// NarrationPair is one JSONL line: the exact grounded prompt that was sent and
// the narration that came back.
type NarrationPair struct {
	At        string `json:"at"`
	Source    string `json:"source,omitempty"`
	Prompt    string `json:"prompt"`
	Narration string `json:"narration"`
}

// Narrate delegates to Inner and records the pair on success. A recording
// failure is deliberately swallowed: collecting training data must never be
// able to break narration, which in turn must never break the diff.
func (r Recording) Narrate(ctx context.Context, d engine.DiffResult) (string, error) {
	if r.Inner == nil {
		return "", nil
	}
	out, err := r.Inner.Narrate(ctx, d)
	if err != nil || out == "" {
		return out, err
	}
	r.record(BuildPrompt(d), out)
	return out, nil
}

func (r Recording) record(prompt, narration string) {
	if r.Path == "" {
		return
	}
	line, err := json.Marshal(NarrationPair{
		At:        time.Now().UTC().Format(time.RFC3339),
		Source:    r.Source,
		Prompt:    prompt,
		Narration: narration,
	})
	if err != nil {
		return
	}

	recordMu.Lock()
	defer recordMu.Unlock()

	f, err := os.OpenFile(r.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}
