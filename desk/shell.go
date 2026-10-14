package desk

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

func trim(s string) string { return strings.TrimSpace(s) }

// titleHeight is the height of the title bar, and of the top row of each
// pane under it. The window buttons fill the title bar (cmd/poc gives the
// window the same title bar height), so their hover covers the whole row.
const titleHeight = hBar

// narrow collapses the sidebar to icons so the chat keeps a readable width.
const narrow = 1180

func (a *App) View(c *ui.Context) {
	a.poll()
	if !a.menu.seen {
		a.menu = menuState{} // what it belonged to is no longer shown
	}
	a.menu.seen = false
	defer a.nameWindow()
	if a.Store != nil && a.worker == nil {
		a.startWorker(c)
	}
	a.pal = a.light
	if c.Theme().Dark {
		a.pal = a.dark
	}
	c.SetTheme(a.pal.theme)
	if a.page == 1 || a.page == 4 { // the settings are a dialog over the page
		tab := "模型"
		if a.page == 4 {
			tab = a.bk.tab
		}
		a.openPrefs(tab)
		a.page = a.shown
	}
	if a.Store == nil && (a.page == 2 || a.page == 3 || a.page == pageCheck) {
		a.page = 0
	}
	if a.busy == "" {
		a.talking = false
	}
	if a.Store != nil {
		a.pollSummaries(c)
		a.pollChecks(c)
		a.backupDue(c)
	}
	if a.Store != nil && a.shown == 3 && a.page != 3 {
		a.flush()
		a.leaveCheck()
		a.leaveChapter()
		a.write.entered = false
	}
	if a.Store != nil && a.page == pageCheck && a.shown != pageCheck {
		a.loadChecks() // as of the disk, without what was put right meanwhile
		a.pruneChecks()
		a.ck.proseOf = ""
	}
	if a.Store != nil && a.shown != a.page && (a.page == 0 || a.page == 3) {
		a.leftWriting = a.page == 3
		a.savePrefs() // the work reopens on the page it was left on
	}
	a.shown = a.page
	a.barAct(c)
	if c.Shortcut(ui.Cmd, ui.KeySlash) {
		a.keysOpen = true // Esc puts it away, as any dialog
	}
	ui.Column(c).Fill().Background(a.pal.canvas).Children(func() {
		if !a.focused() {
			a.titleBar(c)
		}
		ui.Row(c).FillWidth().Grow(1).Shrink(1).AlignItems(ui.Stretch).Children(func() {
			if a.sideShown() {
				a.sidebar(c)
				ui.Box(c).Width(1).FillHeight().Background(a.pal.line)
			}
			ui.Column(c).Grow(1).Shrink(1).FillHeight().Children(func() {
				switch {
				case a.page == 2:
					a.categories(c)
				case a.page == 3:
					a.writing(c)
				case a.page == pageCheck && a.Store != nil:
					a.checkPage(c)
				case a.Store == nil:
					a.welcome(c)
				default:
					a.discussion(c)
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
		a.askView(c)
		if a.write.trashOpen { // the trash is at the foot of every page
			a.trashView(c)
		}
		if a.write.exportOpen { // the menu of files exports from every page
			a.exportView(c)
		}
	}
	a.toasts(c) // over every dialog
}

// focused says the prose is shown alone: no title bar, no sidebar, no pane.
func (a *App) focused() bool { return a.Store != nil && a.page == 3 && a.write.focus }

// sideShown says whether the sidebar is on screen.
func (a *App) sideShown() bool { return a.Store != nil && !a.sideHidden && !a.focused() }

// sessionParts splits a session title into its date and its words.
func (a *App) sessionParts(id string) (when, words string) {
	title := a.titles[id]
	if n := strings.Index(title, " · "); n >= 0 {
		return title[:n], title[n+len(" · "):]
	}
	return "", title
}

func (a *App) sidebar(c *ui.Context) {
	t := a.pal.theme
	w, _ := c.Size()
	slim := w < narrow && a.page != 3 // the writing page needs its tree
	width := float32(232)
	if slim {
		width = 60
	}
	busy := a.busy != ""
	ui.Column(c).Width(width).FillHeight().Background(a.pal.side).Shrink(0).Children(func() {
		work := a.Store.Work()
		var head ui.Element
		// The work, on a row of its own as high as the toolbar beside it.
		top := ui.Row(c).FillWidth().Height(titleHeight).Padding(0, 10).AlignItems(ui.Center).Shrink(0)
		top.Children(func() {
			head = ui.ButtonBase(c).Row().Height(hControlSm).AlignItems(ui.Center).Gap(8).Padding(0, 6, 0, 10).Radius(rControl).Shrink(1).
				Label("切换作品").Tooltip("切换作品").Disabled(busy)
			if head.Hovered() || a.menuOpen("作品") {
				head.Background(t.SurfacePressed)
			}
			head.Children(func() {
				ui.Box(c).Size(hTag, hTag).Radius(rXs).Background(a.pal.ink).Center().Shrink(0).Children(func() {
					ui.Text(c, string([]rune(work.Name + " ")[:1])).FontSize(fsSmall).FontWeight(600).TextColor(a.pal.onInk)
				})
				if !slim {
					ui.Text(c, work.Name).FontSize(fsBody).FontWeight(600).Shrink(1).SingleLine().Ellipsis("…")
					glyph(c, icChevronDown, iconSm, a.pal.faint)
				}
			})
		})
		var chosen func()
		a.dropMenu(c, head, "作品", func(panel ui.Element) {
			// As wide as the row it hangs under, where that is wide enough.
			panel.Margin(4, 0, 0, 0).Width(max(top.Bounds().W-20, menuWide))
		}, func(m *menu) {
			// The work open first, marked; each with what kind it is.
			m.Head("作品")
			m.Item(work.Name).Hint(work.Genre).On(true).Chosen()
			n := 0
			for _, b := range a.shelf.Recent() {
				if sameFolder(b.Path, a.Store.Root()) {
					continue
				}
				if n == 5 {
					break
				}
				n++
				path := b.Path
				if m.Item(b.Name).Hint(b.Genre).Disabled(a.books.missing[path]).Chosen() {
					chosen = func() { a.switchWork(path) }
				}
			}
			m.Separator()
			if m.Item("新建作品…").Chosen() {
				chosen = a.openCreate
			}
			if m.Item("全部作品…").Chosen() {
				chosen = a.closeWork
			}
		})
		if chosen != nil {
			chosen()
			if a.Store == nil {
				return
			}
		}
		a.pal.hairline(c)
		ui.Column(c).FillWidth().Grow(1).Padding(12, 10, 14, 10).Gap(2).Children(func() { a.sideBody(c, slim) })
	})
}

// sideBody is the sidebar under the work: the pages, and what the page
// shown lists.
func (a *App) sideBody(c *ui.Context, slim bool) {
	t := a.pal.theme
	busy := a.busy != ""
	a.navItem(c, icChat, "讨论", 0, slim, 0)
	a.navItem(c, icBook, "正文", 3, slim, 0)
	a.navItem(c, icDone, "检查", pageCheck, slim, len(a.checkItems(false)))
	a.navItem(c, icTag, "作品分类", 2, slim, 0)
	ui.Box(c).Height(20)
	defer a.sideFoot(c, slim)
	if a.page == 3 {
		a.chapterTree(c)
		return
	}
	if slim {
		if iconButton(c, icPlus, "新对话", ui.Color{}).Disabled(busy).Clicked() {
			a.newSession()
		}
		ui.Spacer(c)
		return
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, 4, 6, 10).Children(func() {
		ui.Text(c, "总讨论").FontSize(fsNote).TextColor(t.TextMuted).Grow(1)
		if iconButton(c, icPlus, "新对话", ui.Color{}).Disabled(busy).Clicked() {
			a.newSession()
		}
	})
	ui.Scroll(c.Key("sessions")).FillWidth().Grow(1).Gap(2).Children(func() {
		put := 0
		for n := len(a.sessions) - 1; n >= 0; n-- {
			id := a.sessions[n]
			if a.shelved[id] {
				put++
				if !a.showShelved && id != a.session {
					continue
				}
			}
			a.sessionItem(c, id)
		}
		if put > 0 {
			label := fmt.Sprintf("已收起 %d 段对话", put)
			if a.showShelved {
				label = "不显示收起的对话"
			}
			if a.pal.ghost(c, label, ui.Color{}).Label("收起的对话").Margin(6, 0, 0, 4).Clicked() {
				a.showShelved = !a.showShelved
			}
		}
	})
}

// talkShown tells whether the discussion is on screen, on its page or
// beside the prose.
func (a *App) talkShown() bool {
	if a.Store == nil {
		return false
	}
	return a.page == 0 || a.page == 3 && a.doc.talk && !a.doc.fore && !a.doc.hidden
}

// navItem is a page of the sidebar, with how many things wait there.
func (a *App) navItem(c *ui.Context, s *ui.SVG, label string, page int, slim bool, count int) {
	t := a.pal.theme
	on := a.page == page
	b := ui.ButtonBase(c).Row().FillWidth().Height(hControl).AlignItems(ui.Center).Gap(10).Padding(0, 10).Radius(rControl).Label(label).Disabled(a.busy != "")
	if slim {
		b.Tooltip(label).Justify(ui.Center)
	} else {
		b.Justify(ui.Start)
	}
	fg := t.TextMuted
	switch {
	case on:
		b.Background(t.SurfacePressed)
		fg = t.Text
	case b.Hovered():
		b.Background(t.SurfaceHover)
		fg = t.Text
	}
	b.Children(func() {
		glyph(c, s, iconMd, fg)
		if !slim {
			ui.Text(c, label).FontSize(fsBody).TextColor(fg).FontWeight(map[bool]int{true: 600, false: 400}[on]).Grow(1)
			if count > 0 {
				ui.Textf(c, "%d", count).FontSize(fsSmall).TextColor(a.pal.faint)
			}
		}
	})
	if b.Clicked() {
		a.page = page
	}
}

func (a *App) sessionItem(c *ui.Context, id string) {
	t := a.pal.theme
	when, words := a.sessionParts(id)
	on := id == a.session && a.page == 0
	b := ui.ButtonBase(c.Key("s-"+id)).Row().FillWidth().Justify(ui.Start).AlignItems(ui.Center).Gap(8).Padding(6, 10).Radius(rControl).Label("对话-" + id).Disabled(a.busy != "")
	if on {
		b.Background(t.SurfacePressed)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		fg := t.TextMuted
		if on {
			fg = t.Text
		}
		if a.shelved[id] {
			fg = a.pal.faint
		}
		ui.Text(c, words).FontSize(fsControl).TextColor(fg).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
		ui.Text(c, when).FontSize(fsSmall).TextColor(a.pal.faint).SingleLine()
	})
	a.contextMenu(c, b, "对话-"+id, func(m *menu) {
		if a.shelved[id] {
			if m.Item("移回列表").Disabled(a.busy != "").Chosen() {
				delete(a.shelved, id)
				a.savePrefs()
			}
		} else if m.Item("收起这段对话").Disabled(a.busy != "").Chosen() {
			a.shelve(id)
		}
	})
	if b.Clicked() {
		a.page = 0
		a.switchSession(id)
	}
}

func (a *App) newSession() {
	session, err := a.Store.NewSession()
	if err != nil {
		a.setError(err)
		return
	}
	a.page = 0
	a.switchSession(session)
}

// WindowTitle names the work shown, and on the writing page the chapter
// open in it, so that the taskbar tells windows and works apart.
func (a *App) WindowTitle() string {
	if a.Store == nil {
		return "写作"
	}
	title := a.Store.Work().Name
	if a.page == 3 {
		if n, ok := a.write.tree.Node(a.write.node); ok && n.Title != "" {
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
		if !a.shelved[a.sessions[n]] {
			a.switchSession(a.sessions[n])
			return
		}
	}
	a.newSession()
}
