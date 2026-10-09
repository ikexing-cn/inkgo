package desk

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The palette takes every color it needs from tokens.css, in both themes.
func TestTokens(t *testing.T) {
	light, dark := tokens(tokensCSS)
	need := []string{"bg", "raised", "side", "hover", "active", "line", "border", "ink", "ink-2", "muted", "faint", "on-ink", "amber", "warn", "success", "danger"}
	for _, name := range need {
		if _, ok := light[name]; !ok {
			t.Errorf("light theme lacks --%s", name)
		}
		if _, ok := dark[name]; !ok {
			t.Errorf("dark theme lacks --%s", name)
		}
	}
	if light["ink"] != ui.Hex("#1d1d1b") || dark["ink"] != ui.Hex("#ededea") || dark["page"] != light["page"] {
		t.Fatalf("ink = %v / %v, page = %v / %v", light["ink"], dark["ink"], light["page"], dark["page"])
	}
	if p := darkPalette(); p.side != ui.Hex("#1f1f1e") || p.theme.TextMuted != ui.Hex("#a0a09a") {
		t.Fatalf("dark side = %v, muted = %v", p.side, p.theme.TextMuted)
	}
}

func TestWords(t *testing.T) {
	for n, want := range map[int]string{0: "0 字", 54: "54 字", 2846: "2,846 字", 9999: "9,999 字", 10000: "1 万字", 312345: "31.2 万字", 1200000: "120 万字"} {
		if got := words(n); got != want {
			t.Errorf("words(%d) = %q, want %q", n, got, want)
		}
	}
}
