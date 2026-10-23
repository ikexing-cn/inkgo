package desk

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"
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
	t := a.pal.theme
	busy := a.held()
	counts := a.categoryCounts()
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(640+2*40).Padding(32, 40, 20, 40).Gap(12).Children(func() {
			ui.Text(c, "作品分类").FontSize(fsTitle).FontWeight(700)
			ui.Text(c, "“落点”指设定成熟后的去处，例如设定库或大纲。").FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody).FillWidth()
			ui.Column(c).FillWidth().Children(func() {
				a.pal.hairline(c)
				for _, cat := range a.Store.Work().Categories {
					ui.Row(c.Key(cat.Name)).FillWidth().Height(hBar).Padding(0, 4).Gap(14).AlignItems(ui.Center).Children(func() {
						// One with entries asks first, as they will move to 其他;
						// an empty one goes at the click.
						if a.removeCategory == cat.Name && counts[cat.Name] > 0 {
							ui.Textf(c, "%d 条设定将移到“其他”", counts[cat.Name]).FontSize(fsBody).Grow(1).Shrink(1)
							if a.pal.secondary(c, "取消").Clicked() {
								a.removeCategory = ""
							}
							if a.pal.danger(c, "确认删除").Disabled(busy).Clicked() {
								a.dropCategory(cat.Name)
							}
							return
						}
						ui.Text(c, cat.Name).FontSize(fsBody).FontWeight(600).Width(110).Shrink(0).SingleLine().Ellipsis("…")
						ui.Text(c, "落点："+cat.Target).FontSize(fsBody).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
						ui.Textf(c, "%d 条", counts[cat.Name]).FontSize(fsNote).TextColor(t.TextMuted).Shrink(0)
						edit := a.pal.ghost(c, "编辑", ui.Color{}).Label("编辑-" + cat.Name).Disabled(busy)
						if edit.Clicked() {
							a.categoryName, a.categoryTarget, a.categoryShow = cat.Name, cat.Target, true
						}
						// 其他 cannot go: what has no category is kept there.
						del := a.pal.ghost(c, "删除", ui.Color{}).Label("删除-" + cat.Name).Disabled(busy || cat.Name == workspace.Other)
						if del.IsDisabled() {
							pale(del)
						}
						if del.Clicked() {
							if counts[cat.Name] > 0 {
								a.removeCategory = cat.Name
							} else {
								a.dropCategory(cat.Name)
							}
						}
					})
					a.pal.hairline(c)
				}
			})
			ui.Text(c, "添加或修改分类").FontSize(fsGroup).FontWeight(600).Padding(8, 0, 0, 0)
			form := ui.Row(c).FillWidth().Gap(10).AlignItems(ui.End)
			if a.categoryShow {
				// Below a long list, it would open out of view.
				form.ScrollIntoView()
				a.categoryShow = false
			}
			form.Children(func() {
				labeled(c, "名称", func() {
					ui.TextInput(c, &a.categoryName).Label("分类名称").FillWidth().Disabled(busy)
				}).Grow(1).Basis(0)
				labeled(c, "落点", func() {
					ui.TextInput(c, &a.categoryTarget).Label("落点").Placeholder("例如 设定库").FillWidth().Disabled(busy)
				}).Grow(1).Basis(0)
				// Opened on a category to change, it can be put away.
				if slices.ContainsFunc(a.Store.Work().Categories, func(cat workspace.Category) bool { return cat.Name == trim(a.categoryName) }) &&
					a.pal.secondary(c, "取消").Label("取消修改分类").Height(hControl).Disabled(busy).Clicked() {
					a.categoryName, a.categoryTarget = "", ""
				}
				if a.pal.primary(c, "保存分类").Height(hControl).Disabled(busy || trim(a.categoryName) == "" || trim(a.categoryTarget) == "").Clicked() {
					a.SetError(a.saveCategory())
				}
			})
			// A name already there is changed, not added: say which, and
			// what it lands in now, before it is saved.
			if name := trim(a.categoryName); name != "" {
				note := "将添加新分类。"
				for _, cat := range a.Store.Work().Categories {
					if cat.Name == name {
						note = fmt.Sprintf("将修改已有的分类“%s”，现在的落点是“%s”。", name, cat.Target)
					}
				}
				ui.Text(c, note).FontSize(fsNote).TextColor(t.TextMuted).FillWidth()
			}
		})
	})
}

func (a *App) saveCategory() error {
	name, target := trim(a.categoryName), trim(a.categoryTarget)
	switch { // what is left to fill in is not a failure
	case name == "":
		a.status.hint("分类名称不能为空")
		return nil
	case target == "":
		a.status.hint("请填写落点，例如“设定库”或“大纲”")
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
	a.status.did(fmt.Sprintf("分类“%s”已保存", name))
	a.categoryName, a.categoryTarget = "", ""
	return nil
}

// dropCategory removes the category name, its entries going to 其他.
func (a *App) dropCategory(name string) {
	err := a.book.RemoveCategory(name)
	if err == nil {
		a.status.did("已删除分类“" + name + "”")
		err = a.refreshFromSetting()
	}
	a.removeCategory = ""
	a.SetError(err)
}
