package desk

import (
	"errors"
	"fmt"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

func (a *App) categories(c *ui.Context) {
	t := a.pal.theme
	busy := a.busy != ""
	counts := map[string]int{}
	for _, e := range a.set.Entries {
		counts[e.Category]++
	}
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(640+2*40).Padding(32, 40, 20, 40).Gap(12).Children(func() {
			ui.Text(c, "作品分类").FontSize(fsTitle).FontWeight(700)
			ui.Text(c, "设定文档按分类分节，每条设定只归入一个分类。“落点”指设定成熟后的去处，例如设定库或大纲。").FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody).FillWidth()
			ui.Column(c).FillWidth().Children(func() {
				a.pal.hairline(c)
				for _, cat := range a.Store.Work().Categories {
					ui.Row(c).Key(cat.Name).FillWidth().Height(hBar).Padding(0, 4).Gap(14).AlignItems(ui.Center).Children(func() {
						if a.removeCategory == cat.Name {
							ui.Textf(c, "%d 条设定将移到“其他”", counts[cat.Name]).FontSize(fsBody).Grow(1).Shrink(1)
							if a.pal.secondary(c, "取消").Clicked() {
								a.removeCategory = ""
							}
							if a.pal.danger(c, "确认删除").Disabled(busy).Clicked() {
								err := a.Store.RemoveCategory(cat.Name)
								if err == nil {
									a.notice = "已删除分类“" + cat.Name + "”"
									err = a.refresh()
								}
								a.removeCategory = ""
								a.setError(err)
							}
							return
						}
						ui.Text(c, cat.Name).FontSize(fsBody).FontWeight(600).Width(110).Shrink(0).SingleLine().Ellipsis("…")
						ui.Text(c, "落点："+cat.Target).FontSize(fsBody).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
						ui.Textf(c, "%d 条", counts[cat.Name]).FontSize(fsNote).TextColor(t.TextMuted).Shrink(0)
						edit := ui.ButtonBase(c).Label("编辑-" + cat.Name).Shrink(0).Disabled(busy)
						edit.Children(func() { ui.Text(c, "编辑").FontSize(fsBody).TextColor(a.pal.ink) })
						if edit.Clicked() {
							a.categoryName, a.categoryTarget = cat.Name, cat.Target
						}
						// 其他 cannot go: what has no category is kept there.
						del := ui.ButtonBase(c).Label("删除-" + cat.Name).Shrink(0).Disabled(busy || cat.Name == workspace.Other)
						if del.IsDisabled() {
							del.Opacity(opacityDisabled)
						}
						del.Children(func() { ui.Text(c, "删除").FontSize(fsBody).TextColor(t.TextMuted) })
						if del.Clicked() {
							a.removeCategory = cat.Name
						}
					})
					a.pal.hairline(c)
				}
			})
			ui.Text(c, "添加或修改分类").FontSize(fsGroup).FontWeight(600).Padding(8, 0, 0, 0)
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.End).Children(func() {
				labeled(c, "名称（同名即修改）", func() {
					ui.TextInput(c, &a.categoryName).Label("分类名称").FillWidth().Disabled(busy)
				}).Grow(1).Basis(0)
				labeled(c, "落点", func() {
					ui.TextInput(c, &a.categoryTarget).Label("落点").Placeholder("例如 设定库").FillWidth().Disabled(busy)
				}).Grow(1).Basis(0)
				if a.pal.primary(c, "保存分类").Height(hControl).Disabled(busy || trim(a.categoryName) == "").Clicked() {
					a.setError(a.saveCategory())
				}
			})
		})
	})
}

func (a *App) saveCategory() error {
	name, target := trim(a.categoryName), trim(a.categoryTarget)
	if name == "" {
		return errors.New("分类名称不能为空")
	}
	if target == "" {
		return errors.New("请填写落点，例如“设定库”或“大纲”")
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
		w.Categories = append(w.Categories, workspace.Category{Name: name, Target: target})
	}
	if err := a.Store.SaveWork(w); err != nil {
		return err
	}
	a.notice = fmt.Sprintf("分类“%s”已保存", name)
	a.categoryName, a.categoryTarget = "", ""
	return nil
}
