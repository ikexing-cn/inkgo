package desk

import (
	"slices"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/locales"
	"writer/workspace"
)

func (a *App) categoryCounts() map[string]int {
	return a.write.catCounts.get(a.bookKey(), func() map[string]int {
		counts := map[string]int{}
		for _, e := range a.book.Setting().Entries {
			if e.Place != workspace.PlaceDropped {
				counts[e.Category]++
			}
		}
		return counts
	})
}

func (a *App) categories(c *ui.Context) {
	t := a.pal.Theme
	busy := a.held()
	counts := a.categoryCounts()
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(640+2*40).Padding(32, 40, 20, 40).Gap(12).Children(func() {
			ui.Text(c, locales.CmdCategories()).FontSize(kit.FsTitle).FontWeight(700)
			ui.Text(c, locales.CatsNote()).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody).FillWidth()
			ui.Column(c).FillWidth().Children(func() {
				ui.Row(c).FillWidth().Padding(0, 4, 8, 4).Gap(14).Children(func() {
					ui.Text(c, locales.CatsName()).FontSize(kit.FsNote).TextColor(t.TextMuted).Width(110).Shrink(0)
					ui.Text(c, locales.CatsDest()).FontSize(kit.FsNote).TextColor(t.TextMuted)
				})
				for _, cat := range a.Store.Work().Categories {
					row := ui.Row(c.Key(cat.Name)).FillWidth().Height(kit.HBar).Padding(0, 4).Gap(14).AlignItems(ui.Center).Label("分类行-" + cat.Name)
					shown := row.Hovered() || row.FocusWithin()
					row.Children(func() {
						// One with entries asks first, as they will move to 其他;
						// an empty one goes at the click.
						if a.removeCategory == cat.Name && counts[cat.Name] > 0 {
							ui.Text(c, locales.CatsMoveTo(counts[cat.Name], workspace.Other)).FontSize(kit.FsBody).Grow(1).Shrink(1)
							if a.pal.Secondary(c, locales.CommonCancel()).Clicked() {
								a.removeCategory = ""
							}
							if a.pal.Danger(c, locales.CatsConfirmDelete()).Disabled(busy).Clicked() {
								a.dropCategory(cat.Name)
							}
							return
						}
						ui.Text(c, cat.Name).FontSize(kit.FsBody).FontWeight(600).Width(110).Shrink(0).SingleLine().Ellipsis("…")
						ui.Text(c, cat.Target).FontSize(kit.FsBody).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
						ui.Text(c, locales.CatsCount(counts[cat.Name])).FontSize(kit.FsNote).TextColor(t.TextMuted).Shrink(0)
						edit := a.pal.Ghost(c, locales.SettingsEdit(), ui.Color{}).Label("编辑-" + cat.Name).Disabled(busy)
						if !shown {
							edit.Opacity(0)
						}
						if edit.Clicked() {
							a.categoryName, a.categoryTarget, a.categoryShow = cat.Name, cat.Target, true
						}
						// 其他 cannot go: what has no category is kept there.
						del := a.pal.Ghost(c, locales.CommonDelete(), ui.Color{}).Label("删除-" + cat.Name).Disabled(busy || cat.Name == workspace.Other)
						if del.IsDisabled() {
							kit.Pale(del)
						}
						if !shown {
							del.Opacity(0)
						}
						if del.Clicked() {
							if counts[cat.Name] > 0 {
								a.removeCategory = cat.Name
							} else {
								a.dropCategory(cat.Name)
							}
						}
					})
					a.pal.Hairline(c)
				}
			})
			ui.Text(c, locales.CatsFormTitle()).FontSize(kit.FsGroup).FontWeight(600).Padding(8, 0, 0, 0)
			form := ui.Row(c).FillWidth().Gap(10).AlignItems(ui.End)
			if a.categoryShow {
				// Below a long list, it would open out of view.
				form.ScrollIntoView()
				a.categoryShow = false
			}
			form.Children(func() {
				kit.Labeled(c, locales.CatsName(), func() {
					ui.TextInput(c, &a.categoryName).Label(locales.CatsNameLabel()).FillWidth().Disabled(busy)
				}).Grow(1).Basis(0)
				kit.Labeled(c, locales.CatsDest(), func() {
					ui.TextInput(c, &a.categoryTarget).Label(locales.CatsDest()).Placeholder(locales.CatsLandHint(locales.CatsLandDefault())).FillWidth().Disabled(busy)
				}).Grow(1).Basis(0)
				// Opened on a category to change, it can be put away.
				if slices.ContainsFunc(a.Store.Work().Categories, func(cat workspace.Category) bool { return cat.Name == kit.Trim(a.categoryName) }) &&
					a.pal.Secondary(c, locales.CommonCancel()).Label("取消修改分类").Height(kit.HControl).Disabled(busy).Clicked() {
					a.categoryName, a.categoryTarget = "", ""
				}
				if a.pal.Primary(c, locales.CatsSave()).Height(kit.HControl).Disabled(busy || kit.Trim(a.categoryName) == "" || kit.Trim(a.categoryTarget) == "").Clicked() {
					a.SetError(a.saveCategory())
				}
			})
			// A name already there is changed, not added: say which, and
			// what it lands in now, before it is saved.
			if name := kit.Trim(a.categoryName); name != "" {
				note := locales.CatsWillAdd()
				for _, cat := range a.Store.Work().Categories {
					if cat.Name == name {
						note = locales.CatsWillEdit(name, cat.Target)
					}
				}
				ui.Text(c, note).FontSize(kit.FsNote).TextColor(t.TextMuted).FillWidth()
			}
		})
	})
}

func (a *App) saveCategory() error {
	name, target := kit.Trim(a.categoryName), kit.Trim(a.categoryTarget)
	switch { // what is left to fill in is not a failure
	case name == "":
		a.status.Hint(locales.CatsNameEmpty())
		return nil
	case target == "":
		a.status.Hint(locales.CatsLandEmpty())
		return nil
	}
	w := a.Store.Work()
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
	if err := a.Store.SaveWork(w); err != nil {
		return err
	}
	a.status.Did(locales.CatsSaved(name))
	a.categoryName, a.categoryTarget = "", ""
	return nil
}

// dropCategory removes the category name, its entries going to 其他.
func (a *App) dropCategory(name string) {
	err := a.book.RemoveCategory(name)
	if err == nil {
		a.status.Did(locales.CatsDeleted(name))
		err = a.refreshFromSetting()
	}
	a.removeCategory = ""
	a.SetError(err)
}
