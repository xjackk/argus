package narrator

import (
	"strings"
	"testing"

	"argus/engine"
)

func TestRenderValue(t *testing.T) {
	cases := []struct {
		name   string
		v      any
		format string
		want   string
	}{
		{"percent", 0.2454, "0.0%", "24.5%"},
		{"percent 2dp", 0.274630, "0.00%", "27.46%"},
		{"multiple", 9.5, `0.0\x`, "9.5x"},
		{"multiple 2dp", 2.9932, `0.00\x`, "2.99x"},
		{"currency thousands", 1745.34, `\$#,##0`, "$1,745"},
		{"currency negative", -1745.0, `\$#,##0`, "-$1,745"},
		{"plain float", 3.3645, "General", "3.36"},
		{"string passthrough", "Exit EV", "General", "Exit EV"},
		{"nil", nil, "0.0%", "(empty)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := renderValue(c.v, c.format); got != c.want {
				t.Errorf("renderValue(%v, %q) = %q, want %q", c.v, c.format, got, c.want)
			}
		})
	}
}

func TestWithThousands(t *testing.T) {
	cases := map[string]string{
		"0": "0", "1745": "1,745", "1000000": "1,000,000",
		"-1745": "-1,745", "1745.34": "1,745.34",
	}
	for in, want := range cases {
		if got := withThousands(in); got != want {
			t.Errorf("withThousands(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCanonRef(t *testing.T) {
	cases := map[string]string{
		"'P&L'!B6":       "P&L!B6",
		"Assumptions!B5": "Assumptions!B5",
		"Returns!B14":    "Returns!B14",
		"B9":             "B9",
	}
	for in, want := range cases {
		if got := canonRef(in); got != want {
			t.Errorf("canonRef(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestBuildPromptGroundsMovers guards the ref-quoting fix: a P&L mover's format
// must be recovered (rendered as a percentage), not lost to a bare float.
func TestBuildPromptGroundsMovers(t *testing.T) {
	label := "EBITDA Margin"
	d := engine.DiffResult{
		Sheets: []engine.SheetDiff{{
			Name: "P&L",
			Changes: []engine.CellChange{{
				Coord: "B5", Label: &label, Classification: "computed",
				OldValue: 0.22, NewValue: 0.20, DisplayFormat: "0.0%",
			}},
		}},
		Cascades: []engine.Cascade{{
			Origin: "Assumptions!B12",
			TopMovers: []engine.Mover{{
				Ref: "'P&L'!B5", Label: &label, OldValue: 0.22, NewValue: 0.20,
			}},
		}},
	}
	prompt := BuildPrompt(d)
	if !strings.Contains(prompt, "22.0% → 20.0%") {
		t.Errorf("P&L mover not formatted as a percentage; prompt:\n%s", prompt)
	}
}
