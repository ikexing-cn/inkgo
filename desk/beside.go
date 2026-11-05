package desk

import (
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/locales"
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

// panelTabs switch the writing page's right column between the margin,
// the setting, the talk, the foreshadowings and contributed panels.
func (a *App) panelTabs(c *ui.Context) {
	a.syncExtensions()
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
