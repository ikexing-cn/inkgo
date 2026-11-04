// Package cats provides the book category editor.
package cats

import (
	"slices"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/cats/catstext"
	"writer/ext/hostapi"
	"writer/locales"
	"writer/workspace"
)

func (a *Page) categoryCounts() map[string]int {
	counts := map[string]int{}
	for _, e := range a.Book.Content().Setting().Entries {
		if e.Place != workspace.PlaceDropped {
			counts[e.Category]++
		}
	}
	return counts
}

// View draws the category editor on the frame thread.
func (a *Page) View(c *ui.Context) {
	t := a.Palette.Colors().Theme
	busy := a.Book.Held()
	counts := a.categoryCounts()
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(640+2*40).Padding(32, 40, 20, 40).Gap(12).Children(func() {
			ui.Text(c, locales.CmdCategories()).FontSize(kit.FsTitle).FontWeight(700)
			ui.Text(c, catstext.CatsNote()).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody).FillWidth()
			ui.Column(c).FillWidth().Children(func() {
				ui.Row(c).FillWidth().Padding(0, 4, 8, 4).Gap(14).Children(func() {
					ui.Text(c, catstext.CatsName()).FontSize(kit.FsNote).TextColor(t.TextMuted).Width(110).Shrink(0)
					ui.Text(c, catstext.CatsDest()).FontSize(kit.FsNote).TextColor(t.TextMuted)
				})
				for _, cat := range a.Book.Store().Work().Categories {
					row := ui.Row(c.Key(cat.Name)).FillWidth().Height(kit.HBar).Padding(0, 4).Gap(14).AlignItems(ui.Center).Label("分类行-" + cat.Name)
					shown := row.Hovered() || row.FocusWithin()
					row.Children(func() {
						// One with entries asks first, as they will move to 其他;
						// an empty one goes at the click.
						if a.Remove == cat.Name && counts[cat.Name] > 0 {
							ui.Text(c, catstext.CatsMoveTo(counts[cat.Name], a.Book.Store().Work().Language.OtherCategory())).FontSize(kit.FsBody).Grow(1).Shrink(1)
							if a.Palette.Colors().Secondary(c, locales.CommonCancel()).Clicked() {
								a.Remove = ""
							}
							if a.Palette.Colors().Danger(c, catstext.CatsConfirmDelete()).Disabled(busy).Clicked() {
								a.Drop(cat.Name)
							}
							return
						}
						ui.Text(c, cat.Name).FontSize(kit.FsBody).FontWeight(600).Width(110).Shrink(0).SingleLine().Ellipsis("…")
						ui.Text(c, cat.Target).FontSize(kit.FsBody).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
						ui.Text(c, catstext.CatsCount(counts[cat.Name])).FontSize(kit.FsNote).TextColor(t.TextMuted).Shrink(0)
						edit := a.Palette.Colors().Ghost(c, locales.SettingsEdit(), ui.Color{}).Label("编辑-" + cat.Name).Disabled(busy)
						if !shown {
							edit.Opacity(0)
						}
						if edit.Clicked() {
							a.Name, a.Target, a.Show = cat.Name, cat.Target, true
						}
						// 其他 cannot go: what has no category is kept there.
						del := a.Palette.Colors().Ghost(c, locales.CommonDelete(), ui.Color{}).Label("删除-" + cat.Name).Disabled(busy || cat.Name == a.Book.Store().Work().Language.OtherCategory())
						if del.IsDisabled() {
							kit.Pale(del)
						}
						if !shown {
							del.Opacity(0)
						}
						if del.Clicked() {
							if counts[cat.Name] > 0 {
								a.Remove = cat.Name
							} else {
								a.Drop(cat.Name)
							}
						}
					})
					a.Palette.Colors().Hairline(c)
				}
			})
			ui.Text(c, catstext.CatsFormTitle()).FontSize(kit.FsGroup).FontWeight(600).Padding(8, 0, 0, 0)
			form := ui.Row(c).FillWidth().Gap(10).AlignItems(ui.End)
			if a.Show {
				// Below a long list, it would open out of view.
				form.ScrollIntoView()
				a.Show = false
			}
			form.Children(func() {
				kit.Labeled(c, catstext.CatsName(), func() {
					ui.TextInput(c, &a.Name).Label(catstext.CatsNameLabel()).FillWidth().Disabled(busy)
				}).Grow(1).Basis(0)
				kit.Labeled(c, catstext.CatsDest(), func() {
					ui.TextInput(c, &a.Target).Label(catstext.CatsDest()).Placeholder(catstext.CatsLandHint(catstext.CatsLandDefault())).FillWidth().Disabled(busy)
				}).Grow(1).Basis(0)
				// Opened on a category to change, it can be put away.
				if slices.ContainsFunc(a.Book.Store().Work().Categories, func(cat workspace.Category) bool { return cat.Name == kit.Trim(a.Name) }) &&
					a.Palette.Colors().Secondary(c, locales.CommonCancel()).Label("取消修改分类").Height(kit.HControl).Disabled(busy).Clicked() {
					a.Name, a.Target = "", ""
				}
				if a.Palette.Colors().Primary(c, catstext.CatsSave()).Height(kit.HControl).Disabled(busy || kit.Trim(a.Name) == "" || kit.Trim(a.Target) == "").Clicked() {
					a.Notices.Notices().SetError(a.Save())
				}
			})
			// A name already there is changed, not added: say which, and
			// what it lands in now, before it is saved.
			if name := kit.Trim(a.Name); name != "" {
				note := catstext.CatsWillAdd()
				for _, cat := range a.Book.Store().Work().Categories {
					if cat.Name == name {
						note = catstext.CatsWillEdit(name, cat.Target)
					}
				}
				ui.Text(c, note).FontSize(kit.FsNote).TextColor(t.TextMuted).FillWidth()
			}
		})
	})
}

// Save persists the form and refreshes the open book on the frame thread.
func (a *Page) Save() error {
	name, target := kit.Trim(a.Name), kit.Trim(a.Target)
	switch { // what is left to fill in is not a failure
	case name == "":
		a.Notices.Notices().Hint(catstext.CatsNameEmpty())
		return nil
	case target == "":
		a.Notices.Notices().Hint(catstext.CatsLandEmpty())
		return nil
	}
	w := a.Book.Store().Work()
	updated := false
	for n, cat := range w.Categories {
		if cat.Name == name {
			w.Categories[n].Target = target
			updated = true
		}
	}
	if !updated {
		w.Categories = workspace.AddCategory(w.Categories, workspace.Category{Name: name, Target: target})
	}
	if err := a.Book.Store().SaveWork(w); err != nil {
		return err
	}
	if err := a.Book.CategoriesChanged(); err != nil {
		return err
	}
	a.Notices.Notices().Did(catstext.CatsSaved(name))
	a.Name, a.Target = "", ""
	return nil
}

// Drop removes a category and refreshes the open book on the frame thread.
func (a *Page) Drop(name string) {
	err := a.Book.Content().RemoveCategory(name)
	if err == nil {
		a.Notices.Notices().Did(catstext.CatsDeleted(name))
		err = a.Book.CategoriesChanged()
	}
	a.Remove = ""
	a.Notices.Notices().SetError(err)
}

// Page owns category form state for a book session.
type Page struct {
	Book                 hostapi.Book
	Palette              hostapi.Palette
	Notices              hostapi.Notices
	Name, Target, Remove string
	Show                 bool
}
