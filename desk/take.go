package desk

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/workspace"
)

// openTake starts taking the selected words into the setting.
func (a *App) openTake() {
	w := &a.write
	if w.cur().ed == nil {
		return
	}
	q := strings.TrimSpace(w.cur().ed.SelectedText())
	if q == "" {
		return
	}
	w.takeQuote, w.takeText, w.takeOpen = q, q, true
	if w.takeCategory == "" {
		w.takeCategory = a.firstCategory()
	}
}

func (a *App) take(c *ui.Context) {
	t := a.pal.Theme
	w := &a.write
	a.pal.Modal(c, &w.takeOpen, func() {
		ui.Column(c).Width(kit.DialogSmall).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.pal.DialogHead(c, "加入设定", "选中的原文将记为出处；设定内容可改用自己的表述。")
			kit.Labeled(c, "设定内容", func() {
				ui.TextAreaBase(c, &w.takeText).Label("设定内容").FillWidth().Height(88).FontSize(kit.FsBody).LineHeight(kit.LhLoose).
					Padding(8, 10).Radius(kit.RControl).Border(1, t.Border)
			})
			kit.Labeled(c, "设定分类", func() {
				a.pal.Select(c, &w.takeCategory, a.categoryNames()).Label("设定分类").FillWidth()
			})
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if a.pal.Secondary(c, "取消").Clicked() {
					w.takeOpen = false
				}
				if a.pal.Primary(c, "加入").Disabled(strings.TrimSpace(w.takeText) == "").Clicked() {
					a.takeIn()
				}
			})
		})
	})
}

// takeIn adds the author's entry quoting the chapter, saved first so the
// quote is in the text on disk.
func (a *App) takeIn() {
	w := &a.write
	if err := a.saveText(); err != nil {
		a.warn(err)
		return
	}
	r, err := a.book.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{
		Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: w.takeCategory, Text: strings.TrimSpace(w.takeText),
		Sources: []workspace.Source{{Chapter: w.node, Quote: w.takeQuote}},
	}}})
	if err == nil {
		err = a.refreshFromSetting()
	}
	if err != nil {
		a.warn(err)
		return
	}
	a.flash[r.Ops[0].Entry] = time.Now()
	w.takeOpen = false
	a.status.Did("已加入设定正文")
}
