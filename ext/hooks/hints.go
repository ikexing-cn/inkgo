package hooks

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/locales"
	"writer/workspace"
)

func (a *Panel) toolHints(c *ui.Context) {
	if a.services.Prose.PaneHidden() && !a.services.Desktop.Focused() {
		here := 0
		for _, f := range a.foreshadows() {
			if f.Here && f.Status() == workspace.ForeOpen {
				here++
			}
		}
		if here > 0 {
			b := ui.ButtonBase(c).Row().Height(kit.HControlSm).Gap(6).Padding(0, 6).AlignItems(ui.Center).Shrink(1).MinWidth(40).Label(locales.TabMarksHere()).Tooltip(locales.TabMarksCount(here)).KeepFocus()
			fg := a.pal().Theme.TextMuted
			if b.Hovered() {
				fg = a.pal().Ink
			}
			b.Children(func() {
				kit.Glyph(c, kit.IcFlag, kit.IconMd, fg)
				ui.Text(c, locales.TabMarksCount(here)).FontSize(kit.FsNote).TextColor(fg).SingleLine().Ellipsis("…").Shrink(1)
			})
			if b.Clicked() {
				a.State.Shown = ""
				a.services.Pane.Show("fore")
			}
		}
	}

}
