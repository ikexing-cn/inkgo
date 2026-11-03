package desk

import (
	"fmt"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/locales"
)

// The title bar is the first row of the window, over every page: at its
// left the switch of the sidebar and the menus, at its right the window's
// buttons, with the pages in the middle. Nothing under it keeps room for
// those buttons. Focused, only the chapter's name and tools remain.

const (
	barKey  = "title-menu-" // before a menu's id: the key of its menu, which does not change with the language
	menuBar = 220           // the least width of a menu of the title bar
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
	if ed := a.write.cur().ed; a.bar.pen && ed != nil {
		ed.Focus()
	}
}

// titleBar draws the title bar. Its menus list every command with its
// shortcut; what cannot be done on the page shown is pale, not gone.
func (a *App) titleBar(c *ui.Context) {
	bar := c.TitleBar()
	w := &a.write
	inWork := a.Store != nil
	open, _ := a.chapterOpen()
	var act func() // what was picked
	// A row of the bar's menus does its part once the bar is drawn.
	later := func(do func()) func() { return func() { act = do } }
	held := false // a button of the bar has the focus
	ui.Row(c).FillWidth().Height(kit.TitleHeight).Padding(0, bar.Right, 0, max(10, bar.Left)).Gap(2).AlignItems(ui.Center).
		Background(a.pal.Canvas).Shrink(0).DragWindow().Children(func() {
		if !a.focused() {
			// The shelf has no sidebar to put away: the switch stays, pale, so
			// the menus do not move.
			sw := kit.IconButton(c, kit.IcSide, cmdSide.name(a), ui.Color{}).Margin(0, 4, 0, 0).Disabled(!inWork)
			held = sw.Focused()
			if sw.Clicked() {
				cmdSide.run(a)
				a.penBack()
			}
			held = a.barMenu(c, "file", locales.MenuFile(), func(m *kit.Menu) { a.fileMenu(m, later) }) || held
			held = a.barMenu(c, "edit", locales.MenuEdit(), func(m *kit.Menu) { a.editMenu(m, later) }) || held
			held = a.barMenu(c, "view", locales.MenuView(), func(m *kit.Menu) { a.viewMenu(m, later) }) || held
			held = a.barMenu(c, "help", locales.MenuHelp(), func(m *kit.Menu) { a.helpMenu(m, later) }) || held
		}
		if inWork {
			width, _ := c.Size()
			if a.focused() {
				ui.Box(c).Absolute().Left(width/2).Translate(-width/5, 0).Width(width * 0.4).Height(kit.TitleHeight).Center().Children(func() {
					ui.Text(c, a.shownTitle()).FontSize(kit.FsControl).TextColor(a.pal.Theme.TextMuted).SingleLine().Ellipsis("…").Shrink(1).Label("专注章名")
				})
			} else {
				a.pageSwitch(c, width)
			}
		}
		ui.Spacer(c)
		if inWork && a.page == pageWrite {
			a.writeTools(c)
		}
		if a.focused() {
			ui.Box(c).Size(kit.HControlSm, kit.HControlSm).Margin(0, 8, 0, 2).Shrink(0)
		} else if kit.IconButton(c, kit.IcPanel, cmdPane.name(a), ui.Color{}).Margin(0, 8, 0, 2).Disabled(!a.hasPane()).KeepFocus().Clicked() {
			cmdPane.run(a)
		}
	})
	switch k := &a.bar; {
	case strings.HasPrefix(a.menu.Owner, barKey):
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
		k.pen = open && w.cur().ed.Focused()
	}
	a.bar.outside = false
	if act != nil {
		a.bar.act, a.bar.wait = act, true
		c.Invalidate()
	}
}

// fileMenu lists the title bar's 文件.
func (a *App) fileMenu(m *kit.Menu, later func(func()) func()) {
	a.barItem(m, later, cmdNewChapter)
	a.barItem(m, later, cmdNewFolder)
	m.Separator()
	a.barItem(m, later, cmdSave)
	a.barItem(m, later, cmdSnapshots)
	a.barItem(m, later, cmdExport)
	m.Separator()
	a.barItem(m, later, cmdCategories)
	a.barItem(m, later, cmdTrash)
	m.Separator()
	a.barItem(m, later, cmdSettings)
}

