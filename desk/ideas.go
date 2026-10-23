package desk

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/workspace"
)

// ideasKey is everything ideas reads: the setting and tree, and whether
// the AI is on.
type ideasKey struct {
	book    *book.Book
	version int
	ai      bool
}

// ideas are what the AI suggests and the author has not decided
// (workspace.Setting.Ideas); with the AI off, there are none.
func (a *App) ideas() []workspace.Idea {
	key := ideasKey{a.book, a.book.Version(), a.aiOn()}
	return a.doc.ideas.get(key, func() []workspace.Idea {
		if !a.aiOn() {
			return nil
		}
		return a.book.Setting().Ideas(a.book.Tree().Order())
	})
}

// asks tells whether e, waiting on the author, asks them something: the
// AI writes a question with a question mark at its end.
func asks(e workspace.Entry) bool {
	text := strings.TrimSpace(e.Text)
	return strings.HasSuffix(text, "？") || strings.HasSuffix(text, "?")
}

// idea is one thing the AI suggests, with what it is about when
// about says, and the author's word on it: a question is not a setting to
// take, its answer is; a change taken holds from its chapter on.
func (a *App) idea(c *ui.Context, it workspace.Idea, about string) {
	d, e, g := &a.doc, it.Entry, it.Change
	text := e.Text
	if g.ID != "" {
		text = g.Text
	}
	ui.Column(c.Key(it.ID())).FillWidth().Gap(8).Children(func() {
		if about != "" {
			ui.Text(c, about).FontSize(kit.FsNote).TextColor(a.pal.Theme.TextMuted).SingleLine().Ellipsis("…").FillWidth()
		}
		ui.Text(c, text).FontSize(kit.FsBody).LineHeight(kit.LhLoose).Selectable().FillWidth()
		kit.PaneButtons(c, func() {
			if a.pal.PaneButton(c, "忽略", false).Label("忽略-" + it.ID()).Clicked() {
				if g.ID != "" {
					a.revise(workspace.Op{Op: workspace.OpDropChange, Entry: e.ID, Change: g.ID})
				} else {
					a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: workspace.PlaceDropped})
				}
			}
			switch {
			case g.ID != "":
				if a.pal.PaneButton(c, "采纳", true).Label("采纳-" + it.ID()).Clicked() {
					a.takeChange(e, g)
				}
			case asks(e):
				if a.pal.PaneButton(c, "回答", true).Label("回答-" + e.ID).Clicked() {
					d.route.answer(e.ID)
					d.text, d.category, d.name, d.names = "", e.Category, "", ""
					if len(e.Names) > 0 { // the answer keeps the names asked about, unless the author takes them out
						d.name, d.names = e.Names[0], strings.Join(e.Names[1:], "、")
					}
				}
			default:
				if a.pal.PaneButton(c, "采纳", true).Label("采纳-" + e.ID).Clicked() {
					a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: workspace.PlaceBody})
				}
			}
		})
	})
}

// ideasPage is all that waits on the author, one after another.
func (a *App) ideasPage(c *ui.Context) {
	d := &a.doc
	if back, _ := a.pal.PaneBack(c, "全部设定", ""); back {
		d.route.closeWaiting()
	}
	open := a.ideas()
	if len(open) == 0 {
		d.route.closeWaiting() // the last was decided
	}
	a.pal.PanePage(c, "ideas", func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "待确认").FontSize(kit.FsGroup).FontWeight(600)
			ui.Spacer(c)
			ui.Textf(c, "%d 条 AI 建议", len(open)).FontSize(kit.FsNote).TextColor(a.pal.Theme.TextMuted)
		})
		for i, it := range open {
			if i > 0 {
				a.pal.Hairline(c)
			}
			words := "新设定"
			if b, ok := a.book.Setting().Entry(it.About); ok {
				words = "关于 " + shortName(b)
				if it.Change.ID != "" {
					words += " · " + kit.FromLabel(a.book.Tree(), it.Change.From)
				}
			}
			a.idea(c, it, words)
		}
	})
}
