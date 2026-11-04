package lore

import (
	"slices"
	"strconv"
	"writer/ext/lore/loretext"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/locales"
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
func (a *Panel) here() string {
	if !a.services.Desktop.Writing() {
		return ""
	}
	if _, ok := a.services.Book.Content().Tree().Node(a.selected()); ok {
		return a.selected()
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
		return loretext.LoreLaterChanges()
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
func (a *Panel) timeline(c *ui.Context, e workspace.Entry) {
	d, t, tree := &a.Doc, a.services.Palette.Colors().Theme, a.services.Book.Content().Tree()
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
		d.Route.EditChange(g.ID)
		d.From, d.Text = g.From, g.Text
	}
	add := func() {
		label := loretext.LoreChangeFromChapter()
		if n, _ := tree.Node(here); n != nil && n.Folder {
			label = loretext.LoreChangeFromVolume()
		}
		if a.services.Palette.Colors().Ghost(c, label, ui.Color{}).Clicked() {
			d.Route.EditChange("new")
			d.From, d.Text = here, e.ReadingBefore(o, here)
		}
	}
	mine := here != "" && slices.ContainsFunc(hold, func(g workspace.Change) bool { return g.From == here })
	if len(hold)+len(gone) == 0 {
		if here != "" {
			a.services.Palette.Colors().Hairline(c)
			ui.Row(c).FillWidth().Children(add)
		}
		return
	}
	a.services.Palette.Colors().Hairline(c)
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, loretext.LoreChanges()).FontSize(kit.FsNote).FontWeight(600)
		ui.Spacer(c)
		if here != "" && !mine {
			add()
		}
	})
	if here != "" && !holds {
		a.services.Palette.Colors().PaneNote(c, loretext.LoreOriginalHere())
	}
	step := func(g workspace.Change, label string, current, faded bool) {
		ui.Column(c.Key(g.ID)).FillWidth().Gap(4).Children(func() {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, label).FontSize(kit.FsNote).TextColor(t.TextMuted).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
				if current {
					ui.Text(c, loretext.LoreHere()).FontSize(kit.FsSmall).TextColor(a.services.Palette.Colors().Ink).Padding(0, 6).Radius(kit.RXs).Background(t.SurfacePressed).Shrink(0)
				}
				if a.services.Palette.Colors().Ghost(c, loretext.CommonEdit(), ui.Color{}).Label("修改变化-" + g.ID).Clicked() {
					open(g)
				}
			})
			text := ui.Text(c, g.Text).FontSize(kit.FsBody).LineHeight(kit.LhLoose).Selectable().FillWidth()
			if faded {
				text.TextColor(a.services.Palette.Colors().Faint)
			}
		})
	}
	for _, g := range hold {
		step(g, kit.FromLabel(tree, g.From), holds && on.ID == g.ID, false)
	}
	for _, g := range gone {
		step(g, loretext.LoreGoneInactive(), false, true)
	}
}