// pageSwitch stays in the middle of the window, even with either side away.
func (a *App) pageSwitch(c *ui.Context, width float32) {
	ui.Row(c).Absolute().Left(0).Top(0).Width(width).Height(kit.HBar).Gap(4).AlignItems(ui.Center).Justify(ui.Center).PassThrough().Label("换页").Children(func() {
		for _, p := range a.pages[:pageCategories] {
			if !p.available(a) {
				continue
			}
			b := ui.ButtonBase(c).Row().Height(kit.HControlSm).Padding(0, 10).Gap(4).AlignItems(ui.Center).Shrink(0).Label(p.title(a)).Disabled(a.held()).KeepFocus()
			fg := a.pal.Theme.TextMuted
			if a.page == p.id || b.Hovered() {
				fg = a.pal.Ink
			}
			b.Children(func() {
				ui.Text(c, p.title(a)).FontSize(kit.FsControl).TextColor(fg).SingleLine()
				if n := p.number(); n > 0 {
					ui.Textf(c, "%d", n).FontSize(kit.FsSmall).TextColor(a.pal.Faint)
				}
			})
			if b.Clicked() {
				a.page = p.id
			}
		}
	})
}

// pageSwitchWidth measures this frame's words, including the check count,
// so notices leave room without borrowing the previous frame's bounds.
func (a *App) pageSwitchWidth(c *ui.Context) float32 {
	width, count := float32(0), 0
	for _, p := range a.pages[:pageCategories] {
		if !p.available(a) {
			continue
		}
		words, _ := c.MeasureText(0, ui.Span{Text: p.title(a), Size: kit.FsControl})
		width += words + 2*10
		if n := p.number(); n > 0 {
			badge, _ := c.MeasureText(0, ui.Span{Text: fmt.Sprint(n), Size: kit.FsSmall})
			width += 4 + badge
		}
		count++
	}
	return width + float32(max(0, count-1))*4
}

func (a *App) hasPane() bool {
	return a.Store != nil && !a.focused() && (a.page == pageTalk || a.page == pageWrite || a.page == pagePlan)
}

func (a *App) paneOpen() bool {
	if !a.hasPane() {
		return false
	}
	if a.page == pagePlan {
		return a.plan.Pane
	}
	return !a.doc.hidden
}

func (a *App) togglePane() {
	if !a.hasPane() {
		return
	}
	if a.page == pagePlan {
		a.plan.Pane = !a.plan.Pane
	} else {
		a.setDocHidden(!a.doc.hidden)
	}
}

// editMenu lists the title bar's 编辑.
func (a *App) editMenu(m *kit.Menu, later func(func()) func()) {
	a.barItem(m, later, cmdUndo)
	a.barItem(m, later, cmdRedo)
	m.Separator()
	a.barItem(m, later, cmdFind)
	a.barItem(m, later, cmdRename)
	if !a.aiOn() {
		return
	}
	m.Separator()
	a.barItem(m, later, cmdContinue)
	a.barItem(m, later, cmdDraft)
}

// viewMenu lists the title bar's 视图.
func (a *App) viewMenu(m *kit.Menu, later func(func()) func()) {
	a.barItem(m, later, cmdSide)
	a.barItem(m, later, cmdPane)
	// The key toggles focus; the row only enters it.
	cmdFocus.item(a, m).Do(later(func() { a.write.focus = true }))
	a.barItem(m, later, cmdPhone)
	m.Separator()
	m.Head(locales.ViewZoom(int(math.Round(float64(a.Zoom()) * 100))))
	a.barItem(m, later, cmdZoomIn)
	a.barItem(m, later, cmdZoomOut)
	a.barItem(m, later, cmdZoomReset)
	m.Separator()
	m.Draw(a.themeChoice)
}

// helpMenu lists the title bar's 帮助.
func (a *App) helpMenu(m *kit.Menu, later func(func()) func()) {
	a.barItem(m, later, cmdHelp)
}

// barItem adds a command to a menu of the title bar, doing its part after
// the bar is drawn.
func (a *App) barItem(m *kit.Menu, later func(func()) func(), cmd *command) {
	cmd.item(a, m).Do(later(cmd.act(a)))
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
func (a *App) barMenu(c *ui.Context, id, name string, build func(m *kit.Menu)) bool {
	t := a.pal.Theme
	key := barKey + id
	b := ui.ButtonBase(c).Row().Height(kit.HControlSm).Padding(0, 8).Center().Radius(kit.RControl).Shrink(0).Label("菜单栏-" + name)
	if b.Hovered() || a.menu.MenuOpen(key) {
		b.Background(t.SurfacePressed)
	}
	b.Children(func() { ui.Text(c, name).FontSize(kit.FsControl).TextColor(t.Text).SingleLine() })
	if b.Hovered() && a.menu.Drop && a.menu.Owner != key && strings.HasPrefix(a.menu.Owner, barKey) {
		a.menu = kit.MenuState{Owner: key, Drop: true}
		c.Invalidate()
	}
	a.menu.DropMenu(c, a.pal, b, key, func(panel ui.Element) {
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
	if a.Store == nil || a.page != pageWrite || w.cur().ed == nil {
		return false, false
	}
	if n, ok := a.book.Tree().Node(w.node); !ok || n.Folder {
		return false, false
	}
	return true, w.cur().ed.ReadOnly() || w.cur().lost || w.cur().external != nil
}
