package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
)

// themeChoice picks the appearance of the whole app on this machine:
// following the system, light or dark.
func (a *App) themeChoice(c *ui.Context) {
	a.pal.PopHead(c, "外观")
	// As wide as its three words: the menu grows to hold it, it does not
	// squeeze into the menu.
	ui.Row(c).Padding(2, 6, 6, 6).Children(func() { a.themeSegments(c, false) })
}

// themeSegments are the three appearances side by side.
func (a *App) themeSegments(c *ui.Context, fill bool) {
	t := a.pal.Theme
	bar := ui.Row(c).Gap(2).Padding(2).Radius(kit.RControl).Background(t.SurfacePressed).Shrink(0)
	if fill {
		bar.FillWidth()
	}
	bar.Children(func() {
		for _, m := range []struct {
			v, n string
			s    *ui.SVG
		}{{"", "跟随系统", kit.IcMonitor}, {"light", "浅色", kit.IcSun}, {"dark", "深色", kit.IcMoon}} {
			on := a.shelf.Theme == m.v
			b := ui.ButtonBase(c).Row().Height(kit.HSeg).Center().Gap(6).Padding(0, 10).Radius(kit.RXs).Shrink(0).Label("外观-" + m.n)
			if fill {
				b.Grow(1)
			}
			fg, weight := t.TextMuted, 400
			if on {
				a.pal.Raise.On(b.Background(a.pal.Canvas))
				fg, weight = t.Text, 600
			} else if b.Hovered() {
				fg = t.Text
			}
			b.Children(func() {
				kit.Glyph(c, m.s, kit.IconSm, fg)
				ui.Text(c, m.n).FontSize(kit.FsControl).FontWeight(weight).TextColor(fg).SingleLine()
			})
			if b.Clicked() {
				a.setTheme(m.v)
			}
		}
	})
}

// setTheme keeps the appearance chosen in the shelf, which is this
// machine's, and applies it.
func (a *App) setTheme(v string) {
	if a.shelf.Theme == v {
		return
	}
	a.shelf.Theme = v
	a.saveShelf()
	if a.Appearance != nil {
		a.Appearance(v)
	}
}

// Theme is the appearance chosen on this machine: "light", "dark", or
// empty to follow the system.
func (a *App) Theme() string { return a.shelf.Theme }

// selectionBar floats over the start of the selection with what acts on
// it: the AI's rewrite, while it is on, and taking the words into the
// settings.
func (a *App) selectionBar(c *ui.Context) {
	w := &a.write
	r, ok := w.cur().ed.SelectionTop()
	if !ok || w.cur().ed.ReadOnly() || w.cur().lost || w.cur().external != nil || w.rewriting ||
		w.takeOpen || w.cur().ed.Predicting() || w.fore.noting || w.fore.paying || a.menu.MenuOpen("正文") ||
		w.finding && !w.cur().ed.Focused() { // stepping through what is found picks it, the caret in the find bar
		return
	}
	t := a.pal.Theme
	const height, width = 34, 190
	top := r.Y - height - 6
	if top < 4 { // no room above: under the selection, clear of its other lines
		top = r.Y + r.H + 6
		if box, ok := w.cur().ed.RangeBox(w.cur().ed.Selection()); ok && box.Y+box.H+6+height <= w.cur().ed.Height() {
			top = box.Y + box.H + 6
		}
	}
	left := max(8, min(r.X-12, w.cur().ed.Width()-width-8))
	bar := ui.Row(c).Absolute().Left(left).Top(top).Height(height).Padding(4).Gap(2).AlignItems(ui.Center).Radius(kit.RControl).
		Background(a.pal.Raised).Border(1, t.Border)
	a.pal.Pop.On(bar)
	bar.Children(func() {
		if a.aiOn() && a.selButton(c, kit.IcPen, "AI 改写").Tooltip("由 AI 改写选中内容").Clicked() {
			a.openRewrite()
		}
		if a.selButton(c, kit.IcBookmark, "加入设定").Tooltip("将选中的原文加入设定").Clicked() {
			a.openTake()
		}
	})
}

func (a *App) selButton(c *ui.Context, s *ui.SVG, text string) ui.Element {
	t := a.pal.Theme
	b := ui.ButtonBase(c).Row().Height(kit.HSeg).Padding(0, 8).Gap(6).AlignItems(ui.Center).Radius(kit.RControl).Label(text).KeepFocus()
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		kit.Glyph(c, s, kit.IconSm, t.AccentHover)
		ui.Text(c, text).FontSize(kit.FsNote).TextColor(t.AccentHover).SingleLine()
	})
	return b
}
