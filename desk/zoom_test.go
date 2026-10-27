package desk

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/shelf"
)

// Zoom keys work off the writing page and keep the scale for the next start.
func TestZoomKeysKeepTheScaleOnThisMachine(t *testing.T) {
	a, tt, _ := fixture(t)
	var changed []float32
	a.ZoomChanged = func(z float32) { changed = append(changed, z) }
	tt.Key(ui.Cmd, ui.KeyEqual)
	tt.Key(ui.Cmd, ui.KeyEqual)
	tt.Frame()
	s, _, err := shelf.Load(a.books.file)
	if err != nil || a.Zoom() != 1.25 || s.Zoom != 1.25 || !slices.Equal(changed, []float32{1.1, 1.25}) {
		t.Fatalf("zoom %v, saved %v, changes %v, error %v", a.Zoom(), s.Zoom, changed, err)
	}
	b := newApp(a.ai.ConfigFile())
	if b.Zoom() != 1.25 {
		t.Fatalf("next start: %v", b.Zoom())
	}
	// The shelf, without an open work, keeps the same global keys.
	a.closeWork()
	tt.Frame()
	tt.Key(ui.Cmd, ui.Key0)
	tt.Frame()
	s, _, err = shelf.Load(a.books.file)
	if err != nil || a.Zoom() != 1 || s.Zoom != 0 || cmdZoomReset.available(a) {
		t.Fatalf("actual size: %v, saved %v, error %v", a.Zoom(), s.Zoom, err)
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyEqual)
	if a.Zoom() != 1.1 {
		t.Fatalf("Ctrl+plus: %v", a.Zoom())
	}
	tt.Key(ui.Cmd, ui.KeyMinus)
	if a.Zoom() != 1 {
		t.Fatalf("smaller: %v", a.Zoom())
	}
}

// The view menu keeps the prototype's groups, shortcuts and current percentage.
func TestTheViewMenuSizesTheWholeInterface(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨从傍晚开始下。")
	tt.SetSize(1500, 1100)
	choose := func(row string) {
		t.Helper()
		if err := clickMenu(tt, "视图", "菜单-"+row); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.menu.Owner != "" {
			t.Fatalf("%s left the menu open", row)
		}
	}
	choose("放大")
	choose("放大")
	if a.Zoom() != 1.25 {
		t.Fatalf("menu zoom: %v", a.Zoom())
	}
	if got := openMenu(t, a, tt, "视图"); !slices.Equal(got, []string{"收起侧栏", "收起右栏", "专注写作", "手机预览", "放大", "缩小", "实际大小"}) {
		t.Fatal(got)
	}
	texts := tt.Texts()
	at := -1
	for _, word := range []string{"手机预览", "缩放 125%", "放大", modKey + "=", "缩小", modKey + "-", "实际大小", modKey + "0", "外观"} {
		at = indexFrom(texts, at+1, word)
		if at < 0 {
			t.Fatalf("menu does not list %q in order: %q", word, texts)
		}
	}
	m := &kit.Menu{}
	a.viewMenu(m, func(f func()) func() { return f })
	if len(m.Rows) != 11 || m.Rows[4].Kind != m.Rows[9].Kind || m.Rows[5].Label != "缩放 125%" || m.Rows[4].Label != "" {
		t.Fatalf("menu groups: %+v", m.Rows)
	}
	tt.Key(0, ui.KeyEscape)
	choose("缩小")
	if a.Zoom() != 1.1 {
		t.Fatalf("menu smaller: %v", a.Zoom())
	}
	choose("实际大小")
	if a.Zoom() != 1 || cmdZoomReset.available(a) {
		t.Fatalf("menu actual size: %v", a.Zoom())
	}
	if err := clickMenu(tt, "视图", "菜单-实际大小"); err != nil {
		t.Fatal(err)
	}
	if a.menu.Owner != barKey+"视图" {
		t.Fatal("actual size at 100% was not disabled")
	}
}

// A step at the end or outside the display cannot be picked or keyed.
func TestZoomStopsAtTheScreenAndTheLastStep(t *testing.T) {
	for _, tc := range []struct {
		name string
		zoom float32
		fits func(float32) bool
		key  ui.Key
		row  string
	}{
		{"largest", 2, nil, ui.KeyEqual, "放大"},
		{"smallest", 0.5, nil, ui.KeyMinus, "缩小"},
		{"screen", 1.25, func(z float32) bool { return z <= 1.25 }, ui.KeyEqual, "放大"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, tt, _ := fixture(t)
			tt.SetSize(2100, 1450)
			a.shelf.Zoom, a.ZoomFits = tc.zoom, tc.fits
			tt.Frame()
			if err := clickMenu(tt, "视图", "菜单-"+tc.row); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if a.menu.Owner != barKey+"视图" || a.Zoom() != tc.zoom {
				t.Fatalf("%s was enabled at %v", tc.row, tc.zoom)
			}
			tt.Key(0, ui.KeyEscape)
			tt.Key(ui.Cmd, tc.key)
			if a.Zoom() != tc.zoom {
				t.Fatalf("key passed the limit: %v", a.Zoom())
			}
			if tc.fits != nil {
				a.ZoomFits = func(z float32) bool { return z <= 1.5 }
				tt.Key(ui.Cmd, ui.KeyEqual)
				if a.Zoom() != 1.5 {
					t.Fatal("moving to a larger screen did not allow the step")
				}
			}
		})
	}
}

// The layout uses zoomed DIPs; keyboard zoom leaves the prose size alone.
func TestZoomUsesTheNarrowLayoutWithoutSizingTheProse(t *testing.T) {
	a, _, _ := writingFixture(t, "雨从傍晚开始下。")
	var width float32
	tt := stillTester(func(c *ui.Context) { a.View(c); width, _ = c.Size() }, 1400, 1000)
	tt.Frame()
	side, ok := tt.Find("侧栏标题行")
	if width != 1400 || !ok || side.W != 232 {
		t.Fatalf("wide layout: %v", width)
	}
	shownEditor(a).Focus()
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyEqual)
	tt.Key(ui.Cmd, ui.KeyEqual)
	tt.Frame()
	side, ok = tt.Find("侧栏标题行")
	if width != 1120 || !ok || side.W != 200*a.Zoom() || shownEditor(a).FontSize() != kit.TextSizeDefault || savedPrefs(t, a).TextSize != 0 {
		t.Fatalf("zoomed layout %v, font %v", width, shownEditor(a).FontSize())
	}
	// The prose wheel's callback keeps its own scale and preference.
	// The editor tests dispatch Ctrl+wheel; Tester.Wheel drops modifiers.
	a.sizeText(1)
	if shownEditor(a).FontSize() != kit.TextSizeDefault+1 || savedPrefs(t, a).TextSize != kit.TextSizeDefault+1 || a.Zoom() != 1.25 {
		t.Fatalf("wheel font %v, zoom %v", shownEditor(a).FontSize(), a.Zoom())
	}
	tt.Key(ui.Cmd, ui.Key0)
	tt.Frame()
	if width != 1400 || shownEditor(a).FontSize() != kit.TextSizeDefault+1 {
		t.Fatalf("reset changed the prose size: width %v, font %v", width, shownEditor(a).FontSize())
	}
}
