package desk

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"writer/internal/testshot"
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

// A menu's rows are listed with nothing drawn and nothing done: what a row
// does is kept with it, for the one picked.
func TestMenuListsItsRows(t *testing.T) {
	var done []string
	drawn := 0
	m := &menu{}
	m.Head("章节")
	for _, label := range []string{"重命名", "上移", "下移"} {
		m.Item(label).Disabled(label == "上移").Do(func() { done = append(done, label) })
	}
	m.Separator()
	m.Draw(func(*ui.Context) { drawn++ })
	m.Submenu("移到", func(m *menu) {})
	m.Item("重命名").Hint("F2")
	if len(m.rows) != 8 || len(done) != 0 || drawn != 0 {
		t.Fatalf("%d rows, done %v, drawn %d", len(m.rows), done, drawn)
	}
	if it := m.rows[2]; it.kind != rowItem || it.label != "上移" || !it.off {
		t.Fatalf("the third row is %+v", it)
	}
	m.rows[3].do()
	if !slices.Equal(done, []string{"下移"}) {
		t.Fatalf("done %v", done)
	}
	// A row may have nothing to do: picked, it only closes the menu.
	if it := m.rows[7]; it.kind != rowItem || it.hint != "F2" || it.do != nil {
		t.Fatalf("the last row is %+v", it)
	}
}

// A row picked does its part once, the menu closed by then; one with
// nothing to do, or that cannot be picked, closes it or leaves it.
func TestMenuRowDoesItsPart(t *testing.T) {
	a, tt, _ := writingFixture(t, "沈砚推开门。")
	done, wasOpen := 0, false
	lists, most := 0, 0 // how often the rows were listed, and the most in a frame
	open := func() {
		// As one drawn in the frame before: else it is taken for a menu of
		// what is no longer shown, and put away.
		a.menu = menuState{owner: "试", x: 300, y: 300, seen: true}
		tt.Frame()
	}
	tt = ui.NewTester(func(c *ui.Context) {
		before := lists
		a.View(c)
		a.menuAt(c, "试", func(m *menu) {
			lists++
			m.Item("做一件事").Do(func() { done, wasOpen = done+1, a.menu.owner != "" })
			m.Item("不能点").Disabled(true).Do(func() { done += 10 })
			m.Item("什么也不做")
		})
		most = max(most, lists-before)
	}, 1266, 780)
	tt.Frame()
	open()
	if err := pick(tt, "不能点"); err != nil {
		t.Fatal(err)
	}
	if done != 0 || a.menu.owner == "" {
		t.Fatal("a row that cannot be picked was", done, a.menu.owner)
	}
	if err := tt.Click("菜单-做一件事"); err != nil {
		t.Fatal(err)
	}
	if done != 1 || wasOpen || a.menu.owner != "" {
		t.Fatal("picked:", done, wasOpen, a.menu.owner)
	}
	// The rows are listed once a frame, that of the pick too, and not at
	// all once the menu is closed.
	listed := lists
	tt.Frame()
	if done != 1 || len(rows(tt)) != 0 || most != 1 || lists != listed {
		t.Fatal("after the pick:", done, rows(tt), most, lists-listed)
	}
	open()
	if err := pick(tt, "什么也不做"); err != nil {
		t.Fatal(err)
	}
	if done != 1 || a.menu.owner != "" {
		t.Fatal("a row with nothing to do:", done, a.menu.owner)
	}
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
	testshot.Dir(t)
	a, tt, _ := writingFixture(t, "雨从傍晚开始下，一直没有停。\n林晚背着工具箱，沿着亮着的灯往旧城走。")
	tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138})

	shut := func() {
		a.menu = menuState{}
		tt.Frame()
	}
	a.write.ed.Select(0, 1)
	tt.Frame()
	tt.RightClickAt(520, 240)
	shot(t, tt, "menu-text.png")
	if err := tt.Click("菜单-加入名字表"); err != nil {
		t.Fatal(err)
	}
	shot(t, tt, "menu-names.png")
	shut()
	tt.RightClickAt(520, 735) // at the window's foot
	tt.Frame()
	if err := tt.Click("菜单-加入名字表"); err != nil {
		t.Fatal(err)
	}
	shot(t, tt, "menu-names-low.png")
	shut()

	if err := tt.RightClick("章节-第一章"); err != nil {
		t.Fatal(err)
	}
	shot(t, tt, "menu-tree.png")
	shut()

	if err := tt.Click("切换作品"); err != nil {
		t.Fatal(err)
	}
	shot(t, tt, "menu-work.png")
	shut()

	if err := tt.Click("更多"); err != nil { // the dots over the setting
		t.Fatal(err)
	}
	shot(t, tt, "menu-pane.png")
	shut()

	if err := tt.Click("讨论"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("当前模型"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	shot(t, tt, "menu-model.png")
	shut()
	tt.SetDark(true)
	if err := tt.RightClick("对话-" + a.session); err != nil {
		t.Fatal(err)
	}
	shot(t, tt, "menu-talk-dark.png")
	shut()
}

// The list beside the menu opens as the pointer comes over its row and goes
// as it moves on to another; and in no frame is the menu or the list
// anywhere but where it was put, whatever else comes over the window.
func TestMenuStaysPut(t *testing.T) {
	a, tt, _ := writingFixture(t, "沈砚推开门。")
	// Where the frame before each one had the menu's first row, and the
	// list's.
	var menuX, listY []float32
	tt = ui.NewTester(func(c *ui.Context) {
		if r, ok := tt.Find("菜单-剪切"); ok {
			menuX = append(menuX, r.X)
		}
		if r, ok := tt.Find("菜单-人物"); ok {
			listY = append(listY, r.Y)
		}
		a.View(c)
	}, 1266, 780)
	a.write.ed.Select(0, 2)
	tt.Frame()
	tt.RightClickAt(520, 600)
	tt.Frame()
	if slices.Contains(rows(tt), "人物") {
		t.Fatal("the list is open before the pointer came", rows(tt))
	}
	top, _ := tt.Find("菜单-剪切")
	row, _ := tt.Find("菜单-加入名字表")
	tt.Move(row.X+20, row.Y+10)
	tt.Frame()
	if !slices.Contains(rows(tt), "人物") {
		t.Fatal("the pointer over the row opened nothing", rows(tt))
	}
	// A click on the row leaves the list open, and Tab leaves the menu be.
	tt.ClickAt(row.X+20, row.Y+10)
	tt.Key(0, ui.KeyTab)
	tt.Key(ui.Shift, ui.KeyTab)
	tt.Frame()
	if !slices.Contains(rows(tt), "人物") {
		t.Fatal("the list went", rows(tt))
	}
	// Onto the list, it stays; on to another row of the menu, it goes.
	in, _ := tt.Find("菜单-地点")
	tt.Move(in.X+20, in.Y+10)
	tt.Frame()
	if !slices.Contains(rows(tt), "人物") {
		t.Fatal("the list went from under the pointer", rows(tt))
	}
	other, _ := tt.Find("菜单-全选")
	tt.Move(other.X+20, other.Y+10)
	tt.Frame()
	if slices.Contains(rows(tt), "人物") || len(rows(tt)) == 0 {
		t.Fatal("the list stayed, or the menu went", rows(tt))
	}
	for _, x := range menuX {
		if x != top.X {
			t.Fatalf("the menu moved: %v", menuX)
		}
	}
	if len(listY) == 0 || slices.Min(listY) < top.Y {
		t.Fatalf("the list was drawn away from its row: %v, the menu starts at %v", listY, top.Y)
	}
}
