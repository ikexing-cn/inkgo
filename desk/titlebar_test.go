package desk

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/export"
	"writer/workspace"
)

// openMenu opens a menu of the title bar and returns what its rows say.
func openMenu(t *testing.T, a *App, tt *ui.Tester, name string) []string {
	t.Helper()
	if err := tt.Click(barKey + name); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.menu.owner != barKey+name {
		t.Fatalf("the menu %s did not open: %q is open", name, a.menu.owner)
	}
	return rows(tt)
}

// shut puts away a dialog the test opened, and waits for it to be gone.
func shut(tt *ui.Tester, open *bool) {
	*open = false
	tt.Frame()
	tt.Frame()
}

// right is the right edge of the element showing or labeled s.
func right(t *testing.T, tt *ui.Tester, s string) float32 {
	t.Helper()
	r := find(t, tt, s)
	return r.X + r.W
}

// The title bar is a row of its own at the top of the window, and it alone
// keeps room for the window's buttons: under it the rows run to the edge.
// Focused, the bar is away and the toolbar keeps that room instead.
func TestTheTitleBarHasItsOwnRow(t *testing.T) {
	a, tt, _ := writingFixture(t, "整条街的灯是同时灭的。")
	const buttons = 138
	tt.SetTitleBar(ui.TitleBar{Height: 40, Right: buttons})
	tt.Frame()
	for _, name := range []string{"文件", "编辑", "视图", "帮助"} {
		if r, ok := tt.Find(barKey + name); !ok || r.Y+r.H > titleHeight {
			t.Fatalf("the menu %s is not in the title bar: %+v %v", name, r, ok)
		}
	}
	if r, _ := tt.Find("正文"); r.Y < titleHeight {
		t.Fatalf("the sidebar starts in the title bar: %+v", r)
	}
	if r, _ := tt.Find("上一个打开的章节"); r.Y < titleHeight || r.Y > 2*titleHeight {
		t.Fatalf("the toolbar is not the row under the title bar: %+v", r)
	}
	width := a.LastWrite.X + a.LastWrite.W + 1 + a.LastSetting.W
	if got := right(t, tt, "收起右栏"); got < width-buttons {
		t.Fatalf("the pane's row keeps room for the window's buttons: ends at %v of %v", got, width)
	}
	a.doc.hidden = true
	tt.Frame()
	if got := right(t, tt, "专注写作"); got < width-buttons {
		t.Fatalf("the toolbar keeps room for the window's buttons: ends at %v of %v", got, width)
	}
	if _, ok := tt.Find("更多"); ok {
		t.Fatal("the toolbar still has its 「更多」")
	}

	tt.Key(0, ui.KeyF11)
	tt.Frame()
	if _, ok := tt.Find(barKey + "文件"); ok || !a.write.focus {
		t.Fatal("focused, the title bar is still there")
	}
	if r, _ := tt.Find("退出专注"); r.Y > titleHeight {
		t.Fatalf("focused, the toolbar is not the first row: %+v", r)
	}
	if got := right(t, tt, "退出专注"); got > width-buttons {
		t.Fatalf("focused, the toolbar runs under the window's buttons: ends at %v of %v", got, width)
	}
	// The menus cannot be reached; the keys go on working.
	tt.Key(ui.Cmd, ui.KeySlash)
	tt.Frame()
	if !a.keysOpen {
		t.Fatal("focused, the shortcuts do not open")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if err := tt.Click("退出专注"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find(barKey + "文件"); !ok || a.write.focus {
		t.Fatal("leaving focus did not bring the title bar back")
	}
}

// The switch of the sidebar is in the title bar on every page, puts the
// sidebar away on each of them, and is kept with the work. On the shelf it
// stays, and does nothing.
func TestTheSidebarIsPutAwayOnEveryPage(t *testing.T) {
	a, tt, _ := writingFixture(t, "整条街的灯是同时灭的。")
	shown := func() bool { _, ok := tt.Find("切换作品"); return ok }
	for _, page := range []page{pageWrite, pageTalk, pageCheck, pageCategories} {
		a.page = page
		tt.Frame()
		tt.Frame()
		if !shown() {
			t.Fatalf("page %d: no sidebar to begin with", page)
		}
		if err := tt.Click("收起侧栏"); err != nil {
			t.Fatal(page, err)
		}
		tt.Frame()
		if shown() || !a.sideHidden {
			t.Fatalf("page %d: the sidebar is still there", page)
		}
		a.sideHidden = false
		a.loadPrefs()
		if !a.sideHidden {
			t.Fatalf("page %d: putting the sidebar away was not kept", page)
		}
		a.page = page // loadPrefs goes to the page the work was left on
		tt.Frame()
		if err := tt.Click("展开侧栏"); err != nil {
			t.Fatal(page, err)
		}
		tt.Frame()
		if !shown() || a.sideHidden {
			t.Fatalf("page %d: the sidebar did not come back", page)
		}
	}

	// Put away, it comes back for a name to be typed in the tree.
	a.page = pageWrite
	tt.Frame()
	if err := tt.Click("收起侧栏"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := clickMenu(tt, "编辑", "菜单-重命名"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.sideHidden || a.write.renaming == "" || !shown() {
		t.Fatalf("renaming with the sidebar away: hidden %v, renaming %q", a.sideHidden, a.write.renaming)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	a.closeWork()
	tt.Frame()
	tt.Frame()
	if a.Store != nil {
		t.Fatal("the work did not close")
	}
	if _, ok := tt.Find("收起侧栏"); !ok {
		t.Fatal("on the shelf the switch of the sidebar is gone: the menus moved")
	}
	if err := tt.Click("收起侧栏"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.sideHidden {
		t.Fatal("on the shelf the switch of the sidebar did something")
	}
}

// Each row of the menus does what it says.
func TestTheMenusDoWhatTheySay(t *testing.T) {
	a, tt, id := writingFixture(t, "整条街的灯是同时灭的。")
	w := &a.write
	choose := func(menu, row string) {
		t.Helper()
		if err := clickMenu(tt, menu, "菜单-"+row); err != nil {
			t.Fatal(menu, row, err, rows(tt))
		}
		tt.Frame()
		if a.menu.owner != "" {
			t.Fatalf("%s › %s left the menu open", menu, row)
		}
	}
	if got := openMenu(t, a, tt, "文件"); !slices.Equal(got, []string{"新建章节", "新建文件夹", "保存", "快照", "导出", "设置"}) {
		t.Fatalf("文件: %q", got)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.menu.owner != "" {
		t.Fatal("Escape left the menu open")
	}
	if got := openMenu(t, a, tt, "编辑"); !slices.Equal(got, []string{"撤销", "重做", "查找替换", "重命名", "续写一句", "起草本章"}) {
		t.Fatalf("编辑: %q", got)
	}
	if err := tt.Click(barKey + "编辑"); err != nil { // its name again puts it away
		t.Fatal(err)
	}
	tt.Frame()
	if a.menu.owner != "" {
		t.Fatal("clicking the menu's name again left it open")
	}
	if got := openMenu(t, a, tt, "视图"); !slices.Equal(got, []string{"收起侧栏", "收起右栏", "专注写作", "手机预览"}) || !tt.HasText("跟随系统") {
		t.Fatalf("视图: %q", got)
	}
	tt.ClickAt(a.LastWrite.X+a.LastWrite.W/2, a.LastWrite.Y+a.LastWrite.H-80) // a press elsewhere puts it away
	tt.Frame()
	if a.menu.owner != "" {
		t.Fatal("a press beside the menu left it open")
	}
	if got := openMenu(t, a, tt, "帮助"); !slices.Equal(got, []string{"快捷键"}) {
		t.Fatalf("帮助: %q", got)
	}
	// With one open, the pointer over the name of another opens that one.
	over, _ := tt.Find(barKey + "文件")
	tt.Move(over.X+over.W/2, over.Y+over.H/2)
	tt.Frame()
	tt.Frame()
	if a.menu.owner != barKey+"文件" || !slices.Contains(rows(tt), "保存") || slices.Contains(rows(tt), "快捷键") {
		t.Fatalf("the pointer over 文件 did not open it: %q open, rows %q", a.menu.owner, rows(tt))
	}
	a.menu = menuState{}
	tt.Frame()
	// With none open, the pointer over a name opens nothing.
	tt.Move(over.X+over.W/2, over.Y+over.H/2)
	tt.Frame()
	if a.menu.owner != "" {
		t.Fatal("the pointer over a name opened its menu")
	}

	// 编辑: undo and redo are pale until there is a step, and take it.
	w.ed.Focus()
	tt.Frame()
	choose("编辑", "查找替换")
	if !w.finding {
		t.Fatal("查找替换 did not open the finding")
	}
	shut(tt, &w.finding)
	if err := clickMenu(tt, "编辑", "菜单-撤销"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.menu.owner == "" {
		t.Fatal("撤销 was picked with nothing to take back")
	}
	a.menu = menuState{}
	w.ed.Focus()
	w.ed.MoveTo(0, false)
	tt.Frame()
	tt.Type("戊")
	tt.Frame()
	if got := w.ed.Doc.String(); !strings.HasPrefix(got, "戊整条街") {
		t.Fatalf("typed: %q", got)
	}
	choose("编辑", "撤销")
	if got := w.ed.Doc.String(); got != "整条街的灯是同时灭的。" {
		t.Fatalf("after 撤销: %q", got)
	}
	choose("编辑", "重做")
	if got := w.ed.Doc.String(); !strings.HasPrefix(got, "戊整条街") {
		t.Fatalf("after 重做: %q", got)
	}
	// The caret is back in the prose once the menu is away.
	saveAfter = time.Hour // what is typed now waits for the menu to save it
	tt.Type("己")
	tt.Frame()
	if got := w.ed.Doc.String(); !strings.Contains(got, "己") {
		t.Fatalf("after the menu the prose does not take what is typed: %q", got)
	}

	// 文件
	if w.ed.Doc.Version() == w.savedVer {
		t.Fatal("the chapter is not waiting to be saved")
	}
	choose("文件", "保存")
	if w.ed.Doc.Version() != w.savedVer || !strings.Contains(disk(t, a, "第一章.txt"), "己") {
		t.Fatalf("保存 did not save: %q", disk(t, a, "第一章.txt"))
	}
	saveAfter = 0
	choose("文件", "快照")
	if !w.snapsOpen {
		t.Fatal("快照 did not open the snapshots")
	}
	shut(tt, &w.snapsOpen)
	choose("文件", "导出")
	if !w.exporting.open || !tt.HasText("本章") || w.exporting.selection.Scope != export.ScopeWork {
		t.Fatal("导出 did not ask what to export, the whole work first", w.exporting.selection.Scope, tt.Texts())
	}
	shut(tt, &w.exporting.open)
	choose("文件", "设置")
	if !a.bk.prefs {
		t.Fatal("设置 did not open the settings")
	}
	shut(tt, &a.bk.prefs)
	nodes := func() (n int) { w.tree.Walk(func(*workspace.Node, int) { n++ }); return n }
	before := nodes()
	choose("文件", "新建章节")
	if nodes() != before+1 || w.renaming == "" {
		t.Fatalf("新建章节: %d nodes after %d, naming %q", nodes(), before, w.renaming)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	choose("文件", "新建文件夹")
	if n, ok := w.tree.Node(w.node); nodes() != before+2 || !ok || !n.Folder {
		t.Fatalf("新建文件夹: %d nodes after %d", nodes(), before)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	a.openNode(id)
	tt.Frame()

	// 编辑, the rest
	choose("编辑", "重命名")
	if w.renaming != id {
		t.Fatalf("重命名: naming %q", w.renaming)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if err := clickMenu(tt, "编辑", "菜单-起草本章"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if w.draftOpen || !tt.HasText("请先填写章纲") {
		t.Fatal("起草本章 without an outline", tt.Texts())
	}
	a.menu = menuState{}
	w.outline = "林晚回到旧城。"
	tt.Frame()
	choose("编辑", "起草本章")
	if !w.draftOpen {
		t.Fatal("起草本章 did not ask how to draft")
	}
	shut(tt, &w.draftOpen)
	penWith(a, &stream{pieces: []string{"，屋里", "很暗。"}})
	w.ed.Focus()
	w.ed.MoveTo(w.ed.Doc.Len(), false)
	tt.Frame()
	tt.Frame()
	choose("编辑", "续写一句")
	until(t, tt, "the sentence", func() bool { return w.ed.Ghost() != "" })
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	// 视图
	choose("视图", "收起右栏")
	if !a.doc.hidden {
		t.Fatal("收起右栏 left the pane")
	}
	choose("视图", "展开右栏")
	if a.doc.hidden {
		t.Fatal("展开右栏 did not bring the pane back")
	}
	choose("视图", "收起侧栏")
	if !a.sideHidden {
		t.Fatal("收起侧栏 left the sidebar")
	}
	choose("视图", "展开侧栏")
	if a.sideHidden {
		t.Fatal("展开侧栏 did not bring the sidebar back")
	}
	choose("视图", "专注写作")
	if !w.focus {
		t.Fatal("专注写作 did not focus")
	}
	if _, ok := tt.Find(barKey + "视图"); ok {
		t.Fatal("focused from the menu, the title bar is still there")
	}
	tt.Key(0, ui.KeyF11)
	tt.Frame()

	// 帮助
	choose("帮助", "快捷键")
	if !a.keysOpen {
		t.Fatal("快捷键 did not show the shortcuts")
	}
}

// Off the page of prose the rows that act on a chapter are pale: picking
// one does nothing and leaves the menu open. The book is still exported,
// the settings and the shortcuts still open.
func TestMenuRowsOffThePageOfProse(t *testing.T) {
	a, tt, _ := writingFixture(t, "整条街的灯是同时灭的。")
	w := &a.write
	pale := func(menu string, names ...string) {
		t.Helper()
		for _, row := range names {
			if err := clickMenu(tt, menu, "菜单-"+row); err != nil {
				t.Fatal(menu, row, err)
			}
			tt.Frame()
			if a.menu.owner != barKey+menu {
				t.Fatalf("%s › %s was picked", menu, row)
			}
			a.menu = menuState{}
			tt.Frame()
		}
	}
	for _, page := range []page{pageTalk, pageCheck, pageCategories} {
		a.page = page
		tt.Frame()
		tt.Frame()
		pale("文件", "新建章节", "新建文件夹", "保存", "快照")
		pale("编辑", "撤销", "重做", "查找替换", "重命名", "续写一句", "起草本章")
		pale("视图", "专注写作", "手机预览")
		if page != pageTalk {
			pale("视图", "收起右栏")
		}
		if a.page != page || w.focus || w.finding || w.snapsOpen || w.renaming != "" || w.draftOpen {
			t.Fatalf("page %d: a pale row did something", page)
		}
		if err := clickMenu(tt, "文件", "菜单-导出"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if !w.exporting.open || !tt.HasText("全书") || tt.HasText("本章") {
			t.Fatalf("page %d: the export does not offer the book alone: %q", page, tt.Texts())
		}
		shut(tt, &w.exporting.open)
		if err := clickMenu(tt, "文件", "菜单-设置"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if !a.bk.prefs {
			t.Fatalf("page %d: the settings did not open", page)
		}
		shut(tt, &a.bk.prefs)
		tt.Key(ui.Cmd, ui.KeySlash)
		tt.Frame()
		if !a.keysOpen {
			t.Fatalf("page %d: the shortcuts did not open", page)
		}
		shut(tt, &a.keysOpen)
	}

	// On the shelf there is no work: only what holds for this machine, the
	// settings of the AI and the appearance, and the shortcuts.
	a.closeWork()
	tt.Frame()
	tt.Frame()
	pale("文件", "新建章节", "新建文件夹", "保存", "快照", "导出")
	pale("视图", "收起侧栏", "收起右栏", "专注写作", "手机预览")
	if a.bk.prefs || w.exporting.open || a.sideHidden {
		t.Fatal("on the shelf a pale row did something")
	}
	if err := clickMenu(tt, "文件", "菜单-设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find("设置-外观"); !a.bk.prefs || !ok || tt.HasText("提醒") || tt.HasText("备份") {
		t.Fatalf("on the shelf the settings are not those of the machine: %q", tt.Texts())
	}
	shut(tt, &a.bk.prefs)
	var got []string
	a.Appearance = func(v string) { got = append(got, v) }
	if err := clickMenu(tt, "视图", "外观-深色"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(got) != 1 || got[0] != "dark" {
		t.Fatalf("on the shelf the appearance was not chosen: %q", got)
	}
	a.menu = menuState{}
	tt.Frame()
	if err := clickMenu(tt, "帮助", "菜单-快捷键"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.keysOpen {
		t.Fatal("on the shelf the shortcuts did not open")
	}
}

// A menu of the bar opened and put away leaves the caret in the prose:
// what is typed next goes on where it was. A press elsewhere keeps the
// focus it took, and what a row opens to be typed in has it.
func TestTheBarGivesTheCaretBack(t *testing.T) {
	a, tt, _ := writingFixture(t, "整条街的灯是同时灭的。")
	w := &a.write
	w.ed.Focus()
	w.ed.MoveTo(0, false)
	tt.Frame()
	tt.Frame()
	typed := func(s, why string) {
		t.Helper()
		tt.Frame()
		tt.Type(s)
		tt.Frame()
		if got := w.ed.Doc.String(); !strings.Contains(got, s) {
			t.Fatalf("%s: the prose did not take %q: %q", why, s, got)
		}
	}
	openMenu(t, a, tt, "文件")
	if !tt.Focused("菜单-新建章节") {
		t.Fatal("the menu's first row does not have the focus")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	typed("甲", "after Escape")

	openMenu(t, a, tt, "编辑")
	if err := tt.Click(barKey + "编辑"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	typed("乙", "after the name clicked again")

	openMenu(t, a, tt, "帮助")
	over, _ := tt.Find(barKey + "视图")
	tt.Move(over.X+over.W/2, over.Y+over.H/2)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("菜单-收起右栏"); err != nil {
		t.Fatal(err, rows(tt))
	}
	tt.Frame()
	if !a.doc.hidden {
		t.Fatal("the row of the menu the pointer went to was not picked")
	}
	typed("丙", "after a row of the menu the pointer went to")
	a.setDocHidden(false)
	tt.Frame()

	if err := tt.Click("收起侧栏"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	typed("丁", "after the switch of the sidebar")
	if err := tt.Click("展开侧栏"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// A dialog opened from a menu gives the caret back as it closes.
	if err := clickMenu(tt, "帮助", "菜单-快捷键"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.keysOpen {
		t.Fatal("the shortcuts did not open")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if a.keysOpen {
		t.Fatal("Escape left the shortcuts open")
	}
	typed("戊", "after the dialog a menu opened")

	// A press on a field puts the menu away and the field keeps the focus.
	openMenu(t, a, tt, "文件")
	if err := tt.Click("搜索设定"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	tt.Frame()
	if a.menu.owner != "" || !tt.Focused("搜索设定") {
		t.Fatalf("a press on the field: menu %q, field focused %v", a.menu.owner, tt.Focused("搜索设定"))
	}
	before := w.ed.Doc.String()
	tt.Type("林")
	tt.Frame()
	if w.ed.Doc.String() != before {
		t.Fatal("the prose took what was typed in the field")
	}

	// What a row opens to be typed in has the focus.
	w.ed.Focus()
	tt.Frame()
	tt.Frame()
	if err := clickMenu(tt, "编辑", "菜单-查找替换"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !w.finding || !tt.Focused("查找") {
		t.Fatalf("查找替换: finding %v, its field focused %v", w.finding, tt.Focused("查找"))
	}
	shut(tt, &w.finding)
	w.ed.Focus()
	tt.Frame()
	tt.Frame()
	if err := clickMenu(tt, "编辑", "菜单-重命名"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if w.renaming == "" || !tt.Focused("章节标题") {
		t.Fatalf("重命名: naming %q, its box focused %v", w.renaming, tt.Focused("章节标题"))
	}
}

// A press held on a menu's name still opens the menu as it is let go.
func TestAMenuOpensOnASlowClick(t *testing.T) {
	a, tt, _ := writingFixture(t, "整条街的灯是同时灭的。")
	a.write.ed.Focus()
	tt.Frame()
	tt.Frame()
	r, _ := tt.Find(barKey + "编辑")
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	tt.Frame()
	tt.Frame()
	tt.Release(x, y)
	tt.Frame()
	if a.menu.owner != barKey+"编辑" {
		t.Fatalf("a slow click did not open the menu: %q", a.menu.owner)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Type("甲")
	tt.Frame()
	if got := a.write.ed.Doc.String(); !strings.Contains(got, "甲") {
		t.Fatalf("after it the prose did not take what was typed: %q", got)
	}
}
