package desk

import (
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/workspace"
)

// shortName names e in a few words: its name, or its category.
func shortName(e workspace.Entry) string {
	if len(e.Names) > 0 {
		return kit.Clip(e.Names[0], 12)
	}
	return e.Category
}

// here is the node the setting is read at: the chapter or volume open on
// the writing page; elsewhere none, and entries read as they do now.
func (a *App) here() string {
	if a.page != pageWrite {
		return ""
	}
	if _, ok := a.book.Tree().Node(a.write.node); ok {
		return a.write.node
	}
	return ""
}

// changeSide is what a row of the list says at its end of how e reads at
// the node at: from where it reads so, or, read as itself, that it
// changes later.
func changeSide(tree workspace.Tree, o workspace.Order, e workspace.Entry, at string) string {
	if _, g, ok := e.Reading(o, at); ok {
		return kit.FromLabel(tree, g.From)
	}
	if at != "" && slices.ContainsFunc(e.Holding(o), func(g workspace.Change) bool { return o[g.From] > o[at] }) {
		return "之后有变化"
	}
	return ""
}

// nodeChoices are the nodes a change may start from, in reading order:
// each chapter and volume by its title, a title two share told apart by
// its count.
func nodeChoices(tree workspace.Tree) (names []string, ids map[string]string) {
	ids = map[string]string{}
	tree.Walk(func(n *workspace.Node, _ int) {
		name := n.Title
		for i := 2; ids[name] != ""; i++ {
			name = n.Title + " (" + strconv.Itoa(i) + ")"
		}
		names, ids[name] = append(names, name), n.ID
	})
	return names, ids
}

// timeline is how e changes from chapters on, under all it says: each
// change that holds, in the order of the tree, the one the chapter open
// reads as marked 本章适用; then each whose chapter is gone, which holds
// no more. With a chapter or volume open, a change can be noted from it,
// or the one from it is changed.
func (a *App) timeline(c *ui.Context, e workspace.Entry) {
	d, t, tree := &a.doc, a.pal.Theme, a.book.Tree()
	o, here := tree.Order(), a.here()
	hold := e.Holding(o)
	var gone []workspace.Change
	for _, g := range e.Changes {
		if _, ok := o[g.From]; !ok && g.Place == workspace.PlaceBody {
			gone = append(gone, g)
		}
	}
	on, holds := e.At(o, here)
	open := func(g workspace.Change) {
		d.route.editChange(g.ID)
		d.from, d.text = g.From, g.Text
	}
	add := func() {
		label := "从本章起有变化"
		if n, _ := tree.Node(here); n != nil && n.Folder {
			label = "从本卷起有变化"
		}
		if a.pal.Ghost(c, label, ui.Color{}).Clicked() {
			d.route.editChange("new")
			d.from, d.text = here, e.ReadingBefore(o, here)
		}
	}
	mine := here != "" && slices.ContainsFunc(hold, func(g workspace.Change) bool { return g.From == here })
	if len(hold)+len(gone) == 0 {
		if here != "" {
			a.pal.Hairline(c)
			ui.Row(c).FillWidth().Children(add)
		}
		return
	}
	a.pal.Hairline(c)
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "随章节的变化").FontSize(kit.FsNote).FontWeight(600)
		ui.Spacer(c)
		if here != "" && !mine {
			add()
		}
	})
	if here != "" && !holds {
		a.pal.PaneNote(c, "本章适用上面原来的样子。")
	}
	step := func(g workspace.Change, label string, current, faded bool) {
		ui.Column(c.Key(g.ID)).FillWidth().Gap(4).Children(func() {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, label).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
				if current {
					ui.Text(c, "本章适用").FontSize(kit.FsSmall).TextColor(a.pal.Ink).Padding(0, 6).Radius(kit.RXs).Background(t.SurfacePressed).Shrink(0)
				}
				if a.pal.Ghost(c, "修改", ui.Color{}).Label("修改变化-" + g.ID).Clicked() {
					open(g)
				}
			})
			text := ui.Text(c, g.Text).FontSize(kit.FsBody).LineHeight(kit.LhLoose).Selectable().FillWidth()
			if faded {
				text.TextColor(a.pal.Faint)
			}
		})
	}
	for _, g := range hold {
		step(g, kit.FromLabel(tree, g.From), holds && on.ID == g.ID, false)
	}
	for _, g := range gone {
		step(g, "所在章节已删除 · 不起作用", false, true)
	}
}

