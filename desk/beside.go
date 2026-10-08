package desk

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/discussion"
	"writer/workspace"
)

const (
	panelWidth = 392 // the right column's width until the author drags it
	panelMin   = 300
	pageMin    = 480 // the page between the sidebar and the column
)

// panelWidth is the right column's width in a window win wide: what the
// author dragged it to, no narrower than panelMin nor wider than half the
// window, and narrower while the window leaves the page under pageMin.
func (a *App) panelWidth(win float32) float32 {
	w := a.doc.width
	if w == 0 {
		w = panelWidth
	}
	return min(max(w, panelMin), max(min(win/2, win-233-pageMin), panelMin))
}

// resizer is the column's left edge, which the author drags to widen it;
// the width is kept once they let go.
func (a *App) resizer(c *ui.Context, width float32) {
	win, _ := c.Size()
	edge := ui.Box(c).Absolute().Left(-4).Top(0).Width(8).FillHeight().Cursor(ui.CursorResizeEW).Label("调整右栏宽度")
	if dx, _, ok := edge.Dragged(); ok {
		if dx != 0 {
			a.doc.width = min(max(width-dx, panelMin), max(win/2, panelMin))
			a.doc.resizing = true
		}
	} else if a.doc.resizing {
		a.doc.resizing = false
		a.savePrefs()
	}
}

// panelTabs switch the writing page's right column between the setting
// and the talk.
func (a *App) panelTabs(c *ui.Context) {
	t := a.pal.theme
	tab := func(label string, talk bool) {
		on := a.doc.talk == talk
		b := ui.ButtonBase(c).Padding(4, 10).Radius(7).Label("右栏-" + label)
		if on {
			b.Background(t.SurfacePressed)
		} else if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() {
			text := ui.Text(c, label).FontSize(14)
			if on {
				text.FontWeight(600)
			} else {
				text.TextColor(t.TextMuted)
			}
		})
		if b.Clicked() && !on {
			a.doc.talk = talk
			a.savePrefs()
		}
	}
	ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
		tab("设定", false)
		tab("讨论", true)
	})
}

// talkPanel is the talk beside the prose: the session, its turns, and the
// composer, which sends what the author is writing with their words.
func (a *App) talkPanel(c *ui.Context) {
	t := a.pal.theme
	ui.Row(c).FillWidth().Height(34).Gap(4).AlignItems(ui.Center).Children(func() {
		a.panelTabs(c)
		ui.Spacer(c)
		a.bookDot(c)
		if iconButton(c, icPlus, "新讨论", ui.Color{}).Tooltip("开一条新讨论").Disabled(a.busy != "").Clicked() {
			if id, err := a.Store.NewSession(); err != nil {
				a.setError(err)
			} else {
				a.switchSession(id)
			}
		}
		if iconButton(c, icPanel, "收起右栏", ui.Color{}).Tooltip("收起右栏").Clicked() {
			a.setDocHidden(true)
		}
	})
	_, words := a.sessionParts(a.session)
	pick := ui.ButtonBase(c).Row().FillWidth().Gap(6).AlignItems(ui.Center).Padding(5, 8).Radius(7).Border(1, t.Border).Label("切换讨论").Disabled(a.busy != "")
	if pick.Hovered() {
		pick.Background(t.SurfaceHover)
	}
	pick.Children(func() {
		glyph(c, icChat, 14, t.TextMuted)
		ui.Text(c, words).FontSize(13).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
		glyph(c, icChevronDown, 13, a.pal.faint)
	})
	pick.Menu(func(m *ui.Menu) {
		for n := len(a.sessions) - 1; n >= 0; n-- {
			id := a.sessions[n]
			when, words := a.sessionParts(id)
			if m.Item(clip(words, 24) + "  " + when).Checked(id == a.session).Chosen() {
				a.switchSession(id)
			}
		}
	})
	ui.Column(c).FillWidth().Grow(1).Shrink(1).Children(func() {
		a.chatList(c, true)
	})
	a.composer(c)
}

// beside is what the author is writing, sent with a turn from the writing
// page, or nil when nothing is open or they turned it off.
func (a *App) beside() *discussion.Writing {
	w := &a.write
	if a.page != 3 || !a.doc.attach || w.node == "" || w.lost {
		return nil
	}
	n, ok := w.tree.Node(w.node)
	if !ok {
		return nil
	}
	if n.Folder {
		return &discussion.Writing{Folder: true, Title: n.Title, VolumeOutline: w.outline}
	}
	out := &discussion.Writing{Title: n.Title, Outline: w.outline}
	if p := n.Parent; p != nil && p.Parent != nil {
		out.Volume = p.Title
		if vol, err := a.Store.ReadChapter(p.ID); err == nil {
			out.VolumeOutline = vol.Outline
		}
	}
	if w.ed != nil {
		s, _ := w.ed.Selection()
		out.Before = string([]rune(w.ed.Doc.String())[:s])
	}
	return out
}

// quoteToTalk takes the words selected into the talk beside the prose.
func (a *App) quoteToTalk() {
	w := &a.write
	if w.ed == nil || w.node == "" {
		return
	}
	s, e := w.ed.Selection()
	text := strings.TrimSpace(w.ed.Doc.Slice(s, e))
	if text == "" {
		return
	}
	title := ""
	if n, ok := w.tree.Node(w.node); ok {
		title = n.Title
	}
	a.quotes = append(a.quotes, workspace.Quote{Chapter: w.node, Title: title, Text: text})
	if a.doc.hidden {
		a.doc.hidden = false
	}
	a.doc.talk = true
	a.focusTalk = true
	a.savePrefs()
}

// quoteBlock shows prose quoted in the talk under its chapter; clicking it
// finds the words in the chapter.
func (a *App) quoteBlock(c *ui.Context, q workspace.Quote, key string, remove func()) {
	t := a.pal.theme
	b := ui.ButtonBase(c).Key(key).Row().FillWidth().Gap(8).AlignItems(ui.Stretch).Padding(6, 8, 6, 0).Radius(6).Label("引用-" + key).Tooltip("在正文里找到这段")
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Box(c).Width(3).Radius(2).Background(a.pal.brand.Alpha(0.5)).Shrink(0)
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, "《"+q.Title+"》").FontSize(12).TextColor(a.pal.brand)
			ui.Text(c, clip(q.Text, 160)).FontSize(13).LineHeight(1.6).TextColor(t.TextMuted).FillWidth()
		})
		if remove != nil && iconButton(c, icX, "去掉引用-"+key, ui.Color{}).Tooltip("去掉这段引用").Clicked() {
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
	if !a.write.entered {
		a.write.entered = true
		a.enterWriting()
	}
	if _, ok := a.write.tree.Node(chapter); !ok {
		a.Error = "这一章已经不在了"
		return
	}
	a.page = 3
	if a.write.node != chapter {
		a.openNode(chapter)
	}
	ed := a.write.ed
	if ed == nil {
		return
	}
	at := strings.Index(ed.Doc.String(), text)
	if at < 0 {
		a.notice = "这段原文已经改过，没有在这一章里找到"
		return
	}
	s := len([]rune(ed.Doc.String()[:at]))
	ed.Select(s, s+len([]rune(text)))
	ed.Focus()
}
