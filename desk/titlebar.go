package desk

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// The title bar is the first row of the window, over every page: at its
// left the switch of the sidebar and the menus, at its right the window's
// buttons, and between them the window is held. Nothing under it keeps
// room for those buttons. Focused, the prose has the window alone and the
// bar is away with the rest.

const (
	barKey  = "菜单栏-" // before a menu's name: the key of its menu, and its label
	menuBar = 220    // the least width of a menu of the title bar
)

// barState is what the title bar keeps from frame to frame. A press on the
// bar takes the keyboard focus from the prose, and the toolkit gives it
// back to what was pressed: the bar hands it on to the prose, so that a
// menu opened and put away leaves the caret where it was.
type barState struct {
	pen     bool   // the prose had the caret before the bar was pressed
	open    bool   // a menu of the bar was open in the last frame
	outside bool   // it was put away by a press elsewhere, which has the focus now
	act     func() // what was picked in a menu, to do in the next frame
	wait    bool   // the view is being built again in the frame it was picked in
}

// penBack gives the caret back to the prose, if the bar took it.
func (a *App) penBack() {
	if ed := a.write.ed; a.bar.pen && ed != nil {
		ed.Focus()
	}
}

// titleBar draws the title bar. Its menus list every command with its
// shortcut; what cannot be done on the page shown is pale, not gone.
func (a *App) titleBar(c *ui.Context) {
	bar := c.TitleBar()
	w := &a.write
	inWork := a.Store != nil
	writing := inWork && a.page == 3
	open, busy := a.chapterOpen()
	var act func() // what was picked
	// A row of the bar's menus does its part once the bar is drawn.
	later := func(do func()) func() { return func() { act = do } }
	held := false // a button of the bar has the focus
	ui.Row(c).FillWidth().Height(titleHeight).Padding(0, bar.Right, 0, max(10, bar.Left)).Gap(2).AlignItems(ui.Center).
		Background(a.pal.side).Shrink(0).DragWindow().Children(func() {
		side := "收起侧栏"
		if a.sideHidden {
			side = "展开侧栏"
		}
		// The shelf has no sidebar to put away: the switch stays, pale, so
		// the menus do not move.
		sw := iconButton(c, icSide, side, ui.Color{}).Margin(0, 4, 0, 0).Disabled(!inWork)
		held = sw.Focused()
		if sw.Clicked() {
			a.setSideHidden(!a.sideHidden)
			a.penBack()
		}
		held = a.barMenu(c, "文件", func(m *menu) {
			naming := w.renaming != ""
			m.Item("新建章节").Hint(modKey + "N").Disabled(!writing || naming).Do(later(func() { a.newNode(false) }))
			m.Item("新建文件夹").Disabled(!writing || naming).Do(later(func() { a.newNode(true) }))
			m.Separator()
			m.Item("保存").Hint(modKey + "S").Disabled(!open).Do(later(a.saveNow))
			m.Item("快照").Disabled(!open || busy).Do(later(a.openSnapshots))
			m.Item("导出…").Disabled(!inWork).Do(later(a.openExport))
			m.Separator()
			m.Item("设置…").Do(later(func() { a.openPrefs(a.bk.tab) }))
		}) || held
		held = a.barMenu(c, "编辑", func(m *menu) {
			can := open && !busy
			m.Item("撤销").Hint(modKey + "Z").Disabled(!can || !w.ed.CanUndo()).Do(later(w.ed.Undo))
			m.Item("重做").Hint(modKey + "Y").Disabled(!can || !w.ed.CanRedo()).Do(later(w.ed.Redo))
			m.Separator()
			m.Item("查找替换").Hint(modKey + "F").Disabled(!can).Do(later(a.openFind))
			_, shown := w.tree.Node(w.node)
			m.Item("重命名").Hint("F2").Disabled(!writing || !shown || w.renaming != "").Do(later(a.renameOpen))
			if !a.aiOn() {
				return
			}
			m.Separator()
			m.Item("续写一句").Hint(modKey + "J").Disabled(!can).Do(later(a.continueText))
			hint := ""
			if open && strings.TrimSpace(w.outline) == "" {
				hint = "请先填写章纲"
			}
			m.Item("起草本章").Hint(hint).Disabled(!can || hint != "").Do(later(a.openDraft))
		}) || held
		held = a.barMenu(c, "视图", func(m *menu) {
			m.Item(side).Disabled(!inWork).Do(later(func() { a.setSideHidden(!a.sideHidden) }))
			pane := "收起右栏"
			if a.doc.hidden {
				pane = "展开右栏"
			}
			m.Item(pane).Disabled(!inWork || a.page != 0 && a.page != 3).Do(later(func() { a.setDocHidden(!a.doc.hidden) }))
			m.Item("专注写作").Hint("F11").Disabled(!writing).Do(later(func() { w.focus = true }))
			m.Separator()
			m.Draw(a.themeChoice)
		}) || held
		held = a.barMenu(c, "帮助", func(m *menu) {
			m.Item("快捷键…").Hint(modKey + "/").Do(later(func() { a.keysOpen = true }))
		}) || held
	})
	a.pal.hairline(c)
	switch k := &a.bar; {
	case strings.HasPrefix(a.menu.owner, barKey):
		k.open = true
	case k.open: // put away in this frame
		if !k.outside {
			a.penBack()
		}
		k.open = false
	case held:
		// A press on the bar, or the toolkit giving the focus back to what
		// opened a menu or a dialog: on to the prose. Tab does not come
		// here from the prose, which takes it.
		a.penBack()
	default:
		k.pen = open && w.ed.Focused()
	}
	a.bar.outside = false
	if act != nil {
		a.bar.act, a.bar.wait = act, true
		c.Invalidate()
	}
}

