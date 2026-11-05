package desk

import (
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/hostapi"
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
	if revision := a.localeRevision.Load(); revision != a.localeShown {
		a.localeShown = revision
		a.titled = ""
	}
	a.extensionFrame(c)
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
	a.windowActivity(hostapi.WindowActivity{Active: active, Returned: returned})
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
	if a.Store != nil {
		a.pollChecks(c)
		a.extensionTicks(c)
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
	a.preferences(c)
	a.createDialog(c)
	a.renameDialog(c)
	if a.Store != nil {
		a.leaveView(c)
	}
	a.extensionOverlays(c)
	a.toasts(c) // over every dialog
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
			if head.Hovered() || a.menu.MenuOpen("book") {
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
			} else if draw := a.pages[a.page].sidebarAction; draw != nil {
				draw(c)
			}

		})
		var chosen func()
		a.menu.DropMenu(c, a.pal, head, "book", func(panel ui.Element) {
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
		ui.Column(c).FillWidth().Grow(1).Padding(10).Gap(2).Children(func() {
			if draw := a.pages[a.page].sidebar; draw != nil {
				draw(c)
			} else {
				a.chapterTree(c)
			}
		})
	})
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

func (a *App) treeShown() bool { return a.pages[a.page].sidebar == nil }
