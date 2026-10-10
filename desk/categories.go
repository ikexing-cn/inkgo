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
		ui.Column(c).FillWidth().MaxWidth(760).Padding(24, 32).Gap(20).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "作品分类").FontSize(fsTitle).Bold()
				ui.Text(c, "设定文档按分类分节，每条设定只归入一个分类。“落点”是它成熟后该去的地方，比如设定库或大纲。").FontSize(fsControl).TextColor(t.TextMuted)
			})
			a.pal.cardBox(c).Padding(6, 0).Gap(0).Children(func() {
				for n, cat := range a.Store.Work().Categories {
					if n > 0 {
						ui.Box(c).FillWidth().Height(1).Background(t.Border)
					}
					ui.Row(c).Key(cat.Name).FillWidth().Padding(10, 20).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Text(c, cat.Name).Bold().Width(120)
						ui.Text(c, cat.Target).TextColor(t.TextMuted).Grow(1)
						ui.Textf(c, "%d 条", counts[cat.Name]).FontSize(fsNote).TextColor(t.TextMuted)
						if a.removeCategory == cat.Name {
							ui.Textf(c, "%d 条设定将移到“其他”", counts[cat.Name]).FontSize(fsNote).TextColor(t.Danger)
							if ghost(c, "取消", ui.Color{}).Clicked() {
								a.removeCategory = ""
							}
							if action(c, "确认删除", t.Danger, true).Disabled(busy).Clicked() {
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
						// a fixed width, so 其他, which cannot go, lines up with the rest
						ui.Row(c).Width(100).Gap(12).Shrink(0).Children(func() {
							if ghost(c, "编辑", t.Accent).Label("编辑-" + cat.Name).Disabled(busy).Clicked() {
								a.categoryName, a.categoryTarget = cat.Name, cat.Target
							}
							if cat.Name != workspace.Other && ghost(c, "删除", t.Danger).Label("删除-"+cat.Name).Disabled(busy).Clicked() {
								a.removeCategory = cat.Name
							}
						})
					})
				}
			})
			a.pal.cardBox(c).Padding(20, 20).Gap(12).Children(func() {
				ui.Text(c, "添加或修改分类").FontSize(fsGroup).Bold()
				ui.Row(c).FillWidth().Gap(12).AlignItems(ui.End).Children(func() {
					labeled(c, "名称（同名即修改）", func() {
						ui.TextInput(c, &a.categoryName).Label("分类名称").FillWidth().Disabled(busy)
					}).Width(200)
					labeled(c, "落点", func() {
						ui.TextInput(c, &a.categoryTarget).Label("落点").Placeholder("例如 设定库").FillWidth().Disabled(busy)
					}).Grow(1)
					if ui.PrimaryButton(c, "保存分类").Disabled(busy || trim(a.categoryName) == "").Clicked() {
						a.setError(a.saveCategory())
					}
				})
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
		return errors.New("请填写落点，比如“设定库”或“大纲”")
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