// changeForm notes how e reads from a chapter on, or changes or deletes
// the change open: from which chapter, and all the entry says from then.
// Saving one from a chapter that has one already puts it in that one's
// place.
func (a *Panel) changeForm(c *ui.Context, e workspace.Entry) {
	d, tree := &a.Doc, a.services.Book.Content().Tree()
	name := shortName(e)
	if back, _ := a.services.Palette.Colors().PaneBack(c, name, ""); back {
		d.Route.CloseChange()
	}
	a.services.Palette.Colors().PanePage(c, "change-"+e.ID, func() {
		ui.Text(c, name+" · "+kit.FromLabel(tree, d.From)).FontSize(kit.FsGroup).FontWeight(600)
		note := loretext.LoreChangeNoteNoAi()
		if a.services.AI.AIEnabled() {
			note = loretext.LoreChangeNoteAi()
		}
		a.services.Palette.Colors().PaneNote(c, note)
		a.services.Palette.Colors().PaneField(c, loretext.LoreFromChapter(), func() {
			names, ids := nodeChoices(tree)
			chosen := locales.AppChapterGone()
			for n, id := range ids {
				if id == d.From {
					chosen = n
				}
			}
			if a.services.Palette.Colors().Select(c, &chosen, names).Label(loretext.LoreFromChapter()).FillWidth().Changed() {
				d.From = ids[chosen]
			}
		})
		a.services.Palette.Colors().PaneField(c, loretext.LoreContent(), func() {
			ui.TextAreaBase(c, &d.Text).Label(loretext.LoreChangedText()).FillWidth().Height(120).FontSize(kit.FsBody).LineHeight(kit.LhLoose).
				Padding(8, 10).Radius(kit.RControl).Background(a.services.Palette.Colors().Canvas).Border(1, a.services.Palette.Colors().Theme.Border).AutoFocus()
		})
		text := kit.Trim(d.Text)
		change := d.Route.ChangeID()
		was := slices.IndexFunc(e.Changes, func(g workspace.Change) bool { return g.ID == change })
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			if was >= 0 && a.services.Palette.Colors().Ghost(c, loretext.LoreDropChange(), a.services.Palette.Colors().Theme.Danger).Clicked() {
				a.revise(workspace.Op{Op: workspace.OpDropChange, Entry: e.ID, Change: change})
				d.Route.CloseChange()
				return
			}
			ui.Spacer(c)
			if a.services.Palette.Colors().PaneButton(c, locales.ToastCancel(), false).Clicked() {
				d.Route.CloseChange()
			}
			if !a.services.Palette.Colors().PaneButton(c, locales.CmdSave(), true).Disabled(text == "").Clicked() {
				return
			}
			a.services.Notices.Notices().SetError(a.saveChange())
			if a.services.Notices.Notices().ErrorText() == "" {
				d.Route.CloseChange()
			}
		})
	})
}

// takeChange is the author taking the AI's change g of e: it holds from
// its chapter on, in the place of one that holds from there.
func (a *Panel) takeChange(e workspace.Entry, g workspace.Change) {
	var ops []workspace.Op
	for _, x := range e.Changes {
		if x.ID != g.ID && x.Place == workspace.PlaceBody && x.From == g.From {
			ops = append(ops, workspace.Op{Op: workspace.OpDropChange, Entry: e.ID, Change: x.ID})
		}
	}
	a.services.Edits.UndoRevision(loretext.LoreNotedTo(shortName(e), kit.FromLabel(a.services.Book.Content().Tree(), g.From)), append(ops, workspace.Op{Op: workspace.OpEditChange, Entry: e.ID, Change: g.ID, Place: workspace.PlaceBody})...)
}

func (a *Panel) saveChange() error {
	d := &a.Doc
	tree := a.services.Book.Content().Tree()
	_, id, _ := d.Route.Screen()
	e, _ := a.services.Book.Content().Setting().Entry(id)
	text := kit.Trim(d.Text)
	if text == "" {
		return nil
	}
	change := d.Route.ChangeID()
	was := slices.IndexFunc(e.Changes, func(g workspace.Change) bool { return g.ID == change })
	var ops []workspace.Op
	_, inTree := tree.Node(d.From)
	for _, g := range e.Changes {
		if g.ID != change && g.Place == workspace.PlaceBody && g.From == d.From && inTree {
			ops = append(ops, workspace.Op{Op: workspace.OpDropChange, Entry: e.ID, Change: g.ID})
		}
	}
	if was < 0 {
		ops = append(ops, workspace.Op{Op: workspace.OpChange, Entry: e.ID, From: d.From, Place: workspace.PlaceBody, Text: text})
	} else if g := e.Changes[was]; g.Text != text || g.From != d.From {
		op := workspace.Op{Op: workspace.OpEditChange, Entry: e.ID, Change: g.ID}
		if g.Text != text {
			op.Text = text
		}
		if g.From != d.From {
			op.From = d.From
		}
		ops = append(ops, op)
	}
	if len(ops) > 0 {
		_, err := a.services.Book.Content().Revise(workspace.Revision{By: workspace.ByAuthor, Ops: ops})
		if err != nil {
			return err
		}
		if err = a.services.Edits.Refresh(); err != nil {
			return err
		}
	}
	d.Route.CloseChange()
	return nil
}
