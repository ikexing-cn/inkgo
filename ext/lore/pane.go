package lore

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"

	"writer/locales"
)

// The right column's tabs share one shape: a list, a row to a thing, and
// the page of the thing a row was clicked for. These are their parts.

// paneFind is the top of a list: a box that only searches, the button that
// adds beside it when add names one, and what is rarely looked at under
// the dots when more lists it.
func (a *Panel) paneFind(c *ui.Context, query *string, hint, add string, more func(m *kit.Menu)) (added bool) {
	t := a.services.Palette.Colors().Theme
	ui.Row(c).FillWidth().Gap(8).Padding(12, 20).AlignItems(ui.Center).Shrink(0).Children(func() {
		box := ui.Row(c).Grow(1).Shrink(1).MinWidth(0).Height(kit.HControl).Gap(8).Padding(0, 10).AlignItems(ui.Center).Radius(kit.RControl).Background(a.services.Palette.Colors().Canvas).Border(1, t.Border).Label(hint + "框")
		if box.FocusWithin() {
			box.Border(1, a.services.Palette.Colors().Ink)
		}
		box.Children(func() {
			kit.Glyph(c, kit.IcSearch, kit.IconSm, a.services.Palette.Colors().Faint)
			ui.TextInputBase(c, query).Label(hint).Placeholder(hint).Grow(1).Shrink(1).MinWidth(0).FontSize(kit.FsBody)
		})
		if add != "" {
			b := ui.ButtonBase(c).Row().Height(kit.HControl).Gap(6).Padding(0, 12).AlignItems(ui.Center).Radius(kit.RControl).Background(a.services.Palette.Colors().Canvas).Border(1, t.Border).Shrink(0).Label(add)
			if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			if b.IsDisabled() {
				kit.Pale(b)
			}
			kit.PressRow(c, b)
			b.Children(func() {
				kit.Glyph(c, kit.IcPlus, kit.IconSm, t.Text)
				ui.Text(c, add).FontSize(kit.FsControl).SingleLine()
			})
			added = b.Clicked()
		}
		if more != nil {
			b := ui.ButtonBase(c).Size(kit.HControl, kit.HControl).Center().Radius(kit.RControl).Shrink(0).Label(locales.CommonMore()).Tooltip(locales.CommonMore())
			if b.Hovered() || a.services.Native.MenuOpen(hint) {
				b.Background(t.SurfacePressed)
			}
			b.Children(func() { kit.Glyph(c, kit.IcMore, kit.IconMd, t.TextMuted) })
			a.services.Native.DropMenu(c, b, hint, func(panel ui.Element) {
				panel.TransformOrigin(1, 0)
				panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(4, 0, 0, 0).Width(160)
			}, more)
		}
	})
	return added
}
