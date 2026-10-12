package desk

import (
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// pick clicks the rows of the menu that is open along path: each but the
// last opens a list beside the menu, the last is the one picked.
func pick(tt *ui.Tester, path ...string) error {
	for _, label := range path {
		tt.Frame()
		if err := tt.Click("菜单-" + strings.TrimSuffix(label, "…")); err != nil {
			return err
		}
	}
	tt.Frame()
	return nil
}

// rows returns what the rows of the menu that is open say.
func rows(tt *ui.Tester) []string {
	var out []string
	for _, s := range tt.Texts() {
		if label, ok := strings.CutPrefix(s, "菜单-"); ok {
			out = append(out, label)
		}
	}
	return out
}

// The text's menu is the app's own: it opens where the pointer asked, stays
// in the window, and a press beside it puts it away and does nothing else.
func TestTextMenu(t *testing.T) {
	a, tt, _ := writingFixture(t, "雨从傍晚开始下。\n林晚背着工具箱。")
	a.write.ed.Select(0, 1)
	tt.Frame()
	if len(rows(tt)) != 0 {
		t.Fatal("a menu before one was asked for", rows(tt))
	}
	tt.RightClickAt(550, 300)
	tt.Frame()
	tt.Frame()
	if len(tt.Menu()) != 0 {
		t.Fatal("the system's menu opened", tt.Menu())
	}
	want := []string{"剪切", "复制", "粘贴", "全选", "续写一句", "AI 改写", "引用到讨论", "加入设定", "加入名字表", "记为伏笔", "记为回收"}
	if got := rows(tt); !slices.Equal(got, want) {
		t.Fatalf("rows %q", got)
	}
	// The panel's corner is where the pointer was: its rows start inside
	// its border and padding.
	if r, ok := tt.Find("菜单-剪切"); !ok || r.X != 555 || r.Y != 305 || r.W != menuWide-10 {
		t.Fatalf("the menu is not where the pointer asked: %+v", r)
	}
	if err := pick(tt, "复制"); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != "雨" || len(rows(tt)) != 0 {
		t.Fatalf("copied %q, rows %q", tt.Clipboard(), rows(tt))
	}

	// Asked for at the window's corner, it is still whole inside it.
	tt.RightClickAt(800, 735)
	tt.Frame()
	tt.Frame()
	first, _ := tt.Find("菜单-剪切")
	last, ok := tt.Find("菜单-记为回收")
	if !ok || last.Y+last.H > 780-menuEdge || first.Y < menuEdge {
		t.Fatalf("out of the window: %+v to %+v", first, last)
	}
	// A press outside closes it and goes no further: the page stays.
	page := a.page
	if err := tt.Click("讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(rows(tt)) != 0 || a.page != page {
		t.Fatalf("rows %q, page %d", rows(tt), a.page)
	}

	// Escape closes it too, and the menu key opens it under the caret.
	tt.RightClickAt(550, 300)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if len(rows(tt)) != 0 {
		t.Fatal("Escape left it open", rows(tt))
	}
	a.write.ed.Focus()
	tt.Frame()
	tt.Key(0, ui.KeyContextMenu)
	tt.Frame()
	tt.Frame()
	if len(rows(tt)) == 0 {
		t.Fatal("the menu key opened nothing")
	}
}

// What a row cannot do now does nothing, and the list beside the menu
// opens from its row, level with it.
func TestMenuRows(t *testing.T) {
	a, tt, _ := writingFixture(t, "沈砚推开门。")
	tt.RightClickAt(550, 300) // nothing is selected
	tt.Frame()
	if err := pick(tt, "加入设定"); err != nil {
		t.Fatal(err, rows(tt))
	}
	if a.write.takeOpen || len(rows(tt)) == 0 {
		t.Fatal("a pale row was picked", a.write.takeOpen, rows(tt))
	}
	// Pale as the design has it: the ink at opacityDisabled over the panel.
	off, _ := tt.Find("菜单-加入设定")
	darkest := uint8(255)
	img := tt.Image()
	scale := float32(img.Bounds().Dx()) / 1266
	for y := int(off.Y * scale); y < int((off.Y+off.H)*scale); y++ {
		for x := int(off.X * scale); x < int((off.X+off.W)*scale); x++ {
			darkest = min(darkest, img.RGBAAt(x, y).R)
		}
	}
	if darkest < 150 || darkest > 180 {
		t.Fatalf("a row that cannot be picked is drawn with %d", darkest)
	}
	tt.Key(0, ui.KeyEscape)
	a.write.ed.Select(0, 2)
	tt.Frame()
	tt.RightClickAt(550, 300)
	tt.Frame()
	if err := tt.Click("菜单-加入名字表"); err != nil {
		t.Fatal(err, rows(tt))
	}
	tt.Frame()
	tt.Frame()
	row, _ := tt.Find("菜单-加入名字表")
	sub, ok := tt.Find("菜单-人物")
	if !ok || sub.X < row.X+row.W || sub.Y-row.Y > 1 || sub.Y < row.Y {
		t.Fatalf("the list is not beside its row: %+v, %+v", row, sub)
	}

	// Near the window's foot the list moves up to stay whole inside it, as
	// the menu does.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.RightClickAt(550, 735)
	tt.Frame()
	if err := tt.Click("菜单-加入名字表"); err != nil {
		t.Fatal(err, rows(tt))
	}
	tt.Frame()
	tt.Frame()
	names := rows(tt)
	last, ok := tt.Find("菜单-" + names[len(names)-1])
	if !ok || !slices.Contains(names, "人物") || last.Y+last.H > 780-menuEdge {
		t.Fatalf("the list goes out of the window: %q ends at %+v", names, last)
	}
}

// SHOT=dir go test ./desk -run TestShotMenus draws each menu open.
func TestShotMenus(t *testing.T) {
	dir := os.Getenv("SHOT")
	if dir == "" {
		t.Skip("set SHOT=dir")
	}
	a, tt, _ := writingFixture(t, "雨从傍晚开始下，一直没有停。\n林晚背着工具箱，沿着亮着的灯往旧城走。")
	tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138})
	shot := func(name string) {
		tt.Frame()
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	shut := func() {
		a.menu = menuState{}
		tt.Frame()
	}
	a.write.ed.Select(0, 1)
	tt.Frame()
	tt.RightClickAt(520, 240)
	shot("menu-text.png")
	if err := tt.Click("菜单-加入名字表"); err != nil {
		t.Fatal(err)
	}
	shot("menu-names.png")
	shut()
	tt.RightClickAt(520, 735) // at the window's foot
	tt.Frame()
	if err := tt.Click("菜单-加入名字表"); err != nil {
		t.Fatal(err)
	}
	shot("menu-names-low.png")
	shut()

	if err := tt.RightClick("章节-第一章"); err != nil {
		t.Fatal(err)
	}
	shot("menu-tree.png")
	shut()

	if err := tt.Click("切换作品"); err != nil {
		t.Fatal(err)
	}
	shot("menu-work.png")
	shut()

	tt.ClickAt(1229, 56) // the dots over the setting, not the toolbar's
	shot("menu-pane.png")
	shut()

	if err := tt.Click("讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("当前模型"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	shot("menu-model.png")
	shut()
	tt.SetDark(true)
	if err := tt.RightClick("对话-" + a.session); err != nil {
		t.Fatal(err)
	}
	shot("menu-talk-dark.png")
	shut()
}
