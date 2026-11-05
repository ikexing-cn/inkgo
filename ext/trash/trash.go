package trash

import (
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/trash/trashtext"
	"writer/locales"
)

func (a *View) openTrash() {
	w := a
	a.Editing.Flush()
	items, err := a.Book.Store().Trash()
	a.Notices.Notices().Warn(err)
	w.Trash, w.TrashOpen, w.Purge = items, true, ""
}

func (a *View) trashView(c *ui.Context) {
	t := a.Palette.Colors().Theme
	w := a
	a.Palette.Colors().Modal(c, &w.TrashOpen, func() {
		ui.Column(c).Width(kit.DialogSmall).Padding(kit.DialogPad).Gap(16).Children(func() {
			a.Palette.Colors().DialogHead(c, trashtext.TrashTitle(), trashtext.TrashAbout())
			ui.Scroll(c.Key("trash")).FillWidth().MaxHeight(320).Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					a.Palette.Colors().Hairline(c)
					if len(w.Trash) == 0 {
						ui.Text(c, trashtext.TrashEmpty()).FontSize(kit.FsNote).TextColor(t.TextMuted).Padding(20, 0)
					}
					for _, it := range w.Trash {
						ui.Row(c.Key("t-"+it.ID)).FillWidth().AlignItems(ui.Center).Gap(10).Padding(10, 0).Children(func() {
							ui.Column(c).Grow(1).Shrink(1).Children(func() {
								ui.Text(c, it.Title).FontSize(kit.FsBody).LineHeight(kit.LhBody).SingleLine().Ellipsis("…")
								ui.Text(c, trashtext.TrashDeletedAt(kit.When(it.At, time.Now()))).FontSize(kit.FsNote).LineHeight(kit.LhBody).TextColor(t.TextMuted)
							})
							// Deleting for good is asked twice, in the row itself.
							if w.Purge == it.ID {
								if a.Palette.Colors().Secondary(c, locales.CommonCancel()).Label("取消彻底删除").Clicked() {
									w.Purge = ""
								}
								if a.Palette.Colors().Danger(c, trashtext.TrashConfirm()).Clicked() {
									a.Notices.Notices().Warn(a.Editing.PurgeTrash(it.ID))
									a.openTrash()
								}
								return
							}
							if a.Palette.Colors().Ghost(c, trashtext.TrashDelete(), t.Danger).Clicked() {
								w.Purge = it.ID
							}
							if a.Palette.Colors().Secondary(c, trashtext.TrashRestore()).Label("恢复-" + it.Title).Clicked() {
								a.restore(it.ID, it.Title)
							}
						})
						a.Palette.Colors().Hairline(c)
					}
				})
			})
			ui.Row(c).FillWidth().Children(func() {
				ui.Spacer(c)
				if a.Palette.Colors().Secondary(c, locales.CommonClose()).Clicked() {
					w.TrashOpen = false
				}
			})
		})
	})
}

func (a *View) restore(id, title string) {
	if err := a.Editing.RestoreTrash(id); err != nil {
		a.Notices.Notices().Warn(err)
		return
	}
	a.openTrash()
	a.Notices.Notices().Did(trashtext.TrashRestored(title))
}