// barAct does what was picked in a menu of the title bar, in the frame
// after: the caret is back in the prose by then, so that a dialog opened
// gives the focus back there as it closes, and a box to type in takes it
// from there. The toolkit builds the view a second time in the frame of a
// click, with the menu's parts still alive: that build is let pass.
func (a *App) barAct(c *ui.Context) {
	k := &a.bar
	if k.act == nil {
		return
	}
	if k.wait {
		k.wait = false
		c.Invalidate()
		return
	}
	act := k.act
	k.act = nil
	act()
}

// barMenu is a menu of the title bar: its name, and under it its rows
// while it is open. With one of them open, the pointer over the name of
// another opens that one instead. It reports whether its name has the
// keyboard focus.
func (a *App) barMenu(c *ui.Context, name string, build func(m *menu)) bool {
	t := a.pal.theme
	key := barKey + name
	b := ui.ButtonBase(c).Row().Height(hControlSm).Padding(0, 8).Center().Radius(rControl).Shrink(0).Label(key)
	if b.Hovered() || a.menuOpen(key) {
		b.Background(t.SurfacePressed)
	}
	b.Children(func() { ui.Text(c, name).FontSize(fsControl).TextColor(t.Text).SingleLine() })
	if b.Hovered() && a.menu.drop && a.menu.owner != key && strings.HasPrefix(a.menu.owner, barKey) {
		a.menu = menuState{owner: key, drop: true}
		c.Invalidate()
	}
	a.dropMenu(c, b, key, func(panel ui.Element) {
		panel.Margin(4, 0, 0, 0).MinWidth(menuBar)
		if panel.PressedOutside() {
			a.bar.outside = true
		}
	}, build)
	return b.Focused()
}

// chapterOpen says a chapter is open on the page of prose, and busy that
// it cannot be changed now.
func (a *App) chapterOpen() (open, busy bool) {
	w := &a.write
	if a.Store == nil || a.page != 3 || w.ed == nil {
		return false, false
	}
	if n, ok := w.tree.Node(w.node); !ok || n.Folder {
		return false, false
	}
	return true, w.ed.ReadOnly() || w.lost || w.external != nil
}

// openExport asks what to export, starting from the whole book, its
// paragraphs indented as a text of prose is.
func (a *App) openExport() {
	w := &a.write
	w.exportOpen, w.exportDone, w.exportFrom, w.exportAt, w.indent = true, false, "", 0, true
	// With a chapter open it is the chapter that is offered first.
	if parts := a.exportParts(); len(parts) > 0 && !parts[len(parts)-1].Folder {
		w.exportAt = len(parts)
	}
}
