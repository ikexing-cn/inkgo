package desk

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The palette takes every color it needs from tokens.css, in both themes.
func TestTokens(t *testing.T) {
	light, dark := tokens(tokensCSS)
	need := []string{"bg", "raised", "side", "hover", "active", "line", "border", "ink", "ink-2", "muted", "faint", "dim", "on-ink", "amber", "warn", "success", "danger"}
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

// The sizes, corners, line heights, icons, opacity and shadows the desk uses
// are those of tokens.css, under the same names.
func TestSizeTokens(t *testing.T) {
	code := map[string]float64{
		"fs-display": fsDisplay, "fs-title": fsTitle, "fs-dialog": fsDialog, "fs-group": fsGroup,
		"fs-body": fsBody, "fs-control": fsControl, "fs-note": fsNote, "fs-small": fsSmall,
		"fs-read": fsRead, "fs-paper": textSizeDefault,
		"r-xs": rXs, "r-control": rControl, "r-card": rCard, "r-dialog": rDialog, "r-round": rRound,
		"lh-tight": lhTight, "lh-body": lhBody, "lh-loose": lhLoose, "lh-read": lhRead, "lh-paper": lhPaper,
		"icon-xs": iconXs, "icon-sm": iconSm, "icon-md": iconMd, "icon-lg": iconLg,
		"opacity-disabled": opacityDisabled,
		"h-control":        hControl, "h-control-sm": hControlSm, "h-row": hRow, "h-seg": hSeg, "h-tag": hTag, "h-bar": hBar,
	}
	// Only the design's boards use these: its notes and window frames.
	board := map[string]bool{"fs-board-title": true, "fs-board-h2": true, "r-window": true, "r-window-win": true}
	seen := map[string]bool{}
	blocks(tokensCSS, func(_ bool, body string) {
		for _, d := range regexp.MustCompile(`--((?:fs|r|lh|icon|opacity|h)-[a-z-]+)\s*:\s*([0-9.]+)(?:px)?;`).FindAllStringSubmatch(body, -1) {
			seen[d[1]] = true
			v, _ := strconv.ParseFloat(d[2], 64)
			if want, ok := code[d[1]]; ok && want != v {
				t.Errorf("--%s is %v in tokens.css, %v in the code", d[1], v, want)
			} else if !ok && !board[d[1]] {
				t.Errorf("--%s has no constant in the code", d[1])
			}
		}
	})
	for name := range code {
		if !seen[name] {
			t.Errorf("tokens.css lacks --%s", name)
		}
	}
	light, dark := shadows(tokensCSS)
	for _, name := range []string{"raise", "pop", "dialog"} {
		if light[name].color.A == 0 || dark[name].color.A == 0 {
			t.Errorf("--shadow-%s missing: %v / %v", name, light[name], dark[name])
		}
	}
	if p := light["pop"]; p != (shadow{0, 8, 28, 0, ui.RGBA(0, 0, 0, 0.14)}) || dark["pop"].color != ui.RGBA(0, 0, 0, 0.4) {
		t.Fatalf("pop = %v / %v", p, dark["pop"])
	}
	if s := darkPalette().scrim; s != ui.RGBA(25, 25, 25, 0.32) {
		t.Fatalf("scrim = %v", s)
	}
}

// The desk writes the names of the design rules, not numbers: a text size,
// a corner, a line height, an icon's size, a button's height, a color or a shadow written
// out in a view drifts from the design. Its alphas keep to the alpha scale, its
// padding, gaps and margins keep to the spacing scale of tokens.css; what
// lines up with something else is written as the sum it is.
func TestNoLiteralSizes(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	call := regexp.MustCompile(`\.(FontSize|Radius|LineHeight)\(([^()]*)\)|\bglyph\(c, [^,]+, ([^,]+),`)
	space := regexp.MustCompile(`\.(Padding|Gap|Margin)\(([^()]*)\)`)
	alpha := regexp.MustCompile(`\.(?:Alpha|Opacity)\(([0-9.]+)\)|ui\.RGBA\([^()]*, ([0-9.]+)\)`)
	// From 100 on it is a layout size, as a card's.
	height := regexp.MustCompile(`\.(?:Height|Size)\([0-9]{1,2}(?:\.[0-9]+)?[,)]`)
	number := regexp.MustCompile(`^-?[0-9.]+$`)
	m := regexp.MustCompile(`--alpha:\s*([0-9. ]+);`).FindStringSubmatch(tokensCSS)
	if m == nil {
		t.Fatal("tokens.css lacks --alpha")
	}
	alphas := map[float64]bool{0: true, 1: true}
	for _, v := range strings.Fields(m[1]) {
		a, _ := strconv.ParseFloat(v, 64)
		alphas[a] = true
	}
	scale := map[string]bool{}
	for _, name := range []string{"space", "space-board"} {
		m := regexp.MustCompile(`--` + name + `:\s*([0-9 ]+);`).FindStringSubmatch(tokensCSS)
		if m == nil {
			t.Fatalf("tokens.css lacks --%s", name)
		}
		for _, v := range strings.Fields(m[1]) {
			scale[v] = true
		}
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			for _, m := range space.FindAllStringSubmatch(line, -1) {
				for _, arg := range strings.Split(m[2], ",") {
					if arg = strings.TrimSpace(arg); number.MatchString(arg) && !scale[strings.TrimPrefix(arg, "-")] {
						t.Errorf("%s:%d: %s(%s); %s is off the spacing scale", f, i+1, m[1], m[2], arg)
					}
				}
			}
			for _, m := range alpha.FindAllStringSubmatch(line, -1) {
				if a, _ := strconv.ParseFloat(m[1]+m[2], 64); !alphas[a] {
					t.Errorf("%s:%d: %s is off the alpha scale", f, i+1, m[0])
				}
			}
			if strings.Contains(line, "ButtonBase(") && height.MatchString(line) {
				t.Errorf("%s:%d: a button's height written out; use h*", f, i+1)
			}
			if f == "theme.go" {
				continue
			}
			if strings.Contains(line, "ui.RGBA(") {
				t.Errorf("%s:%d: a color written out; use the palette, as a.pal.scrim", f, i+1)
			}
			if strings.Contains(line, ".Shadow(") {
				t.Errorf("%s:%d: a shadow written out; use a.pal.pop and the like", f, i+1)
			}
			for _, m := range call.FindAllStringSubmatch(line, -1) {
				name, args := m[1], m[2]
				if name == "" {
					name, args = "glyph", m[3]
				}
				for _, arg := range strings.Split(args, ",") {
					if arg = strings.TrimSpace(arg); number.MatchString(arg) && arg != "0" {
						t.Errorf("%s:%d: %s(%s); use fs*, r*, lh* or icon*", f, i+1, name, args)
					}
				}
			}
		}
	}
}

