package lore

import (
	"github.com/egoist/mygo/ui"
	"slices"
	"strings"
	"unicode/utf8"
	"writer/desk/kit"
	"writer/ext/hostapi"
	"writer/ext/lore/loretext"
	"writer/locales"
	"writer/workspace"
)

func (a *Panel) named(name string) bool {
	return slices.ContainsFunc(a.services.Book.Content().Setting().Entries, func(e workspace.Entry) bool {
		return e.Place != workspace.PlaceDropped && slices.Contains(e.Names, name)
	})
}

// nameMenu offers to make the selection, a short name, an entry of its
// own in a category of names.
func (a *Panel) nameMenu(sel string) []hostapi.SelectionAction {
	name := strings.TrimSpace(sel)
	n := utf8.RuneCountInString(name)
	if n == 0 || n > 12 || strings.ContainsAny(name, "\n\r\t") {
		return []hostapi.SelectionAction{{Label: loretext.EditorAddName(), Disabled: true}}
	}
	if a.named(name) {
		return []hostapi.SelectionAction{{Label: loretext.EditorAlreadyNamed(), Disabled: true}}
	}
	children := make([]hostapi.SelectionAction, 0)
	for _, cat := range a.services.Book.Store().Work().Categories {
		if slices.Contains(workspace.NameCategories, cat.Name) {
			children = append(children, hostapi.SelectionAction{Label: cat.Name, Run: func() {
				a.revise(workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: cat.Name, Text: name, Names: []string{name}})
				if a.services.Notices.Notices().ErrorText() == "" {
					a.services.Notices.Notices().Did(loretext.EditorAddedToLore(name, cat.Name))
				}
			}})
		}
	}
	return []hostapi.SelectionAction{{Label: loretext.EditorAddName(), Children: children}}
}

// splitNames reads names as the author writes them, apart by spaces or
// the marks that list things.
func splitNames(s string) []string {
	return workspace.CleanNames(strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '　' || strings.ContainsRune("、,，;；/|", r)
	}))
}

// renamed is the name an entry was called before the author renamed it,
// while the prose still says it.
func (a *Panel) noteRename(old, new string) {
	a.renamed = renamed{}
	if old == "" || old == new || a.services.Book.Store() == nil {
		return
	}
	n := a.services.Search.CountText(old)
	if n > 0 {
		a.renamed = renamed{old: old, new: new, count: n}
	}
}

// renameNotice says the prose still has the old name, and replaces it
// through the find review interface when that extension is available.
func (a *Panel) renameNotice(c *ui.Context) {
	r := a.renamed
	if r.count == 0 {
		return
	}
	ui.Row(c).FillWidth().Gap(6).Padding(6, 8, 6, 10).Radius(kit.RCard).Background(a.services.Palette.Colors().Raised).Border(1, a.services.Palette.Colors().Line).AlignItems(ui.Center).Children(func() {
		ui.Text(c, loretext.LoreRenamedLeft(r.count, r.old)).FontSize(kit.FsNote).Grow(1).Shrink(1)
		if a.replacement != nil {
			if review, ok := a.replacement.Get(); ok {
				if a.services.Palette.Colors().Ghost(c, loretext.LoreReplaceAll(), ui.Color{}).Label("将旧名替换为新名").Clicked() {
					a.renamed = renamed{}
					review.Review(r.old, r.new)
				}
			}
		}

		if kit.IconButton(c, kit.IcX, loretext.LoreKeepOld(), ui.Color{}).Tooltip(loretext.LoreKeepOldTip()).Clicked() {
			a.renamed = renamed{}
		}
	})
}

// editSetting initializes the entry form.
func (a *Panel) editSetting(e workspace.Entry) {
	d := &a.Doc
	d.Route.EditEntry(e.ID)
	d.Text, d.Category, d.Name, d.Names = e.Text, e.Category, "", ""
	if len(e.Names) > 0 {
		d.Name, d.Names = e.Names[0], strings.Join(e.Names[1:], "、")
	}
}

func (a *Panel) changeActions(it workspace.Idea) []noteAction {
	return []noteAction{
		{locales.CheckIgnore(), func() {
			a.services.Edits.UndoRevision(locales.CheckIgnored(), workspace.Op{Op: workspace.OpDropChange, Entry: it.Entry.ID, Change: it.Change.ID})
		}},
		{locales.EditorAccept(), func() { a.takeChange(it.Entry, it.Change) }},
	}
}
