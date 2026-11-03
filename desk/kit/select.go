package kit

import "github.com/egoist/mygo/ui"

// Select is ui.Select whose options pop in as the desk's other menus do,
// with their shadow. ui.Select opens at once and keeps its parts to
// itself, so its look is built again here on ui.SelectBase.
func (p *Palette) Select(c *ui.Context, selected *string, options []string) ui.Element {
	return p.SelectNamed(c, selected, options, func(o string) string { return o })
}

// SelectNamed is Select whose options are kept as they are and shown by
// name: an ID saved or compared stays the same in every language.
func (p *Palette) SelectNamed(c *ui.Context, selected *string, options []string, name func(string) string) ui.Element {
	t := c.Theme()
	sel := ui.SelectBase(c, selected)
	b := sel.Trigger.Padding(t.Space(1.5), t.Space(3.5)).Gap(t.Space(1.5)).Radius(t.Radius).
		Background(t.Surface).TextColor(t.Text).Border(1, t.Border).Justify(ui.SpaceBetween).MinWidth(t.Space(35))
	switch {
	case b.Pressed():
		b.Background(t.SurfacePressed)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Text(c, name(*selected)).SingleLine()
		Glyph(c, IcChevronDown, t.Space(2.5), t.TextMuted)
	})
	sel.Popup(func(panel ui.Element) {
		panel.Margin(t.Space(1), 0, 0, 0).Padding(t.Space(1)).Radius(t.Radius+2).Background(t.Background).Border(1, t.Border)
		p.Pop.On(panel)
		PopIn(panel, 0, 0)
		// Options past the room under the trigger scroll.
		_, h := c.Size()
		ui.Scroll(c).MaxHeight(h - b.Bounds().Y - b.Bounds().H - t.Space(4)).Children(func() {
			for _, opt := range options {
				item := sel.Item(opt).Padding(t.Space(1.5), t.Space(2.5)).Radius(t.Radius)
				switch {
				case item.Highlighted():
					item.Background(t.Accent).TextColor(t.AccentText)
				case opt == *selected:
					item.Background(t.Surface)
				}
				item.Children(func() { ui.Text(c, name(opt)).SingleLine() })
			}
		})
	})
	return b
}
