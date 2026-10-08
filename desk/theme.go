package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// palette is the desk's design system: a neutral paper-white canvas, ink for
// primary actions, one brand hue for what the AI organised, and a quiet tint
// per idea status. Everything else is typography and spacing.
type palette struct {
	theme                          *ui.Theme
	canvas, side, raised, bubble   ui.Color
	line, faint, brand, ink, onInk ui.Color
}

func lightPalette() *palette {
	t := ui.LightTheme()
	t.Background = ui.Hex("#ffffff")
	t.Surface = ui.Hex("#ffffff")
	t.SurfaceHover = ui.Hex("#f3f3f1")
	t.SurfacePressed = ui.Hex("#eaeae7")
	t.Border = ui.Hex("#e4e4e0")
	t.Text = ui.Hex("#1d1d1b")
	t.TextMuted = ui.Hex("#76766f")
	t.Accent = ui.Hex("#1d1d1b")
	t.AccentHover = ui.Hex("#3a3a37")
	t.AccentPressed = ui.Hex("#000000")
	t.AccentText = ui.Hex("#ffffff")
	t.Success = ui.Hex("#2f8a5b")
	t.Warning = ui.Hex("#b7791f")
	t.Danger = ui.Hex("#c4473a")
	t.Selection = ui.RGBA(94, 84, 214, 0.18)
	t.Focus = ui.RGBA(94, 84, 214, 0.45)
	t.Scrollbar = ui.RGBA(0, 0, 0, 0.18)
	t.Radius = 8
	return &palette{theme: t,
		canvas: ui.Hex("#ffffff"), side: ui.Hex("#f7f7f5"), raised: ui.Hex("#ffffff"), bubble: ui.Hex("#f1f1ee"),
		line: ui.Hex("#ececE8"), faint: ui.Hex("#a3a39c"), brand: ui.Hex("#5e54d6"), ink: ui.Hex("#1d1d1b"), onInk: ui.Hex("#ffffff")}
}

func darkPalette() *palette {
	t := ui.DarkTheme()
	t.Background = ui.Hex("#191919")
	t.Surface = ui.Hex("#222221")
	t.SurfaceHover = ui.Hex("#2b2b2a")
	t.SurfacePressed = ui.Hex("#333331")
	t.Border = ui.Hex("#343432")
	t.Text = ui.Hex("#ededea")
	t.TextMuted = ui.Hex("#9a9a93")
	t.Accent = ui.Hex("#ededea")
	t.AccentHover = ui.Hex("#ffffff")
	t.AccentPressed = ui.Hex("#d6d6d2")
	t.AccentText = ui.Hex("#191919")
	t.Success = ui.Hex("#5cb684")
	t.Warning = ui.Hex("#d6a24a")
	t.Danger = ui.Hex("#e4675b")
	t.Selection = ui.RGBA(139, 131, 240, 0.32)
	t.Focus = ui.RGBA(139, 131, 240, 0.6)
	t.Scrollbar = ui.RGBA(255, 255, 255, 0.2)
	t.Radius = 8
	return &palette{theme: t,
		canvas: ui.Hex("#191919"), side: ui.Hex("#1f1f1e"), raised: ui.Hex("#222221"), bubble: ui.Hex("#2a2a29"),
		line: ui.Hex("#2c2c2b"), faint: ui.Hex("#6f6f69"), brand: ui.Hex("#8b83f0"), ink: ui.Hex("#ededea"), onInk: ui.Hex("#191919")}
}

func (p *palette) statusColor(s workspace.Status) ui.Color {
	t := p.theme
	switch s {
	case workspace.Kept:
		return t.Success
	case workspace.Deferred:
		return t.Warning
	case workspace.Rejected, workspace.Superseded:
		return p.faint
	case workspace.Adopted:
		return ui.Hex("#3b82c4")
	}
	return p.brand
}

// dot is a small status disc.
func dot(c *ui.Context, color ui.Color, size float32) *ui.Element {
	return ui.Box(c).Size(size, size).Radius(999).Background(color).Shrink(0)
}

// pill is a small rounded label.
func pill(c *ui.Context, text string, color ui.Color) *ui.Element {
	if color.A == 0 {
		color = c.Theme().TextMuted
	}
	b := ui.Row(c).Padding(1, 7).Radius(5).Background(color.Alpha(0.1)).Shrink(0)
	b.Children(func() { ui.Text(c, text).FontSize(11.5).TextColor(color).SingleLine() })
	return b
}

// ghost is a borderless text button: muted until hovered.
func ghost(c *ui.Context, text string, color ui.Color) *ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Row().AlignItems(ui.Center).Gap(5).Padding(4, 8).Radius(6).Shrink(0)
	fg := color
	if fg.A == 0 {
		fg = t.TextMuted
	}
	if b.Hovered() {
		b.Background(t.SurfaceHover)
		if color.A == 0 {
			fg = t.Text
		}
	}
	if b.IsDisabled() {
		b.Opacity(0.4)
	}
	b.Children(func() { ui.Text(c, text).FontSize(12.5).TextColor(fg).SingleLine() })
	return b
}

