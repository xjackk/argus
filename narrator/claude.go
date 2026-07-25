package narrator

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"argus/engine"
)

// ClaudeCLI narrates by shelling out to the local `claude -p` (Claude Code CLI)
// in non-interactive mode. It uses the user's existing Claude auth — no API key
// wiring — which makes it the ideal demo/dev adapter. NOTE: `claude -p` calls
// Anthropic's cloud under the hood, so it needs network at call time; it is not
// local inference. In production this adapter is swapped for a direct-SDK or a
// self-hosted OpenAI-compatible one behind the same Narrator interface.
type ClaudeCLI struct {
	// Bin is the claude executable (default "claude", resolved on PATH).
	Bin string
	// Model optionally pins a model (e.g. "claude-haiku-4-5-20251001" — a small
	// fast model is plenty for narration). Empty uses the CLI default.
	Model string
	// Timeout bounds the call so a stalled request can't hang the UI. The
	// Fallback wrapper turns a timeout into the cached string. Default 8s.
	Timeout time.Duration
}

// Narrate builds the grounded prompt and runs `claude -p`, piping the prompt on
// stdin and the grounding rules via --append-system-prompt.
func (c ClaudeCLI) Narrate(ctx context.Context, d engine.DiffResult) (string, error) {
	bin := c.Bin
	if bin == "" {
		bin = "claude"
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 8 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{"-p", "--append-system-prompt", SystemPrompt, "--output-format", "text"}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader(BuildPrompt(d))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("claude -p timed out after %s", timeout)
		}
		return "", fmt.Errorf("claude -p failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	out := strings.TrimSpace(stdout.String())
	if out == "" {
		return "", fmt.Errorf("claude -p returned empty output")
	}
	return out, nil
}
