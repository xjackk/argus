// Package narrator turns a deterministic engine.DiffResult into a short,
// plain-English narrative. It is a POST-PROCESSING step, fully separate from
// the engine: the engine emits narrative=null and never imports this package.
// The AI only ever narrates facts we already computed — it never judges whether
// a value is correct (DATA-CONTRACT / PROJECT.md §11).
package narrator

import (
	"context"
	"fmt"
	"strings"

	"argus/engine"
)

// Narrator produces the summary.narrative string for a diff. Implementations
// must be null-safe by contract: on any failure they return ("", err) and the
// caller leaves narrative=null, which the UI already tolerates.
type Narrator interface {
	Narrate(ctx context.Context, d engine.DiffResult) (string, error)
}

// SystemPrompt is the grounding rule shared by every LLM-backed adapter. It is
// what keeps the model narrating rather than judging, and what stops it from
// inventing figures — it is handed the exact numbers in the user prompt.
const SystemPrompt = `You narrate a spreadsheet diff for a finance reviewer.
Rules:
- Describe ONLY the changes provided below, using the exact figures given.
- Do NOT invent numbers, and do NOT judge whether any value is correct, wise, or reasonable.
- 2-3 sentences, plain English, no bullet points, no preamble.
Lead with the authored input that changed, then the largest downstream effects.`

// BuildPrompt distills a DiffResult into the minimal grounded fact block the
// model narrates. This is the load-bearing part: every number is PRE-RENDERED
// via its displayFormat (so the model reads "27.5% → 24.5%", never a raw float
// or a format code), which removes both arithmetic and formatting from the
// model's job and leaves it only prose.
func BuildPrompt(d engine.DiffResult) string {
	var b strings.Builder

	// Mover carries no displayFormat (see DATA-CONTRACT), so recover it by
	// keying every changed cell by its fully-qualified ref "Sheet!Coord".
	fmtByRef := map[string]string{}
	for _, s := range d.Sheets {
		for _, c := range s.Changes {
			fmtByRef[s.Name+"!"+c.Coord] = c.DisplayFormat
		}
	}

	b.WriteString("AUTHORED INPUT CHANGES (a human typed these):\n")
	authored := 0
	for _, s := range d.Sheets {
		for _, c := range s.Changes {
			if c.Classification != "authored" {
				continue
			}
			authored++
			fmt.Fprintf(&b, "- %s: %s → %s\n",
				labelOr(c.Label, s.Name+"!"+c.Coord),
				renderValue(c.OldValue, c.DisplayFormat),
				renderValue(c.NewValue, c.DisplayFormat))
		}
	}
	if authored == 0 {
		b.WriteString("- (none)\n")
	}

	b.WriteString("\nLARGEST DOWNSTREAM EFFECTS (recalculated by formulas):\n")
	movers := 0
	for _, cas := range d.Cascades {
		for _, m := range cas.TopMovers {
			movers++
			mf := fmtByRef[m.Ref]
			fmt.Fprintf(&b, "- %s: %s → %s\n",
				labelOr(m.Label, m.Ref),
				renderValue(m.OldValue, mf),
				renderValue(m.NewValue, mf))
		}
	}
	if movers == 0 {
		b.WriteString("- (none)\n")
	}

	return b.String()
}

// renderValue formats a raw cell value for the prompt using the cell's Excel
// number-format code. It handles the format families these models use
// (percent, currency, the "x" multiple suffix); anything else falls back to a
// compact number. Exact UI rendering is the frontend's job — this only needs to
// read cleanly to the model.
func renderValue(v any, format string) string {
	f, ok := toFloat(v)
	if !ok {
		if v == nil {
			return "(empty)"
		}
		return fmt.Sprintf("%v", v)
	}
	switch {
	case strings.Contains(format, "%"):
		return trimNum(f*100, decimals(format)) + "%"
	case strings.Contains(format, "$"):
		return "$" + withThousands(trimNum(f, decimals(format)))
	case strings.Contains(strings.ToLower(format), "x"):
		return trimNum(f, orDefault(decimals(format), 2)) + "x"
	default:
		return trimNum(f, 2)
	}
}

func labelOr(label *string, fallback string) string {
	if label != nil && *label != "" {
		return *label
	}
	return fallback
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// decimals counts the digits after the decimal point in a format code
// (e.g. "0.0%" -> 1, "0.00\\x" -> 2). Returns 0 if there is no fractional part.
func decimals(format string) int {
	dot := strings.Index(format, ".")
	if dot < 0 {
		return 0
	}
	n := 0
	for i := dot + 1; i < len(format) && format[i] == '0'; i++ {
		n++
	}
	return n
}

func orDefault(n, def int) int {
	if n == 0 {
		return def
	}
	return n
}

func trimNum(f float64, dec int) string {
	return fmt.Sprintf("%.*f", dec, f)
}

// withThousands inserts comma group separators into the integer part of a
// numeric string, preserving any fractional part and leading minus sign.
func withThousands(s string) string {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	intPart, frac := s, ""
	if dot := strings.Index(s, "."); dot >= 0 {
		intPart, frac = s[:dot], s[dot:]
	}
	var out []byte
	for i, c := range []byte(intPart) {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	res := string(out) + frac
	if neg {
		res = "-" + res
	}
	return res
}