// icon parses a 24×24 stroked icon drawn in the text color.
func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	icChat    = icon(`<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>`)
	icTag     = icon(`<path d="M20.6 13.4 13.4 20.6a2 2 0 0 1-2.8 0L3 13V3h10l7.6 7.6a2 2 0 0 1 0 2.8z"/><circle cx="7.5" cy="7.5" r="1.2"/>`)
	icGear    = icon(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/>`)
	icPlus    = icon(`<path d="M12 5v14M5 12h14"/>`)
	icSend    = icon(`<path d="M12 19V5M5 12l7-7 7 7"/>`)
	icSparkle = icon(`<path d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8z"/><path d="M19 17l.7 1.8 1.8.7-1.8.7L19 22l-.7-1.8-1.8-.7 1.8-.7z"/>`)
	icQuote   = icon(`<path d="M4 6h16M4 12h10M4 18h7"/>`)
	icEye     = icon(`<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>`)
	icRetry   = icon(`<path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/>`)
	icCheck   = icon(`<path d="M20 6 9 17l-5-5"/>`)
	icX       = icon(`<path d="M18 6 6 18M6 6l12 12"/>`)
	icClock   = icon(`<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`)
	icSearch  = icon(`<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>`)
	icAlert   = icon(`<path d="M12 9v4M12 17h.01"/><path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/>`)
	icChevron = icon(`<path d="m9 18 6-6-6-6"/>`)
)

// glyph shows an icon at size in color (zero: inherit the text color).
func glyph(c *ui.Context, s *ui.SVG, size float32, color ui.Color) *ui.Element {
	e := ui.Icon(c, s).Size(size, size).Shrink(0)
	if color.A != 0 {
		e.TextColor(color)
	}
	return e
}

// iconButton is a square borderless button with a tooltip; label also names
// it for tests and assistive technology.
func iconButton(c *ui.Context, s *ui.SVG, label string, color ui.Color) *ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Size(28, 28).Center().Radius(6).Shrink(0).Label(label).Tooltip(label)
	fg := color
	if fg.A == 0 {
		fg = t.TextMuted
	}
	if b.Hovered() {
		b.Background(t.SurfaceHover)
		if color.A == 0 {
			fg = t.Text
		}
	}
	if b.IsDisabled() {
		b.Opacity(0.35)
	}
	b.Children(func() { glyph(c, s, 16, fg) })
	return b
}

// primary is the one solid ink button of a view.
func (p *palette) primary(c *ui.Context, text string) *ui.Element {
	b := ui.ButtonBase(c).Row().Center().Gap(6).Padding(7, 16).Radius(8).Shrink(0)
	bg := p.ink
	if b.Hovered() {
		bg = p.theme.AccentHover
	}
	b.Background(bg)
	if b.IsDisabled() {
		b.Opacity(0.3)
	}
	b.Children(func() { ui.Text(c, text).FontSize(13).FontWeight(600).TextColor(p.onInk).SingleLine() })
	return b
}

// secondary is an outlined button.
func (p *palette) secondary(c *ui.Context, text string) *ui.Element {
	t := p.theme
	b := ui.ButtonBase(c).Row().Center().Gap(6).Padding(6, 14).Radius(8).Border(1, t.Border).Background(p.raised).Shrink(0)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	if b.IsDisabled() {
		b.Opacity(0.4)
	}
	b.Children(func() { ui.Text(c, text).FontSize(13).SingleLine() })
	return b
}

// group is a rounded, hairline-bordered container whose rows are separated
// by hairlines, as in system settings.
func (p *palette) group(c *ui.Context) *ui.Element {
	return ui.Column(c).FillWidth().Radius(10).Border(1, p.theme.Border).Background(p.raised).Clip()
}

func (p *palette) hairline(c *ui.Context) {
	ui.Box(c).FillWidth().Height(1).Background(p.line).Shrink(0)
}

// cardBox is a padded group.
func (p *palette) cardBox(c *ui.Context) *ui.Element {
	return p.group(c).Padding(18, 20).Gap(12)
}

// action is a tinted button; strong fills it, for destructive confirmations.
func action(c *ui.Context, text string, color ui.Color, strong bool) *ui.Element {
	b := ui.ButtonBase(c).Row().Center().Padding(5, 12).Radius(7).Shrink(0)
	bg, fg := color.Alpha(0.1), color
	if strong {
		bg, fg = color, ui.Hex("#ffffff")
	}
	if b.Hovered() {
		bg = bg.Mix(color, 0.2)
	}
	b.Background(bg)
	if b.IsDisabled() {
		b.Opacity(0.4)
	}
	b.Children(func() { ui.Text(c, text).FontSize(12.5).FontWeight(600).TextColor(fg).SingleLine() })
	return b
}
