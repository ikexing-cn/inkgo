package desk

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// idea is a thing the AI suggests that the author has not decided: an
// entry waiting, with the settled entry it is about, if one; or a change of
// a settled entry from a chapter on.
type idea struct {
	e      workspace.Entry  // the entry waiting, or the one the change is of
	change workspace.Change // with an ID, the change
	about  string           // the settled entry it is about
}

// id names the idea in the labels of its buttons.
func (i idea) id() string {
	if i.change.ID != "" {
		return i.change.ID
	}
	return i.e.ID
}

// ideas are what the AI suggests and the author has not decided: the
// entries waiting, each about the settled one that shares a name with it,
// or whose name it says; then the changes of settled entries, each about
// its own, while its chapter is in the tree. With the AI off, there are
// none.
func (a *App) ideas() []idea {
	if !a.aiOn() {
		return nil
	}
	o := a.write.tree.Order()
	var out, changes []idea
	for _, e := range a.set.Entries {
		if e.Place == workspace.PlaceBody {
			for _, g := range e.Changes {
				if _, ok := o[g.From]; ok && g.Place == workspace.PlaceOpen {
					changes = append(changes, idea{e: e, change: g, about: e.ID})
				}
			}
		}
		if e.Place != workspace.PlaceOpen {
			continue
		}
		it := idea{e: e}
		best := 0
		for _, b := range a.set.Entries {
			if b.Place != workspace.PlaceBody {
				continue
			}
			for _, name := range b.Names {
				n := len(name)
				if slices.Contains(e.Names, name) {
					n += len(e.Text) // a shared name tells more than a mention
				} else if !strings.Contains(e.Text, name) {
					continue
				}
				if n > best {
					best, it.about = n, b.ID
				}
			}
		}
		out = append(out, it)
	}
	return append(out, changes...)
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
func (a *App) idea(c *ui.Context, it idea, about string) {
	d, e, g := &a.doc, it.e, it.change
	text := e.Text
	if g.ID != "" {
		text = g.Text
	}
	ui.Column(c.Key(it.id())).FillWidth().Gap(8).Children(func() {
		if about != "" {
			ui.Text(c, about).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).SingleLine().Ellipsis("…").FillWidth()
		}
		ui.Text(c, text).FontSize(fsBody).LineHeight(lhLoose).Selectable().FillWidth()
		paneButtons(c, func() {
			if a.paneButton(c, "忽略", false).Label("忽略-" + it.id()).Clicked() {
				if g.ID != "" {
					a.revise(workspace.Op{Op: workspace.OpDropChange, Entry: e.ID, Change: g.ID})
				} else {
					a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: workspace.PlaceDropped})
				}
			}
			switch {
			case g.ID != "":
				if a.paneButton(c, "采纳", true).Label("采纳-" + it.id()).Clicked() {
					a.takeChange(e, g)
				}
			case asks(e):
				if a.paneButton(c, "回答", true).Label("回答-" + e.ID).Clicked() {
					d.editing, d.asking, d.text, d.category, d.name, d.names = "new", e.ID, "", e.Category, "", ""
					if len(e.Names) > 0 { // the answer keeps the names asked about, unless the author takes them out
						d.name, d.names = e.Names[0], strings.Join(e.Names[1:], "、")
					}
				}
			default:
				if a.paneButton(c, "采纳", true).Label("采纳-" + e.ID).Clicked() {
					a.revise(workspace.Op{Op: workspace.OpMove, Entry: e.ID, Place: workspace.PlaceBody})
				}
			}
		})
	})
}

// ideasPage is all that waits on the author, one after another.
func (a *App) ideasPage(c *ui.Context) {
	d := &a.doc
	if back, _ := a.paneBack(c, "全部设定", ""); back {
		d.todo = false
	}
	open := a.ideas()
	if len(open) == 0 {
		d.todo = false // the last was decided
	}
	a.panePage(c, "ideas", func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "待确认").FontSize(fsGroup).FontWeight(600)
			ui.Spacer(c)
			ui.Textf(c, "%d 条 AI 建议", len(open)).FontSize(fsNote).TextColor(a.pal.theme.TextMuted)
		})
		for i, it := range open {
			if i > 0 {
				a.pal.hairline(c)
			}
			words := "新设定"
			if b, ok := a.set.Entry(it.about); ok {
				words = "关于 " + shortName(b)
				if it.change.ID != "" {
					words += " · " + fromLabel(a.write.tree, it.change.From)
				}
			}
			a.idea(c, it, words)
		}
	})
}
