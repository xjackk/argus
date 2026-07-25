// Command argus-diff diffs two Excel workbooks and prints the DiffResult JSON.
//
// Usage:
//
//	argus-diff [--narrate] [--prompt-only] <fromPath> <toPath>
//
// It emits indented JSON matching DATA-CONTRACT.md to stdout. The engine never
// recomputes values; it reads Excel's cached values and diffs them.
//
// By default summary.narrative is null (demo-safe: no live model call). With
// --narrate, it fills the narrative via `claude -p`. With --prompt-only, it
// prints the grounded prompt that WOULD be sent (no model call) — handy for
// eyeballing grounding.
//
// Env: ARGUS_NARRATION_LOG=path.jsonl appends every (prompt, narration) pair to
// that file — the training set for a future local narrator. Unset by default.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"argus/engine"
	"argus/narrator"
)

func main() {
	narrate := flag.Bool("narrate", false, "fill summary.narrative via a live `claude -p` call")
	promptOnly := flag.Bool("prompt-only", false, "print the grounded narration prompt and exit (no model call)")
	model := flag.String("model", "", "model for --narrate (default: claude CLI default)")
	flag.Parse()

	if flag.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s [--narrate] [--prompt-only] [--model M] <fromPath> <toPath>\n", os.Args[0])
		os.Exit(2)
	}
	fromPath, toPath := flag.Arg(0), flag.Arg(1)

	result, err := engine.Diff(fromPath, toPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "argus-diff: %v\n", err)
		os.Exit(1)
	}

	if *promptOnly {
		fmt.Print(narrator.BuildPrompt(result))
		return
	}

	if *narrate {
		var n narrator.Narrator = narrator.ClaudeCLI{Model: *model}
		// Opt-in: with ARGUS_NARRATION_LOG set, append every (prompt, narration)
		// pair to that JSONL file. Unset (the default) is an exact no-op.
		if logPath := os.Getenv("ARGUS_NARRATION_LOG"); logPath != "" {
			n = narrator.Recording{Inner: n, Path: logPath, Source: "claude-cli"}
		}
		text, err := n.Narrate(context.Background(), result)
		if err != nil {
			// Non-fatal: narrative stays null, engine output is still valid.
			fmt.Fprintf(os.Stderr, "argus-diff: narration skipped: %v\n", err)
		} else {
			result.Summary.Narrative = &text
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "argus-diff: encoding result: %v\n", err)
		os.Exit(1)
	}
}
