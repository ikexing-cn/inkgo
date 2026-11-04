package lore

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/locales"
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
func (a *Panel) ideas() []workspace.Idea {
	key := ideasKey{a.services.Book.Content(), a.services.Book.Content().Version(), a.services.AI.AIEnabled()}
	return a.Doc.ideas.get(key, func() []workspace.Idea {
		if !a.services.AI.AIEnabled() {
			return nil
		}
		return a.services.Book.Content().Setting().Ideas(a.services.Book.Content().Tree().Order())
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
func (a *Panel) idea(c *ui.Context, it workspace.Idea, about string) {
	d, e, g := &a.Doc, it.Entry, it.Change
	text := e.Text
	if g.ID != "" {
		text = g.Text
	}
	ui.Column(c.Key(it.ID())).FillWidth().Gap(8).Children(func() {
		if about != "" {
			ui.Text(c, about).FontSize(kit.FsNote).TextColor(a.services.Palette.Colors().Theme.TextMuted).SingleLine().Ellipsis("…").FillWidth()
		}
		ui.Text(c, text).FontSize(kit.FsBody).LineHeight(kit.LhLoose).Selectable().FillWidth()
		kit.PaneButtons(c, func() {
			if g.ID != "" {
				for i, act := range a.changeActions(it) {
					if a.services.Palette.Colors().PaneButton(c, act.label, i == 1).Label(act.label + "-" + it.ID()).Clicked() {
						act.run()
					}
				}
				return
			}
			if a.services.Palette.Colors().PaneButton(c, locales.CommonIgnore(), false).Label("忽略-" + it.ID()).Clicked() {
				a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: workspace.PlaceDropped})
			}
			switch {
			case asks(e):
				if a.services.Palette.Colors().PaneButton(c, locales.CommonAnswer(), true).Label("回答-" + e.ID).Clicked() {
					d.Route.Answer(e.ID)
					d.Text, d.Category, d.Name, d.Names = "", e.Category, "", ""
					if len(e.Names) > 0 { // the answer keeps the names asked about, unless the author takes them out
						d.Name, d.Names = e.Names[0], strings.Join(e.Names[1:], "、")
					}
				}
			default:
				if a.services.Palette.Colors().PaneButton(c, locales.EditorAccept(), true).Label("采纳-" + e.ID).Clicked() {
					a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: workspace.PlaceBody})
				}
			}
		})
	})
}

// ideasPage is all that waits on the author, one after another.
func (a *Panel) ideasPage(c *ui.Context) {
	d := &a.Doc
	if back, _ := a.services.Palette.Colors().PaneBack(c, locales.LoreAll(), ""); back {
		d.Route.CloseWaiting()
	}
	open := a.ideas()
	if len(open) == 0 {
		d.Route.CloseWaiting() // the last was decided
	}
	a.services.Palette.Colors().PanePage(c, "ideas", func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.LoreToConfirm()).FontSize(kit.FsGroup).FontWeight(600)
			ui.Spacer(c)
			ui.Text(c, locales.LoreAiCount(len(open))).FontSize(kit.FsNote).TextColor(a.services.Palette.Colors().Theme.TextMuted)
		})
		for i, it := range open {
			if i > 0 {
				a.services.Palette.Colors().Hairline(c)
			}
			words := locales.LoreNew()
			if b, ok := a.services.Book.Content().Setting().Entry(it.About); ok {
				words = locales.LoreAbout(shortName(b))
				if it.Change.ID != "" {
					words = locales.LoreAboutFrom(shortName(b), kit.FromLabel(a.services.Book.Content().Tree(), it.Change.From))
				}
			}
			a.idea(c, it, words)
		}
	})
}
