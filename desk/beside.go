package desk

import (
	"errors"
	"strings"
	"time"
	"writer/locales"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
	"writer/desk/kit"
	"writer/workspace"
)

const (
	panelWidth = 390 // the right column's width until the author drags it
	panelMin   = 300
	pageMin    = 480
)

// panelWidth is the right column's width in a window win wide: what the
// author dragged it to, no narrower than panelMin nor wider than half the
// window, leaving at least pageMin for the page when there is room.
func (a *App) panelWidth(win float32) float32 {
	w := a.doc.width
	if w == 0 {
		w = panelWidth
		if win < narrow {
			w = 320
		}
	}
	return min(max(w, panelMin), a.panelLimit(win))
}

func (a *App) panelLimit(win float32) float32 {
	side := a.sidebarWidth(win)
	if side > 0 {
		side++ // the sidebar's divider
	}
	return max(panelMin, min(win/2, win-side-1-pageMin))
}

// resizer is the column's left edge, which the author drags to widen it;
// the width is kept once they let go.
func (a *App) resizer(c *ui.Context, width float32) {
	win, _ := c.Size()
	edge := ui.Box(c).Absolute().Left(-4).Top(0).Width(8).FillHeight().Cursor(ui.CursorResizeEW).Label(locales.PaneResize())
	if dx, _, ok := edge.Dragged(); ok {
		if dx != 0 {
			a.doc.width = min(max(width-dx, panelMin), a.panelLimit(win))
			a.doc.resizing = true
		}
	} else if a.doc.resizing {
		a.doc.resizing = false
		a.savePrefs()
	}
}

// paneKey is everything the right column's tab counts read.
type paneKey struct {
	fores foreshadowKey
	ai    bool
	late  int
}

type paneNums struct {
	waiting, open int
}

func (a *App) paneCounts() paneNums {
	key := paneKey{a.foreshadowKey(), a.aiOn(), a.write.lateAfter}
	return a.write.pane.get(key, func() paneNums {
		n := paneNums{open: len(a.ideas())}
		for _, f := range a.foreshadows() {
			if f.Status() == workspace.ForeLost || f.Late(a.write.lateAfter) {
				n.waiting++
			}
		}
		return n
	})
}

// panelTabs switch the writing page's right column between the margin,
// the setting, the talk, the foreshadowings and the phone preview.
func (a *App) panelTabs(c *ui.Context) {
	t := a.pal.Theme
	shown := a.paneTabs.shown(a, a.doc.tab)
	tab := func(entry tabEntry[paneTab]) {
		label := entry.title(a)
		on := shown == entry.id
		b := ui.ButtonBase(c).Row().AlignItems(ui.Center).Padding(4, 10).Radius(kit.RControl).Label("右栏-" + label)
		fg := t.TextMuted
		if on || b.Hovered() {
			fg = t.Text
		}
		b.Children(func() {
			ui.Text(c, label).FontSize(kit.FsControl).TextColor(fg).SingleLine()
			// The count sits inside its own tab; beside the tabs it read
			// as unread talk.
			if n := entry.number(); n > 0 {
				ui.Textf(c, "%d", n).FontSize(kit.FsSmall).TextColor(a.pal.Faint).Padding(0, 0, 0, 4)
			}
		})
		if b.Clicked() && !on {
			a.doc.tab = entry.id
			a.savePrefs()
		}
	}
	ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
		for _, entry := range a.paneTabs {
			if entry.available(a) {
				tab(entry)
			}
		}
	})
}

// talkTab tells whether the column beside the prose shows the talk: the
// author chose it there, and the AI is on. Off, the choice is kept for
// when it is back.
func (a *App) talkTab() bool { return a.paneTabs.shown(a, a.doc.tab) == paneTalk }

// talkPanel is the talk beside the prose: one talk with its turns and the
// composer, which sends what the author is writing with their words, or
// the list of them all.
func (a *App) talkPanel(c *ui.Context) {
	d := &a.doc
	a.pal.PaneTop(c).Children(func() {
		a.panelTabs(c)
	})
	if d.talks {
		a.talkList(c)
		return
	}
	if back, _ := a.pal.PaneBack(c, locales.ChatAll(), ""); back && !a.held() {
		d.talks = true
	}
	a.pal.Hairline(c)
	ui.Column(c).FillWidth().Grow(1).Shrink(1).Padding(0, 20).Children(func() {
		a.chatList(c, true)
	})
	ui.Box(c).FillWidth().Padding(0, 20, 14, 20).Shrink(0).Children(func() { a.composer(c) })
}

