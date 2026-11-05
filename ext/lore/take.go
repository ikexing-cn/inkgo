package lore

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/editor"
	"writer/locales"
	"writer/workspace"
)

// openTake starts taking the selected words into the setting.
func (a *Panel) OpenSelection() {
	w := &a.Take
	if a.selectionEditor() == nil {
		return
	}
	q := strings.TrimSpace(a.selectionEditor().SelectedText())
	if q == "" {
		return
	}
	w.Quote, w.Text, w.Open = q, q, true
	if w.Category == "" {
		w.Category = a.firstCategory()
	}
}

func (a *Panel) take(c *ui.Context) {
	t := a.services.Palette.Colors().Theme
	w := &a.Take
	a.services.Palette.Colors().Modal(c, &w.Open, func() {
		ui.Column(c).Width(kit.DialogSmall).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.services.Palette.Colors().DialogHead(c, locales.EditorAddToLore(), locales.DialogTakeAbout())
			kit.Labeled(c, locales.DialogSettingText(), func() {
				ui.TextAreaBase(c, &w.Text).Label(locales.DialogSettingText()).FillWidth().Height(88).FontSize(kit.FsBody).LineHeight(kit.LhLoose).
					Padding(8, 10).Radius(kit.RControl).Border(1, t.Border)
			})
			kit.Labeled(c, locales.DialogSettingCat(), func() {
				a.services.Palette.Colors().Select(c, &w.Category, a.categoryNames()).Label(locales.DialogSettingCat()).FillWidth()
			})
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if a.services.Palette.Colors().Secondary(c, locales.ToastCancel()).Clicked() {
					w.Open = false
				}
				if a.services.Palette.Colors().Primary(c, locales.DialogAdd()).Disabled(strings.TrimSpace(w.Text) == "").Clicked() {
					a.takeIn()
				}
			})
		})
	})
}

// takeIn adds the author's entry quoting the chapter, saved first so the
// quote is in the text on disk.
func (a *Panel) takeIn() {
	w := &a.Take
	if err := a.services.Editing.SaveText(); err != nil {
		a.services.Notices.Notices().Warn(err)
		return
	}
	r, err := a.services.Book.Content().Revise(workspace.Revision{By: workspace.ByAuthor, Ops: []workspace.Op{{
		Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: w.Category, Text: strings.TrimSpace(w.Text),
		Sources: []workspace.Source{{Chapter: a.selectionChapter(), Quote: w.Quote}},
	}}})
	if err == nil {
		err = a.services.Editing.Refresh()
	}
	if err != nil {
		a.services.Notices.Notices().Warn(err)
		return
	}
	a.services.Editing.Flash(r.Ops[0].Entry)
	w.Open = false
	a.services.Notices.Notices().Did(locales.DialogSettingAdded())
}

// SelectionDraft owns the author form for taking prose into the setting.
type SelectionDraft struct {
	Open                  bool
	Quote, Text, Category string
}

// SelectionOpen reports whether the selection form is open on the frame thread.
func (a *Panel) SelectionOpen() bool             { return a.Take.Open }
func (a *Panel) selectionEditor() *editor.Editor { _, ed, _ := a.services.Editing.Chapter(); return ed }
func (a *Panel) selectionChapter() string        { id, _, _ := a.services.Editing.Chapter(); return id }
