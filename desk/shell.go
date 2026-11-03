package desk

import (
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/locales"
	"writer/shelf"
)

// narrow gives both columns and the paper smaller margins.
const narrow = 1180

// page names the pages in title bar order.
type page int

const (
	pageTalk page = iota
	pageWrite
	pagePlan
	pageCheck
	pageCategories
)

// turnPage does what a change of page does: the page left lets go of
// what it held, and the page come to reads what it shows. View runs it
// before anything is built, and pageTurn again as the page is, for a page
// chosen in the title bar or the sidebar during the frame: either way it
// is done once, before the page is built.
func (a *App) turnPage() {
	if a.shown == a.page {
		return
	}
	if a.Store != nil {
		if leave := a.pages[a.shown].leave; leave != nil {
			leave()
		}
		if enter := a.pages[a.page].enter; enter != nil {
			enter()
		}
		// After the entry, so the chapter it opened and the counts it read
		// are what is kept.
		if a.page == pageTalk || a.page == pageWrite {
			a.leftWriting = a.page == pageWrite
			a.savePrefs() // the work reopens on the page it was left on
		}
	}
	a.shown = a.page
}

func (a *App) View(c *ui.Context) {
	defer func() {
		if a.MenuChanged != nil {
			a.MenuChanged(a.SystemMenu())
		}
	}()
	c.SetZoom(a.Zoom())
	a.poll()
	active := !a.windowInactive.Load()
	returned := a.windowReturned.Swap(false)
	if returned {
		a.write.checked = time.Time{}
	}
	a.plan.WindowActivity(active, returned)
	if !a.menu.Seen {
		a.menu = kit.MenuState{} // what it belonged to is no longer shown
	}
	a.menu.Seen = false
	defer a.nameWindow()
	a.pal = a.light
	if c.Theme().Dark {
		a.pal = a.dark
	}
	c.SetTheme(a.pal.Theme)
	a.page = a.pageShown()
	if a.busy == "" {
		a.turn.active = false
	}
	if a.Store != nil {
		a.pollChecks(c)
		a.backupDue(c)
	}
	a.turnPage()
	a.barAct(c)
	a.systemMenuAct()
	ui.Column(c).Fill().Background(a.pal.Canvas).Children(func() {
		a.titleBar(c)
		// Keyed, the page stays the element it was as the title bar and the
		// sidebar come and go, and the prose keeps the keyboard.
		body := ui.Row(c.Key("body")).FillWidth().Grow(1).Shrink(1).AlignItems(ui.Stretch)
		a.foldPaper(c, body)
		body.Children(func() {
			if a.sideShown() || a.foldLeaving(true) {
				a.sidebar(c)
			}
			ui.Column(c.Key("page")).Grow(1).Shrink(1).MinWidth(0).FillHeight().Children(func() {
				a.pageTurn(c)
				if a.Store == nil {
					a.welcome(c)
				} else {
					a.pages[a.page].view(c)
				}
			})
		})
	})
	a.keysHelp(c)
	if a.sourceOpen && a.source != nil {
		a.extraction(c)
	}
	a.preferences(c)
	a.createDialog(c)
	a.renameDialog(c)
	if a.Store != nil {
		a.plan.Dialog(c)
		a.askView(c)
		a.leaveView(c)
		if a.write.trashOpen { // the trash is at the foot of every page
			a.trashView(c)
		}
		if a.exporting.Open { // the menu of files exports from every page
			a.exporting.View(c)
		}
	}
	a.toasts(c) // over every dialog
	if a.page != pageWrite || !a.paneOpen() && !a.foldLeaving(false) || a.focused() || a.paneTabs.shown(a, a.doc.tab) != paneMargin {
		a.write.margin.visible = false
	}
	// A key or menu may have changed the scale during this build.
	c.SetZoom(a.Zoom())
}

// focused says the prose is shown without the sidebar, tabs or pane.
func (a *App) focused() bool { return a.Store != nil && a.page == pageWrite && a.write.focus }

// sideShown says whether the sidebar is on screen.
func (a *App) sideShown() bool { return a.Store != nil && !a.sideHidden && !a.focused() }

func (a *App) sidebarWidth(win float32) float32 {
	if !a.sideShown() {
		return 0
	}
	if win < narrow {
		return 200
	}
	return 232
}

