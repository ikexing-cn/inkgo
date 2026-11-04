package talk

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"time"
	"writer/desk/kit"
	"writer/ext/talk/talkapi"
	"writer/ext/talk/talktext"
	"writer/locales"
	"writer/workspace"
)

// talkPaneBody draws the contributed conversation pane below the shared tabs.
// TalkPaneBody runs on the frame thread.
func (a *Panel) TalkPaneBody(c *ui.Context) {
	if a.Talks {
		a.TalkList(c)
		return
	}
	if back, _ := a.palette().PaneBack(c, talktext.ChatAll(), ""); back && !a.held() {
		a.Talks = true
	}
	a.palette().Hairline(c)
	ui.Column(c).FillWidth().Grow(1).Shrink(1).Padding(0, 20).Children(func() {
		a.ChatList(c, true)
	})
	ui.Box(c).FillWidth().Padding(0, 20, 14, 20).Shrink(0).Children(func() { a.Composer(c) })
}

// talkList is the talks of the work, the latest first, a row each.
// TalkList runs on the frame thread.
func (a *Panel) TalkList(c *ui.Context) {
	if a.paneFind(c, &a.TalkQuery, talktext.ChatSearch(), talktext.ChatNew(), nil) {
		if id, err := a.BlankSession(); err != nil {
			a.SetError(err)
		} else {
			a.SwitchSession(id)
			a.Talks, a.FocusTalk = false, true
		}
	}
	q := kit.Trim(a.TalkQuery)
	a.palette().PaneRows(c, "talks", func() {
		group := talktext.ChatRecent()
		if q != "" {
			group = talktext.ChatFound()
		}
		n := 0
		for i := len(a.Sessions) - 1; i >= 0; i-- {
			id := a.Sessions[i]
			words, _ := a.TalkName(id)
			if a.Blank[id] || a.Shelved[id] && id != a.Session || q != "" && !strings.Contains(words, q) {
				continue
			}
			if n == 0 {
				a.palette().PaneGroup(c, group)
			}
			n++
			if a.palette().PaneRow(c.Key(id), "讨论-"+id, words, "", talkDay(id, time.Now()), "", false).Clicked() {
				a.SwitchSession(id)
				a.Talks = false
			}
		}
		if n == 0 {
			a.palette().PaneEmpty(c, locales.FindNone(), "")
		}
	})
}

// beside is what the author is writing, sent with a turn from the writing
// page, or nil when nothing is open or they turned it off.
// Beside runs on the frame thread.
func (a *Panel) Beside() *talkapi.Beside {
	if !a.desktop.Writing() || !a.Attach || a.selected() == "" || a.lost() {
		return nil
	}
	n, ok := a.content().Tree().Node(a.selected())
	if !ok {
		return nil
	}
	if n.Folder {
		return &talkapi.Beside{Folder: true, Title: n.Title, VolumeOutline: a.chapterOutline(a.selected())}
	}
	out := &talkapi.Beside{Title: n.Title, Outline: a.chapterOutline(a.selected())}
	if p := n.Parent; p != nil && p.Parent != nil {
		out.Volume = p.Title
		out.VolumeOutline = a.chapterOutline(p.ID)
	}
	if a.editor() != nil {
		s, _ := a.editor().Selection()
		out.Before = string([]rune(a.editor().Doc.String())[:s])
	}
	return out
}

// quoteToTalk takes the words selected into the talk beside the prose.
// QuoteToTalk runs on the frame thread.
func (a *Panel) QuoteToTalk() {
	if a.editor() == nil || a.selected() == "" {
		return
	}
	s, e := a.editor().Selection()
	text := strings.TrimSpace(a.editor().Doc.Slice(s, e))
	if text == "" {
		return
	}
	title := ""
	if n, ok := a.content().Tree().Node(a.selected()); ok {
		title = n.Title
	}
	a.Quotes = append(a.Quotes, workspace.Quote{Chapter: a.selected(), Title: title, Text: text})
	a.desktop.SetFocus(false) // the talk is in the column
	a.pane.Show("talk")
	a.FocusTalk = true
	a.savePrefs()
}

// quoteBlock shows prose quoted in the talk under its chapter; clicking it
// finds the words in the chapter.
// QuoteBlock runs on the frame thread.
func (a *Panel) QuoteBlock(c *ui.Context, q workspace.Quote, key string, remove func()) {
	t := a.palette().Theme
	b := ui.ButtonBase(c.Key(key)).Row().FillWidth().Gap(8).AlignItems(ui.Stretch).Padding(6, 8, 6, 0).Radius(kit.RControl).Label("引用-" + key).Tooltip(talktext.ChatLocate())
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Box(c).Width(3).Radius(kit.RRound).Background(a.palette().Ink.Alpha(0.55)).Shrink(0)
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			title := q.Title // as it was named then, unless the tree knows it now
			if n, ok := a.content().Tree().Node(q.Chapter); ok {
				title = n.Title
			}
			ui.Text(c, "《"+title+"》").FontSize(kit.FsNote).TextColor(a.palette().Ink)
			ui.Text(c, kit.Clip(q.Text, 160)).FontSize(kit.FsControl).LineHeight(kit.LhBody).TextColor(t.TextMuted).FillWidth()
		})
		if remove != nil && kit.IconButton(c, kit.IcX, "去掉引用-"+key, ui.Color{}).Tooltip(talktext.ChatRemoveCite()).Clicked() {
			remove()
		}
	})
	if b.Clicked() {
		a.findInChapter(q.Chapter, q.Text)
	}
}