// talkList is the talks of the work, the latest first, a row each.
func (a *App) talkList(c *ui.Context) {
	d := &a.doc
	if a.paneFind(c, &d.talkQuery, locales.ChatSearch(), locales.ChatNew(), nil) {
		if id, err := a.blankSession(); err != nil {
			a.SetError(err)
		} else {
			a.switchSession(id)
			d.talks, a.focusTalk = false, true
		}
	}
	q := kit.Trim(d.talkQuery)
	a.pal.PaneRows(c, "talks", func() {
		group := locales.ChatRecent()
		if q != "" {
			group = locales.ChatFound()
		}
		n := 0
		for i := len(a.sessions) - 1; i >= 0; i-- {
			id := a.sessions[i]
			words := a.titles[id]
			if a.blank[id] || a.shelved[id] && id != a.session || q != "" && !strings.Contains(words, q) {
				continue
			}
			if n == 0 {
				a.pal.PaneGroup(c, group)
			}
			n++
			if a.pal.PaneRow(c.Key(id), "讨论-"+id, words, "", talkDay(id, time.Now()), "", false).Clicked() {
				a.switchSession(id)
				d.talks = false
			}
		}
		if n == 0 {
			a.pal.PaneEmpty(c, locales.FindNone(), "")
		}
	})
}

// beside is what the author is writing, sent with a turn from the writing
// page, or nil when nothing is open or they turned it off.
func (a *App) beside() *aiport.Beside {
	w := &a.write
	if a.page != pageWrite || !a.doc.attach || w.node == "" || w.cur().lost {
		return nil
	}
	n, ok := a.book.Tree().Node(w.node)
	if !ok {
		return nil
	}
	if n.Folder {
		return &aiport.Beside{Folder: true, Title: n.Title, VolumeOutline: w.cur().outline.Text}
	}
	out := &aiport.Beside{Title: n.Title, Outline: w.cur().outline.Text}
	if p := n.Parent; p != nil && p.Parent != nil {
		out.Volume = p.Title
		if vol, err := a.Store.ReadChapter(p.ID); err == nil {
			out.VolumeOutline = vol.Outline
		}
	}
	if w.cur().ed != nil {
		s, _ := w.cur().ed.Selection()
		out.Before = string([]rune(w.cur().ed.Doc.String())[:s])
	}
	return out
}

// quoteToTalk takes the words selected into the talk beside the prose.
func (a *App) quoteToTalk() {
	w := &a.write
	if w.cur().ed == nil || w.node == "" {
		return
	}
	s, e := w.cur().ed.Selection()
	text := strings.TrimSpace(w.cur().ed.Doc.Slice(s, e))
	if text == "" {
		return
	}
	title := ""
	if n, ok := a.book.Tree().Node(w.node); ok {
		title = n.Title
	}
	a.quotes = append(a.quotes, workspace.Quote{Chapter: w.node, Title: title, Text: text})
	if a.doc.hidden {
		a.doc.hidden = false
	}
	w.focus = false // the talk is in the column
	a.doc.tab = paneTalk
	a.focusTalk = true
	a.savePrefs()
}

// quoteBlock shows prose quoted in the talk under its chapter; clicking it
// finds the words in the chapter.
func (a *App) quoteBlock(c *ui.Context, q workspace.Quote, key string, remove func()) {
	t := a.pal.Theme
	b := ui.ButtonBase(c.Key(key)).Row().FillWidth().Gap(8).AlignItems(ui.Stretch).Padding(6, 8, 6, 0).Radius(kit.RControl).Label("引用-" + key).Tooltip(locales.ChatLocate())
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Box(c).Width(3).Radius(kit.RRound).Background(a.pal.Ink.Alpha(0.55)).Shrink(0)
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			title := q.Title // as it was named then, unless the tree knows it now
			if n, ok := a.book.Tree().Node(q.Chapter); ok {
				title = n.Title
			}
			ui.Text(c, "《"+title+"》").FontSize(kit.FsNote).TextColor(a.pal.Ink)
			ui.Text(c, kit.Clip(q.Text, 160)).FontSize(kit.FsControl).LineHeight(kit.LhBody).TextColor(t.TextMuted).FillWidth()
		})
		if remove != nil && kit.IconButton(c, kit.IcX, "去掉引用-"+key, ui.Color{}).Tooltip(locales.ChatRemoveCite()).Clicked() {
			remove()
		}
	})
	if b.Clicked() {
		a.findInChapter(q.Chapter, q.Text)
	}
}

// findInChapter opens the chapter on the writing page and selects text in
// it, if the text is still there.
func (a *App) findInChapter(chapter, text string) {
	if _, ok := a.book.Tree().Node(chapter); !ok {
		a.SetError(errors.New(locales.ChatChapterGone()))
		return
	}
	a.page = pageWrite
	if a.write.cur().ed == nil || a.write.node != chapter {
		a.openNode(chapter)
	}
	ed := a.write.cur().ed
	if ed == nil {
		return
	}
	at := strings.Index(ed.Doc.String(), text)
	if at < 0 {
		a.status.Hint(locales.ChatQuoteChanged())
		return
	}
	s := len([]rune(ed.Doc.String()[:at]))
	ed.Select(s, s+len([]rune(text)))
	ed.Focus()
}
