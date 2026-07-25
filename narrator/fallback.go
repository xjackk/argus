package narrator

import (
	"context"

	"argus/engine"
)

// Noop is the default narrator: it produces no narrative, leaving
// summary.narrative=null. The UI renders fine without it.
type Noop struct{}

func (Noop) Narrate(context.Context, engine.DiffResult) (string, error) { return "", nil }

// Static always returns a fixed, pre-generated string. Used as the cached
// safety net behind Fallback, and on its own for a fully offline demo.
type Static struct{ Text string }

func (s Static) Narrate(context.Context, engine.DiffResult) (string, error) {
	return s.Text, nil
}

// Fallback makes live narration uncrashable for a demo: it tries Primary
// (e.g. a live ClaudeCLI call, shown behind a spinner), and if that errors OR
// returns empty, it silently returns Backup's result instead. The audience sees
// the live narrative when it works and never sees a failure when it doesn't.
type Fallback struct {
	Primary Narrator
	Backup  Narrator
}

func (f Fallback) Narrate(ctx context.Context, d engine.DiffResult) (string, error) {
	if f.Primary != nil {
		if out, err := f.Primary.Narrate(ctx, d); err == nil && out != "" {
			return out, nil
		}
	}
	if f.Backup != nil {
		return f.Backup.Narrate(ctx, d)
	}
	return "", nil
}
