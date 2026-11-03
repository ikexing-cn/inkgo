package kit

import (
	"io/fs"
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
	light, dark := colorTokens(tokensCSS)
	need := []string{"bg", "raised", "side", "hover", "active", "line", "border", "ink", "ink-2", "muted", "faint", "dim", "on-ink", "amber", "warn", "success", "danger", "phone-eye"}
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
	if p := DarkPalette(); p.Side != ui.Hex("#1f1f1e") || p.Theme.TextMuted != ui.Hex("#a0a09a") {
		t.Fatalf("dark side = %v, muted = %v", p.Side, p.Theme.TextMuted)
	}
	if LightPalette().PhoneEye != light["phone-eye"] || DarkPalette().PhoneEye != dark["phone-eye"] {
		t.Fatal("phone-eye palette drift")
	}
}

// The sizes, corners, line heights, icons, opacity and shadows the desk uses
// are those of tokens.css, under the same names.
func TestSizeTokens(t *testing.T) {
	code := map[string]float64{
		"fs-display": FsDisplay, "fs-title": FsTitle, "fs-dialog": FsDialog, "fs-group": FsGroup,
		"fs-body": FsBody, "fs-control": FsControl, "fs-note": FsNote, "fs-small": FsSmall,
		"fs-read": FsRead, "fs-paper": TextSizeDefault,
		"r-xs": RXs, "r-control": RControl, "r-card": RCard, "r-dialog": RDialog, "r-round": RRound,
		"lh-tight": LhTight, "lh-body": LhBody, "lh-loose": LhLoose, "lh-read": LhRead, "lh-paper": LhPaper,
		"icon-xs": IconXs, "icon-sm": IconSm, "icon-md": IconMd, "icon-lg": IconLg,
		"opacity-disabled": OpacityDisabled,
		"h-control":        HControl, "h-control-sm": HControlSm, "h-row": HRow, "h-seg": HSeg, "h-tag": HTag, "h-bar": HBar,
	}
	// The prototype's window frame is not the software's.
	board := map[string]bool{"r-window-win": true}
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
	if l, d := LightPalette().Scrim, DarkPalette().Scrim; l != ui.RGBA(25, 25, 25, 0.32) || d != ui.RGBA(0, 0, 0, 0.55) {
		t.Fatalf("scrim = %v / %v", l, d)
	}
}

// The desk writes the names of the design rules, not numbers: a text size,
// a corner, a line height, an icon's size, a button's height, a color or a shadow written
// out in a view drifts from the design. Its alphas keep to the alpha scale, its
// padding, gaps and margins keep to the spacing scale of tokens.css; what
// lines up with something else is written as the sum it is.
func TestNoLiteralSizes(t *testing.T) {
	files := deskSources(t)
	call := regexp.MustCompile(`\.(FontSize|Radius|LineHeight)\(([^()]*)\)|\b(?:kit\.)?Glyph\(c, [^,]+, ([^,]+),`)
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
	files := deskSources(t)
	// The palette is a.pal in desk, pal where a page took it from its host
	// once, or the host's Colors() itself.
	button := regexp.MustCompile(`(?:\bpal|Colors\(\))\.(Primary|Secondary|Danger)\(|(?:\bpal|Colors\(\))\.Ghost\(`)
	answer := regexp.MustCompile(`(?:\bpal|Colors\(\))\.\w+\(c, "([^"]+)"\)`)
	asked := 0 // the rows in the system's order: the one question about saving
	rows := 0  // the rows with a button in them
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
			main, counted := false, false
			for k, in := range own {
				m := button.FindStringSubmatch(in)
				if m == nil || !strings.HasPrefix(in, "if ") {
					continue
				}
				if !counted {
					rows, counted = rows+1, true
				}
				switch {
				case k < spacer && (k > 0 || spacer > 1 || !strings.Contains(m[0], "Ghost")):
					t.Errorf("%s:%d: a button left of the row's spacer: %s", f, i+1, in)
				case k == 0 && spacer < 0 && !strings.Contains(line, "Justify(ui.End)"):
					t.Errorf("%s:%d: a row of buttons that is not at the right: say Justify(ui.End)", f, i+1)
				}
				if main && m[1] != "Primary" && !system {
					t.Errorf("%s:%d: a button after the main one of its row: %s", f, i+1, in)
				}
				main = main || m[1] == "Primary"
			}
			if system {
				asked++
				var said []string
				for _, in := range own {
					// The shell's saving question now uses generated labels.
					for key, text := range map[string]string{"CommonSave": "保存", "DialogDontSave": "不保存", "CommonCancel": "取消"} {
						in = strings.ReplaceAll(in, "locales."+key+"()", strconv.Quote(text))
					}
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
	if rows < 40 {
		t.Errorf("%d rows of buttons read, of the forty and more there are: the check no longer finds them", rows)
	}
}

// What can be clicked answers the pointer: DESIGN.md has it as a rule. Every
// button made here by hand says what it does when hovered, within the lines
// that draw it; the ones of theme.go (primary, secondary, ghost, ...) are
// made that way once.
func TestButtonsAnswerThePointer(t *testing.T) {
	files := deskSources(t)
	// A button is made with the context, or with the context given a key.
	button := regexp.MustCompile(`ui\.ButtonBase\(c[).]`)
	made := regexp.MustCompile(`^\s*(\w+) :?= ui\.ButtonBase\(c[).]`)
	seen := 0
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
			if !button.MatchString(line) {
				continue
			}
			seen++
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
	// The way buttons are written changed once without this check seeing it.
	if seen < 40 {
		t.Errorf("%d buttons read, of the fifty and more there are: the check no longer finds them", seen)
	}
}

func deskSources(t *testing.T) []string {
	t.Helper()
	here, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".go") {
			relative, err := filepath.Rel(here, path)
			if err != nil {
				return err
			}
			files = append(files, relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