// changeForm notes how e reads from a chapter on, or changes or deletes
// the change open: from which chapter, and all the entry says from then.
// Saving one from a chapter that has one already puts it in that one's
// place.
func (a *App) changeForm(c *ui.Context, e workspace.Entry) {
	d, tree := &a.doc, a.book.Tree()
	name := shortName(e)
	if back, _ := a.pal.PaneBack(c, name, ""); back {
		d.route.closeChange()
	}
	a.pal.PanePage(c, "change-"+e.ID, func() {
		ui.Text(c, name+" · "+kit.FromLabel(tree, d.from)).FontSize(kit.FsGroup).FontWeight(600)
		note := "从这一章起，这一条就是下面这段话。"
		if a.aiOn() {
			note = "从这一章起，这一条就是下面这段话；起草、改写这一章及以后的章时，AI 看到的是它。"
		}
		a.pal.PaneNote(c, note)
		a.pal.PaneField(c, "从哪一章起", func() {
			names, ids := nodeChoices(tree)
			chosen := "所在章节已删除"
			for n, id := range ids {
				if id == d.from {
					chosen = n
				}
			}
			if ui.Select(c, &chosen, names).Label("从哪一章起").FillWidth().Changed() {
				d.from = ids[chosen]
			}
		})
		a.pal.PaneField(c, "内容", func() {
			ui.TextAreaBase(c, &d.text).Label("变化后的内容").FillWidth().Height(120).FontSize(kit.FsBody).LineHeight(kit.LhLoose).
				Padding(8, 10).Radius(kit.RControl).Background(a.pal.Canvas).Border(1, a.pal.Theme.Border).AutoFocus()
		})
		text := kit.Trim(d.text)
		change := d.route.changeID()
		was := slices.IndexFunc(e.Changes, func(g workspace.Change) bool { return g.ID == change })
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			if was >= 0 && a.pal.Ghost(c, "删除这处变化", a.pal.Theme.Danger).Clicked() {
				a.revise(workspace.Op{Op: workspace.OpDropChange, Entry: e.ID, Change: change})
				d.route.closeChange()
				return
			}
			ui.Spacer(c)
			if a.pal.PaneButton(c, "取消", false).Clicked() {
				d.route.closeChange()
			}
			if !a.pal.PaneButton(c, "保存", true).Disabled(text == "").Clicked() {
				return
			}
			var ops []workspace.Op
			_, inTree := tree.Node(d.from)
			for _, g := range e.Changes {
				if g.ID != change && g.Place == workspace.PlaceBody && g.From == d.from && inTree {
					ops = append(ops, workspace.Op{Op: workspace.OpDropChange, Entry: e.ID, Change: g.ID})
				}
			}
			if was < 0 {
				ops = append(ops, workspace.Op{Op: workspace.OpChange, Entry: e.ID, From: d.from, Place: workspace.PlaceBody, Text: text})
			} else if g := e.Changes[was]; g.Text != text || g.From != d.from {
				op := workspace.Op{Op: workspace.OpEditChange, Entry: e.ID, Change: g.ID}
				if g.Text != text {
					op.Text = text
				}
				if g.From != d.from {
					op.From = d.from
				}
				ops = append(ops, op)
			}
			if len(ops) > 0 {
				a.revise(ops...)
			}
			if a.errorText() == "" {
				d.route.closeChange()
			}
		})
	})
}

// takeChange is the author taking the AI's change g of e: it holds from
// its chapter on, in the place of one that holds from there.
func (a *App) takeChange(e workspace.Entry, g workspace.Change) {
	var ops []workspace.Op
	for _, x := range e.Changes {
		if x.ID != g.ID && x.Place == workspace.PlaceBody && x.From == g.From {
			ops = append(ops, workspace.Op{Op: workspace.OpDropChange, Entry: e.ID, Change: x.ID})
		}
	}
	a.revise(append(ops, workspace.Op{Op: workspace.OpEditChange, Entry: e.ID, Change: g.ID, Place: workspace.PlaceBody})...)
}
