package desk

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/aiport"
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

// panelTabs switch the writing page's right column between the setting,
// the talk and the foreshadowings.
func (a *App) panelTabs(c *ui.Context) {
	t := a.pal.theme
	// Each count is of what waits on the author there.
	waiting := 0
	for _, f := range a.foreshadows() {
		if f.status() == foreLost {
			waiting++
		}
	}
	open, _ := a.ideas()
	tab := func(label string, talk, fore bool) {
		on := a.doc.fore == fore && (fore || a.talkTab() == talk)
		b := ui.ButtonBase(c).Row().AlignItems(ui.Center).Padding(4, 10).Radius(rControl).Label("右栏-" + label)
		if on {
			b.Background(t.SurfacePressed)
		} else if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() {
			text := ui.Text(c, label).FontSize(fsBody)
			if on {
				text.FontWeight(600)
			} else {
				text.TextColor(t.TextMuted)
			}
			// The count sits inside its own tab; beside the tabs it read
			// as unread talk.
			n := len(open)
			if fore {
				n = waiting
			}
			if n > 0 && !talk {
				ui.Textf(c, "%d", n).FontSize(fsNote).TextColor(a.pal.faint).Padding(0, 0, 0, 4)
			}
		})
		if b.Clicked() && !on {
			a.doc.fore = fore
			if !fore {
				a.doc.talk = talk
			}
			a.savePrefs()
		}
	}
	ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
		tab("设定", false, false)
		if a.aiOn() {
			tab("讨论", true, false)
		}
		tab("伏笔", false, true)
	})
}

// talkTab tells whether the column beside the prose shows the talk: the
// author chose it there, and the AI is on. Off, the choice is kept for
// when it is back.
func (a *App) talkTab() bool { return a.doc.talk && a.aiOn() }

// talkPanel is the talk beside the prose: one talk with its turns and the
// composer, which sends what the author is writing with their words, or
// the list of them all.
func (a *App) talkPanel(c *ui.Context) {
	d := &a.doc
	paneTop(c).Children(func() {
		a.panelTabs(c)
		ui.Spacer(c)
		a.bookDot(c)
		if iconButton(c, icPanel, "收起右栏", ui.Color{}).Tooltip("收起右栏").Clicked() {
			a.setDocHidden(true)
		}
	})
	if d.talks {
		a.talkList(c)
		return
	}
	if back, _ := a.paneBack(c, "全部讨论", ""); back && a.busy == "" {
		d.talks = true
	}
	a.pal.hairline(c)
	ui.Column(c).FillWidth().Grow(1).Shrink(1).Padding(0, 20).Children(func() {
		a.chatList(c, true)
	})
	ui.Box(c).FillWidth().Padding(0, 20, 14, 20).Shrink(0).Children(func() { a.composer(c) })
}

// talkList is the talks of the work, the latest first, a row each.
func (a *App) talkList(c *ui.Context) {
	d := &a.doc
	if a.paneFind(c, &d.talkQuery, "搜索讨论", "新讨论", nil) {
		if id, err := a.Store.NewSession(); err != nil {
			a.setError(err)
		} else {
			a.switchSession(id)
			d.talks, a.focusTalk = false, true
		}
	}
	q := trim(d.talkQuery)
	a.paneRows(c, "talks", func() {
		group := "最近"
		if q != "" {
			group = "找到的"
		}
		n := 0
		for i := len(a.sessions) - 1; i >= 0; i-- {
			id := a.sessions[i]
			when, words := a.sessionParts(id)
			if a.shelved[id] && id != a.session || q != "" && !strings.Contains(words, q) {
				continue
			}
			if n == 0 {
				a.paneGroup(c, group)
			}
			n++
			if a.paneRow(c.Key(id), "讨论-"+id, words, "", talkDay(when, time.Now()), "", false).Clicked() {
				a.switchSession(id)
				d.talks = false
			}
		}
		if n == 0 {
			a.paneEmpty(c, "未找到", "")
		}
	})
}

// talkDay is the day of a talk in the fewest words, from when as its title
// has it: 今天, 昨天, or the month and day.
func talkDay(when string, now time.Time) string {
	day, _, _ := strings.Cut(when, " ")
	switch day {
	case now.Format("01-02"):
		return "今天"
	case now.AddDate(0, 0, -1).Format("01-02"):
		return "昨天"
	}
	return day
}

// beside is what the author is writing, sent with a turn from the writing
// page, or nil when nothing is open or they turned it off.
func (a *App) beside() *aiport.Beside {
	w := &a.write
	if a.page != 3 || !a.doc.attach || w.node == "" || w.lost {
		return nil
	}
	n, ok := w.tree.Node(w.node)
	if !ok {
		return nil
	}
	if n.Folder {
		return &aiport.Beside{Folder: true, Title: n.Title, VolumeOutline: w.outline}
	}
	out := &aiport.Beside{Title: n.Title, Outline: w.outline}
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
	w.focus = false // the talk is in the column
	a.doc.talk, a.doc.fore = true, false
	a.focusTalk = true
	a.savePrefs()
}

// quoteBlock shows prose quoted in the talk under its chapter; clicking it
// finds the words in the chapter.
func (a *App) quoteBlock(c *ui.Context, q workspace.Quote, key string, remove func()) {
	t := a.pal.theme
	b := ui.ButtonBase(c.Key(key)).Row().FillWidth().Gap(8).AlignItems(ui.Stretch).Padding(6, 8, 6, 0).Radius(rControl).Label("引用-" + key).Tooltip("在正文中定位此段")
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Box(c).Width(3).Radius(rRound).Background(a.pal.ink.Alpha(0.55)).Shrink(0)
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			title := q.Title // as it was named then, unless the tree knows it now
			if n, ok := a.write.tree.Node(q.Chapter); ok {
				title = n.Title
			}
			ui.Text(c, "《"+title+"》").FontSize(fsNote).TextColor(a.pal.ink)
			ui.Text(c, clip(q.Text, 160)).FontSize(fsControl).LineHeight(lhBody).TextColor(t.TextMuted).FillWidth()
		})
		if remove != nil && iconButton(c, icX, "去掉引用-"+key, ui.Color{}).Tooltip("移除此段引用").Clicked() {
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
		a.Error = "该章已不存在"
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
		a.notice = "该段原文已修改，在本章中未找到"
		return
	}
	s := len([]rune(ed.Doc.String()[:at]))
	ed.Select(s, s+len([]rune(text)))
	ed.Focus()
}