func (a *App) sidebar(c *ui.Context) {
	w, _ := c.Size()
	width := a.sidebarWidth(w)
	leaving := a.foldLeaving(true)
	if leaving {
		width = a.fold.from.sideWidth
	}
	// The sidebar folds with its divider. Leaving, they hang from a place of
	// no width before the page, which draws over them: the prose passes over
	// the sidebar going away, as in the prototype. The place is always there,
	// so that the tree keeps its scroll as it goes.
	place := ui.Box(c.Key("side")).FillHeight().Shrink(0)
	if leaving {
		place.Width(0)
	}
	place.Children(func() {
		folding := ui.Row(c.Key("sidebar")).FillHeight().AlignItems(ui.Stretch)
		if leaving {
			folding.Absolute().Left(0).Top(0)
		}
		a.foldColumn(folding, true, width)
		folding.Children(func() {
			a.sidePanel(c, width)
			ui.Box(c).Width(1).FillHeight().Background(a.pal.Line)
			if leaving {
				foldCover(c)
			}
		})
	})
}

// sidePanel is the sidebar itself: the work over the chapters or talks.
func (a *App) sidePanel(c *ui.Context, width float32) {
	t := a.pal.Theme
	busy := a.busy != ""
	ui.Column(c).Width(width).FillHeight().Background(a.pal.Canvas).Shrink(0).Children(func() {
		work := a.Store.Work()
		var head ui.Element
		// The work shares the tab row's height and bottom rule.
		top := ui.Row(c).FillWidth().Height(kit.TitleHeight).Padding(0, 10).AlignItems(ui.Center).Shrink(0).
			BorderWidth(0, 0, 1, 0).BorderColor(a.pal.Line).Label("侧栏标题行")
		top.Children(func() {
			head = ui.ButtonBase(c).Row().Height(kit.HControlSm).AlignItems(ui.Center).Gap(8).Padding(0, 6, 0, 10).Radius(kit.RControl).Shrink(1).
				Label(locales.SideSwitchBook()).Tooltip(locales.SideSwitchBook()).Disabled(busy)
			if head.Hovered() || a.menu.MenuOpen("作品") {
				head.Background(t.SurfacePressed)
			}
			head.Children(func() {
				ui.Box(c).Size(kit.HTag, kit.HTag).Radius(kit.RXs).Background(a.pal.Ink).Center().Shrink(0).Children(func() {
					ui.Text(c, string([]rune(work.Name + " ")[:1])).FontSize(kit.FsSmall).FontWeight(600).TextColor(a.pal.OnInk)
				})
				ui.Text(c, work.Name).FontSize(kit.FsBody).FontWeight(600).Shrink(1).SingleLine().Ellipsis("…")
				kit.Glyph(c, kit.IcChevronDown, kit.IconSm, a.pal.Faint)
			})
			ui.Spacer(c)
			if a.treeShown() {
				if kit.IconButton(c, kit.IcPlus, locales.CmdNewChapter(), ui.Color{}).Tooltip(locales.SideNewChapterKey(cmdNewChapter.keys())).Disabled(busy).Clicked() {
					a.page = pageWrite
					a.newNode(false)
				}
			} else if kit.IconButton(c, kit.IcPlus, locales.SideNewTalk(), ui.Color{}).Disabled(a.held()).Clicked() {
				a.newSession()
			}
		})
		var chosen func()
		a.menu.DropMenu(c, a.pal, head, "作品", func(panel ui.Element) {
			// As wide as the row it hangs under, where that is wide enough.
			panel.Margin(4, 0, 0, 0).Width(max(top.Bounds().W-20, kit.MenuWide))
		}, func(m *kit.Menu) {
			// The work open first, marked; each with what kind it is.
			m.Head(locales.SideBooks())
			m.Item(work.Name).Hint(genreName(work.Genre)).On(true)
			n := 0
			for _, b := range a.shelf.Recent() {
				if shelf.SameFolder(b.Path, a.Store.Root()) {
					continue
				}
				if n == 5 {
					break
				}
				n++
				path := b.Path
				m.Item(b.Name).Hint(genreName(b.Genre)).Disabled(a.books.missing[path]).Do(func() {
					chosen = func() { a.switchWork(path) }
				})
			}
			m.Separator()
			m.Item(locales.SideNewBook()).Do(func() { chosen = a.openCreate })
			m.Item(locales.SideAllBooks()).Do(func() { chosen = a.closeWork })
		})
		if chosen != nil && !a.Leave(chosen) { // what the AI proposed is let go first
			chosen()
			if a.Store == nil {
				return
			}
		}
		ui.Column(c).FillWidth().Grow(1).Padding(10).Gap(2).Children(func() { a.sideBody(c) })
	})
}