// Buttons stand at the right of their row, the one that goes ahead last:
// DESIGN.md has it as a rule. A row that begins with a button
// has nothing before it to push it there, so it must say Justify(ui.End). No
// button stands left of a spacer, but for the one that decides nothing (as
// "稍后再说"), alone there; and no lesser button comes after the main one.
func TestButtonsAtTheRight(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	button := regexp.MustCompile(`a\.pal\.(primary|secondary|danger)\(|a\.pal\.ghost\(`)
	answer := regexp.MustCompile(`a\.pal\.\w+\(c, "([^"]+)"\)`)
	asked := 0 // the rows in the system's order: the one question about saving
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(b), "\n")
		for i, line := range lines {
			// The question about words not saved stands in the system's
			// order, and its row says so: 保存 · 不保存 · 取消.
			line, system := strings.CutSuffix(line, " // system order")
			if !strings.Contains(line, "ui.Row(c)") || !strings.HasSuffix(strings.TrimSpace(line), ".Children(func() {") {
				continue
			}
			// The row's own statements are those one tab deeper than it; of
			// them, the ones that draw.
			indent := line[:len(line)-len(strings.TrimLeft(line, "\t"))]
			var own []string
			for _, in := range lines[i+1:] {
				if strings.HasPrefix(in, indent+"})") {
					break
				}
				if strings.HasPrefix(in, indent+"\t") && !strings.HasPrefix(in, indent+"\t\t") && !strings.HasPrefix(in, indent+"\t}") && !strings.HasPrefix(in, indent+"\t//") && strings.Contains(in, "(c") {
					own = append(own, strings.TrimSpace(in))
				}
			}
			spacer := slices.Index(own, "ui.Spacer(c)")
			main := false
			for k, in := range own {
				m := button.FindStringSubmatch(in)
				if m == nil || !strings.HasPrefix(in, "if ") {
					continue
				}
				switch {
				case k < spacer && (k > 0 || spacer > 1 || !strings.Contains(m[0], "ghost")):
					t.Errorf("%s:%d: a button left of the row's spacer: %s", f, i+1, in)
				case k == 0 && spacer < 0 && !strings.Contains(line, "Justify(ui.End)"):
					t.Errorf("%s:%d: a row of buttons that is not at the right: say Justify(ui.End)", f, i+1)
				}
				if main && m[1] != "primary" && !system {
					t.Errorf("%s:%d: a button after the main one of its row: %s", f, i+1, in)
				}
				main = main || m[1] == "primary"
			}
			if system {
				asked++
				var said []string
				for _, in := range own {
					if m := answer.FindStringSubmatch(in); m != nil {
						said = append(said, m[1])
					}
				}
				if got := strings.Join(said, " · "); got != "保存 · 不保存 · 取消" || !strings.Contains(line, "Justify(ui.End)") {
					t.Errorf("%s:%d: a row in the system's order reads %q", f, i+1, got)
				}
			}
		}
	}
	if asked != 1 {
		t.Errorf("%d rows stand in the system's order: it is for the one question about saving", asked)
	}
}

// What can be clicked answers the pointer: DESIGN.md has it as a rule. Every
// button made here by hand says what it does when hovered, within the lines
// that draw it; the ones of theme.go (primary, secondary, ghost, ...) are
// made that way once.
func TestButtonsAnswerThePointer(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	made := regexp.MustCompile(`^\s*(\w+) :?= ui\.ButtonBase\(c\)`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(b), "\n")
		for i, line := range lines {
			if !strings.Contains(line, "ui.ButtonBase(c)") {
				continue
			}
			m := made.FindStringSubmatch(line)
			if m == nil {
				t.Errorf("%s:%d: a button with no name cannot answer the pointer: %s", f, i+1, strings.TrimSpace(line))
				continue
			}
			// The lines that draw it: to the end of its function, sixteen at most.
			answers := false
			for _, in := range lines[i+1 : min(i+17, len(lines))] {
				if strings.HasPrefix(in, "}") {
					break
				}
				if answers = strings.Contains(in, m[1]+".Hovered()"); answers {
					break
				}
			}
			if !answers {
				t.Errorf("%s:%d: a button that does not answer the pointer (no %s.Hovered()): %s", f, i+1, m[1], strings.TrimSpace(line))
			}
		}
	}
}

func TestWords(t *testing.T) {
	for n, want := range map[int]string{0: "0 字", 54: "54 字", 2846: "2,846 字", 9999: "9,999 字", 10000: "1 万字", 312345: "31.2 万字", 1200000: "120 万字"} {
		if got := words(n); got != want {
			t.Errorf("words(%d) = %q, want %q", n, got, want)
		}
	}
}
