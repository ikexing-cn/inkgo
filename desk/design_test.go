package desk

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"go.yaml.in/yaml/v3"
)

// DESIGN.md is written in the DESIGN.md format: its front matter gives the
// values of tokens.css under the format's names, and its sections are the
// format's, in the format's order.
func TestDesignMD(t *testing.T) {
	raw, err := os.ReadFile("../DESIGN.md")
	if err != nil {
		t.Fatal(err)
	}
	front, body, ok := strings.Cut(strings.TrimPrefix(string(raw), "---\n"), "\n---\n")
	if !ok || !strings.HasPrefix(string(raw), "---\n") {
		t.Fatal("DESIGN.md does not start with front matter between two --- lines")
	}
	var d struct {
		Colors     map[string]string
		Typography map[string]struct {
			FontSize   string  `yaml:"fontSize"`
			LineHeight float64 `yaml:"lineHeight"`
		}
		Rounded    map[string]string
		Spacing    map[string]string
		Components map[string]map[string]string
	}
	if err := yaml.Unmarshal([]byte(front), &d); err != nil {
		t.Fatal(err)
	}
	px := func(s string) float64 {
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "px"), 64)
		if err != nil || !strings.HasSuffix(s, "px") {
			return -1
		}
		return v
	}

	// Colors: both themes, the dark one under -dark. The page behind the
	// prototype's window is not the software's.
	light, dark := colorTokens(tokensCSS)
	delete(light, "page")
	for name, want := range light {
		for suffix, want := range map[string]ui.Color{"": want, "-dark": dark[name]} {
			if got := d.Colors[name+suffix]; got == "" || ui.Hex(got) != want {
				t.Errorf("colors.%s is %q, tokens.css has %v", name+suffix, got, want)
			}
		}
	}
	if d.Colors["primary"] != "{colors.ink}" || len(d.Colors) != 2*len(light)+1 {
		t.Errorf("colors has %d names, primary = %q; tokens.css has %d colors in each theme", len(d.Colors), d.Colors["primary"], len(light))
	}

	// Sizes, corners, line heights, icons and heights, by their prefix.
	sizes := map[string]map[string]float64{}
	board := map[string]bool{"fs-board-title": true, "fs-board-h2": true, "r-window": true, "r-window-win": true}
	blocks(tokensCSS, func(_ bool, body string) {
		for _, m := range regexp.MustCompile(`--(fs|r|lh|icon|h)-([a-z0-9-]+)\s*:\s*([0-9.]+)(?:px)?;`).FindAllStringSubmatch(body, -1) {
			if board[m[1]+"-"+m[2]] {
				continue
			}
			if sizes[m[1]] == nil {
				sizes[m[1]] = map[string]float64{}
			}
			sizes[m[1]][m[2]], _ = strconv.ParseFloat(m[3], 64)
		}
	})
	lineHeights := []float64{0}
	for _, v := range sizes["lh"] {
		lineHeights = append(lineHeights, v)
	}
	for name, want := range sizes["fs"] {
		got := d.Typography[name]
		if px(got.FontSize) != want {
			t.Errorf("typography.%s.fontSize is %q, --fs-%s is %v", name, got.FontSize, name, want)
		}
		if !slices.Contains(lineHeights, got.LineHeight) {
			t.Errorf("typography.%s.lineHeight is %v, not one of --lh-…", name, got.LineHeight)
		}
	}
	for name, want := range sizes["r"] {
		if px(d.Rounded[name]) != want {
			t.Errorf("rounded.%s is %q, --r-%s is %v", name, d.Rounded[name], name, want)
		}
	}
	if len(d.Typography) != len(sizes["fs"]) || len(d.Rounded) != len(sizes["r"]) {
		t.Errorf("%d typography levels and %d corners; tokens.css has %d and %d", len(d.Typography), len(d.Rounded), len(sizes["fs"]), len(sizes["r"]))
	}

	space := regexp.MustCompile(`--space\s*:\s*([0-9 ]+);`).FindStringSubmatch(tokensCSS)
	if space == nil {
		t.Fatal("tokens.css lacks --space")
	}
	steps := strings.Fields(space[1])
	for _, n := range steps {
		if d.Spacing[n] != n+"px" {
			t.Errorf("spacing.%s is %q", n, d.Spacing[n])
		}
	}
	if len(d.Spacing) != len(steps) {
		t.Errorf("spacing has %d steps, --space has %d", len(d.Spacing), len(steps))
	}

	// Components: a height or a size is one of --h-… and --icon-…, and each of
	// those is some component's; padding is a step of the spacing; whatever
	// has colors has them in the dark theme too, under -dark.
	unused := map[float64]string{}
	for _, group := range []string{"h", "icon"} {
		for name, v := range sizes[group] {
			unused[v] = "--" + group + "-" + name
		}
	}
	known := slices.Collect(func(yield func(float64) bool) {
		for v := range unused {
			if !yield(v) {
				return
			}
		}
	})
	for name, c := range d.Components {
		for _, k := range []string{"height", "size"} {
			if v, ok := c[k]; ok {
				if !slices.Contains(known, px(v)) {
					t.Errorf("components.%s.%s is %q, not one of --h-… or --icon-…", name, k, v)
				}
				delete(unused, px(v))
			}
		}
		if v, ok := c["padding"]; ok && !slices.Contains(steps, strings.TrimSuffix(v, "px")) {
			t.Errorf("components.%s.padding is %q, not a step of the spacing", name, v)
		}
		if base, isDark := strings.CutSuffix(name, "-dark"); isDark {
			if _, ok := d.Components[base]; !ok {
				t.Errorf("components.%s has no components.%s", name, base)
			}
			continue
		}
		for _, k := range []string{"backgroundColor", "textColor"} {
			if v, ok := c[k]; ok {
				if want := strings.TrimSuffix(v, "}") + "-dark}"; d.Components[name+"-dark"][k] != want {
					t.Errorf("components.%s-dark.%s is %q, want %q", name, k, d.Components[name+"-dark"][k], want)
				}
			}
		}
	}
	for _, name := range unused {
		t.Errorf("no component has the size of %s", name)
	}

	var sections []string
	for _, m := range regexp.MustCompile(`(?m)^## (.+)$`).FindAllStringSubmatch(body, -1) {
		sections = append(sections, m[1])
	}
	want := []string{"Overview", "Colors", "Typography", "Layout", "Elevation & Depth", "Shapes", "Components", "Do's and Don'ts"}
	if !slices.Equal(sections, want) {
		t.Errorf("sections are %q, the format has %q", sections, want)
	}
}