// sideBody lists the chapters, or the talks on the discussion page.
func (a *App) sideBody(c *ui.Context) {
	if a.treeShown() {
		a.chapterTree(c)
		return
	}
	ui.Scroll(c.Key("sessions")).FillWidth().Grow(1).Gap(2).Children(func() {
		put := 0
		for n := len(a.sessions) - 1; n >= 0; n-- {
			id := a.sessions[n]
			if a.blank[id] {
				continue
			}
			if a.shelved[id] {
				put++
				if !a.showShelved && id != a.session {
					continue
				}
			}
			a.sessionItem(c, id)
		}
		if put > 0 {
			label := locales.SideFoldedCount(put)
			if a.showShelved {
				label = locales.SideHideFolded()
			}
			if a.pal.Ghost(c, label, ui.Color{}).Label("收起的对话").Margin(6, 0, 0, 4).Clicked() {
				a.showShelved = !a.showShelved
			}
		}
	})
}

// treeShown tells whether the sidebar lists chapters rather than talks.
func (a *App) treeShown() bool { return a.page != pageTalk || !a.aiOn() }

// talkShown tells whether the discussion is on screen, on its page or
// beside the prose: not the list of talks there, nor the prose alone.
func (a *App) talkShown() bool {
	if a.Store == nil {
		return false
	}
	return a.page == pageTalk || a.page == pageWrite && a.talkTab() && !a.doc.hidden && !a.doc.talks && !a.focused()
}

func (a *App) sessionItem(c *ui.Context, id string) {
	t := a.pal.Theme
	words := a.titles[id]
	on := id == a.session && a.page == pageTalk
	b := ui.ButtonBase(c.Key("s-"+id)).Row().FillWidth().Justify(ui.Start).AlignItems(ui.Center).Gap(8).Padding(6, 10).Radius(kit.RControl).Label("对话-" + id).Disabled(a.held())
	if on {
		b.Background(t.SurfacePressed)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	kit.PressRow(c, b)
	b.Children(func() {
		fg := t.TextMuted
		if on {
			fg = t.Text
		}
		if a.shelved[id] {
			fg = a.pal.Faint
		}
		ui.Text(c, words).FontSize(kit.FsControl).TextColor(fg).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
		if b.Hovered() || b.FocusWithin() || a.menu.MenuOpen("对话-"+id) {
			ui.Text(c, talkDay(id, time.Now())).FontSize(kit.FsSmall).TextColor(a.pal.Faint).SingleLine()
		}
	})
	a.menu.ContextMenu(c, a.pal, b, "对话-"+id, func(m *kit.Menu) {
		if a.shelved[id] {
			m.Item(locales.SideMoveBack()).Disabled(a.held()).Do(func() {
				delete(a.shelved, id)
				a.savePrefs()
			})
		} else {
			m.Item(locales.SideCollapseTalk()).Disabled(a.held()).Do(func() { a.shelve(id) })
		}
	})
	if b.Clicked() {
		a.page = pageTalk
		a.switchSession(id)
	}
}

func (a *App) newSession() {
	session, err := a.blankSession()
	if err != nil {
		a.SetError(err)
		return
	}
	a.page = pageTalk
	a.switchSession(session)
}

// blankSession is a talk to start: the one open or another nothing was said
// in yet, or else a new one, so that new talks left unused do not pile up.
// One put away stays away.
func (a *App) blankSession() (string, error) {
	if a.blank[a.session] && !a.shelved[a.session] {
		return a.session, nil
	}
	for _, id := range a.sessions {
		if a.blank[id] && !a.shelved[id] {
			return id, nil
		}
	}
	return a.Store.NewSession()
}

// WindowTitle names the work shown, and on the writing page the chapter
// open in it, so that the taskbar tells windows and works apart.
func (a *App) WindowTitle() string {
	if a.Store == nil {
		return "Inkgo"
	}
	title := a.Store.Work().Name
	if a.page == pageWrite {
		if n, ok := a.book.Tree().Node(a.write.node); ok && n.Title != "" {
			title = n.Title + " — " + title
		}
	}
	return title
}

// nameWindow sets the window title when it changed.
func (a *App) nameWindow() {
	if a.SetTitle == nil {
		return
	}
	if t := a.WindowTitle(); t != a.titled && a.SetTitle(t) {
		a.titled = t
	}
}

// shelve puts a conversation out of the list, keeping its file: the
// setting cites what was said in it. Shelving the one open opens the
// latest left, or a new one.
func (a *App) shelve(id string) {
	if a.shelved == nil {
		a.shelved = map[string]bool{}
	}
	a.shelved[id] = true
	a.savePrefs()
	if id != a.session {
		return
	}
	for n := len(a.sessions) - 1; n >= 0; n-- {
		if id := a.sessions[n]; !a.shelved[id] && !a.blank[id] {
			a.switchSession(id)
			return
		}
	}
	a.newSession()
}
