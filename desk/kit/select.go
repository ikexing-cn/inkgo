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
	return p.selectNamed(c, selected, options, name, 0)
}

// SelectNamedSize draws a named select with an explicit label font size.
func (p *Palette) SelectNamedSize(c *ui.Context, selected *string, options []string, name func(string) string, size float32) ui.Element {
	return p.selectNamed(c, selected, options, name, size)
}

func (p *Palette) selectNamed(c *ui.Context, selected *string, options []string, name func(string) string, size float32) ui.Element {
	t := c.Theme()
	sel := ui.SelectBase(c, selected)
	b := sel.Trigger.Padding(t.Space(1.5), t.Space(3.5)).Gap(t.Space(1.5)).Radius(t.Radius).
		Background(t.Surface).TextColor(t.Text).Border(1, t.Border).Justify(ui.SpaceBetween).MinWidth(t.Space(35))
	if size > 0 {
		b.Height(HControl).Padding(0, 10).Gap(8)
	}
	switch {
	case b.Pressed():
		b.Background(t.SurfacePressed)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		label := ui.Text(c, name(*selected)).SingleLine()
		if size > 0 {
			label.FontSize(size).Shrink(0)
		}
		arrow := t.Space(2.5)
		if size > 0 {
			arrow = IconSm
		}
		Glyph(c, IcChevronDown, arrow, t.TextMuted)
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
				item.Children(func() {
					label := ui.Text(c, name(opt)).SingleLine()
					if size > 0 {
						label.FontSize(size)
					}
				})
			}
		})
	})
	return b
}
