package kit

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// popPanel is the panel of a menu.
func (p *Palette) PopPanel(c *ui.Context) ui.Element {
	return p.PopLook(ui.Column(c))
}

// popLook gives panel the look of a menu.
func (p *Palette) PopLook(panel ui.Element) ui.Element {
	panel.Padding(4).Radius(RCard).Background(p.Raised).Border(1, p.Theme.Border).Role(ui.RoleMenu).FocusGroup(ui.Vertical)
	p.Pop.On(panel)
	return panel
}

// popRow is a row of a menu: a picture if any, what it does, and at its
// right its shortcut, a count, or why it cannot be done now. on is the
// row in use, or the one whose list is open.
func (p *Palette) PopRow(c *ui.Context, pic *ui.SVG, label, hint string, off, on bool) ui.Element {
	t := p.Theme
	b := ui.ButtonBase(c).Row().FillWidth().Height(HRow).Padding(0, 10).Gap(8).AlignItems(ui.Center).Radius(RControl).Shrink(0).
		Role(ui.RoleMenuItem).Label("菜单-" + strings.TrimSuffix(label, "…")).Disabled(off)
	switch {
	case off:
		Pale(b)
	case on || b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		if pic != nil && pic != IcChevron {
			Glyph(c, pic, IconMd, t.TextMuted)
		}
		// Its own ink: a disabled button would pale its words a second time.
		ui.Text(c, label).FontSize(FsControl).TextColor(t.Text).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
		if hint != "" {
			ui.Text(c, hint).FontSize(FsNote).TextColor(t.TextMuted).SingleLine().Shrink(0)
		}
		if pic == IcChevron {
			Glyph(c, pic, IconSm, p.Faint)
		}
	})
	return b
}

// popHead names the rows under it.
func (p *Palette) PopHead(c *ui.Context, label string) {
	ui.Text(c, label).FontSize(FsSmall).TextColor(p.Faint).Padding(6, 10, 2, 10).Shrink(0)
}

// popLine parts groups of rows.
func (p *Palette) PopLine(c *ui.Context) {
	ui.Box(c).FillWidth().Height(1).Margin(4, 0).Background(p.Line).Shrink(0)
}
