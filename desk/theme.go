package desk

import (
	_ "embed"
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// palette is the desk's design system: a neutral paper-white canvas, ink for
// primary actions and links, greys for everything quieter, and amber for what
// waits on the author. Everything else is typography and spacing.
//
// Its colors come from tokens.css, a copy of the design's base colors: the
// design and the code name each color the same way, so one changed there is
// copied here and nothing else moves.
type palette struct {
	theme                        *ui.Theme
	canvas, side, raised, bubble ui.Color
	line, faint, ink, onInk      ui.Color
	amber                        ui.Color // what waits on the author
	warnBg, warnBd               ui.Color // a warning's fill and border
	// The shadows of the design rules, named as there: the chosen part of
	// a segmented control; menus, popovers and toasts; dialogs.
	raise, pop, dialog shadow
}

// The sizes of the design rules (设计规范 in the design): eight text sizes and
// four corners, named as there. A size between them is not used. They are
// the --fs-* and --r-* of tokens.css, which a test holds them to; the desk
// writes these names, never a number.
const (
	fsDisplay = 26   // the welcome page's big words
	fsTitle   = 22   // a page's title, the settings' too
	fsDialog  = 17   // a dialog's title
	fsGroup   = 15   // a group's title in a page
	fsBody    = 13.5 // list rows, form rows
	fsControl = 13   // what buttons, inputs and selects say
	fsNote    = 12.5 // what explains, under or beside
	fsSmall   = 11.5 // labels, counts, keys
	// Prose to read, as the discussion's and a snapshot's, is apart from
	// them, as the manuscript's own size is.
	fsRead = 14.5

	rXs      = 4  // checkboxes, labels
	rControl = 6  // buttons, inputs, selects, menu rows
	rCard    = 10 // groups, cards, menus, toasts
	rDialog  = 12 // dialogs, and the discussion's bubbles and box
	rRound   = 999
)

//go:embed tokens.css
var tokensCSS string

// blocks calls fn with what each block of tokens.css declares, comments
// dropped, and whether it is the dark one.
func blocks(css string, fn func(dark bool, body string)) {
	css = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	for _, b := range regexp.MustCompile(`(?s)([^{}]+)\{([^}]*)\}`).FindAllStringSubmatch(css, -1) {
		fn(strings.TrimSpace(b[1]) == ".dark", b[2])
	}
}

// tokens reads the colors of tokens.css: the light ones, and the dark ones
// over them.
func tokens(css string) (light, dark map[string]ui.Color) {
	light, dark = map[string]ui.Color{}, map[string]ui.Color{}
	decl := regexp.MustCompile(`--([a-z0-9-]+)\s*:\s*(#[0-9a-fA-F]{3,6})`)
	blocks(css, func(isDark bool, body string) {
		into := light
		if isDark {
			into = dark
		}
		for _, d := range decl.FindAllStringSubmatch(body, -1) {
			into[d[1]] = ui.Hex(d[2])
		}
	})
	for k, v := range light {
		if _, ok := dark[k]; !ok {
			dark[k] = v
		}
	}
	return light, dark
}

// shadow is a level of the design's shadows. An element draws one shadow,
// so it is the level's first layer.
type shadow struct {
	x, y, blur, spread float32
	color              ui.Color
}

// on gives e the shadow.
func (s shadow) on(e *ui.Element) *ui.Element {
	return e.Shadow(s.x, s.y, s.blur, s.spread, s.color)
}

var shadowsLight, shadowsDark = shadows(tokensCSS)

// shadowOf is the named shadow of the theme c draws with, for what draws
// without the App's palette.
func shadowOf(c *ui.Context, name string) shadow {
	if c.Theme().Dark {
		return shadowsDark[name]
	}
	return shadowsLight[name]
}

// shadows reads the --shadow-* of tokens.css as tokens does the colors. An
// inset one is left out: the desk draws none.
func shadows(css string) (light, dark map[string]shadow) {
	light, dark = map[string]shadow{}, map[string]shadow{}
	decl := regexp.MustCompile(`--shadow-([a-z]+)\s*:\s*(-?[0-9.]+)(?:px)?\s+(-?[0-9.]+)(?:px)?\s+([0-9.]+)(?:px)?\s+(?:([0-9.]+)px\s+)?rgba\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*([0-9.]+)\s*\)`)
	f := func(s string) float32 {
		v, _ := strconv.ParseFloat(s, 32)
		return float32(v)
	}
	blocks(css, func(isDark bool, body string) {
		into := light
		if isDark {
			into = dark
		}
		for _, d := range decl.FindAllStringSubmatch(body, -1) {
			into[d[1]] = shadow{f(d[2]), f(d[3]), f(d[4]), f(d[5]),
				ui.RGBA(uint8(f(d[6])), uint8(f(d[7])), uint8(f(d[8])), f(d[9]))}
		}
	})
	for k, v := range light {
		if _, ok := dark[k]; !ok {
			dark[k] = v
		}
	}
	return light, dark
}

func lightPalette() *palette {
	k, _ := tokens(tokensCSS)
	s, _ := shadows(tokensCSS)
	t := ui.LightTheme()
	themeOf(t, k)
	t.Selection = ui.RGBA(50, 120, 215, 0.2) // --sel over the page, so the text shows through
	t.Focus = k["ink"].Alpha(0.35)
	t.Scrollbar = ui.RGBA(0, 0, 0, 0.18)
	return paletteOf(t, k, s)
}

func darkPalette() *palette {
	_, k := tokens(tokensCSS)
	_, s := shadows(tokensCSS)
	t := ui.DarkTheme()
	themeOf(t, k)
	t.Selection = ui.RGBA(90, 140, 220, 0.3)
	t.Focus = k["ink"].Alpha(0.45)
	t.Scrollbar = ui.RGBA(255, 255, 255, 0.2)
	return paletteOf(t, k, s)
}

func themeOf(t *ui.Theme, k map[string]ui.Color) {
	t.Background = k["bg"]
	t.Surface = k["raised"]
	t.SurfaceHover = k["hover"]
	t.SurfacePressed = k["active"]
	t.Border = k["border"]
	t.Text = k["ink"]
	t.TextMuted = k["muted"]
	t.Accent = k["ink"]
	t.AccentHover = k["ink-2"]
	t.AccentPressed = k["ink"]
	t.AccentText = k["on-ink"]
	t.Success = k["success"]
	t.Warning = k["warn"]
	t.Danger = k["danger"]
	t.Radius = rControl
}

func paletteOf(t *ui.Theme, k map[string]ui.Color, s map[string]shadow) *palette {
	return &palette{theme: t,
		canvas: k["bg"], side: k["side"], raised: k["raised"], bubble: k["hover"],
		line: k["line"], faint: k["faint"], ink: k["ink"], onInk: k["on-ink"],
		amber: k["amber"], warnBg: k["warn-bg"], warnBd: k["warn-bd"],
		raise: s["raise"], pop: s["pop"], dialog: s["dialog"]}
}

// dot is a small status disc.
func dot(c *ui.Context, color ui.Color, size float32) *ui.Element {
	return ui.Box(c).Size(size, size).Radius(rRound).Background(color).Shrink(0)
}

// pill is a small rounded label.
func pill(c *ui.Context, text string, color ui.Color) *ui.Element {
	if color.A == 0 {
		color = c.Theme().TextMuted
	}
	b := ui.Row(c).Padding(2, 8).Radius(rXs).Background(color.Alpha(0.1)).Shrink(0)
	b.Children(func() { ui.Text(c, text).FontSize(fsSmall).TextColor(color).SingleLine() })
	return b
}

// ghost is a borderless text button: muted until hovered.
func ghost(c *ui.Context, text string, color ui.Color) *ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Row().AlignItems(ui.Center).Gap(6).Padding(4, 8).Radius(rControl).Shrink(0)
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
	b.Children(func() { ui.Text(c, text).FontSize(fsNote).TextColor(fg).SingleLine() })
	return b
}

// icon parses a 24×24 stroked icon drawn in the text color.
func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	icChat        = icon(`<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>`)
	icTag         = icon(`<path d="M20.6 13.4 13.4 20.6a2 2 0 0 1-2.8 0L3 13V3h10l7.6 7.6a2 2 0 0 1 0 2.8z"/><circle cx="7.5" cy="7.5" r="1.2"/>`)
	icGear        = icon(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/>`)
	icPlus        = icon(`<path d="M12 5v14M5 12h14"/>`)
	icSend        = icon(`<path d="M12 19V5M5 12l7-7 7 7"/>`)
	icSparkle     = icon(`<path d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8z"/><path d="M19 17l.7 1.8 1.8.7-1.8.7L19 22l-.7-1.8-1.8-.7 1.8-.7z"/>`)
	icQuote       = icon(`<path d="M4 6h16M4 12h10M4 18h7"/>`)
	icCopy        = icon(`<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/>`)
	icPanel       = icon(`<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M15 4v16"/>`)
	icPencil      = icon(`<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z"/>`)
	icUndo        = icon(`<path d="M9 14 4 9l5-5"/><path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11"/>`)
	icEye         = icon(`<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>`)
	icRetry       = icon(`<path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/>`)
	icDone        = icon(`<circle cx="12" cy="12" r="9"/><path d="m8 12.5 2.8 2.8 5.7-5.8"/>`)
	icCheck       = icon(`<path d="M20 6 9 17l-5-5"/>`)
	icX           = icon(`<path d="M18 6 6 18M6 6l12 12"/>`)
	icClock       = icon(`<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`)
	icSearch      = icon(`<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>`)
	icAlert       = icon(`<path d="M12 9v4M12 17h.01"/><path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/>`)
	icChevron     = icon(`<path d="m9 18 6-6-6-6"/>`)
	icBack        = icon(`<path d="m15 6-6 6 6 6"/>`)
	icForward     = icon(`<path d="m9 6 6 6-6 6"/>`)
	icChevronUp   = icon(`<path d="m18 15-6-6-6 6"/>`)
	icChip        = icon(`<path d="M9 3v3M15 3v3M9 18v3M15 18v3M3 9h3M3 15h3M18 9h3M18 15h3M7 6h10a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1zM10 10h4v4h-4z"/>`)
	icChevronDown = icon(`<path d="m6 9 6 6 6-6"/>`)
	icBook        = icon(`<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/>`)
	icFolder      = icon(`<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>`)
	icFolderPlus  = icon(`<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><path d="M12 10.5v5M9.5 13h5"/>`)
	icDoc         = icon(`<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/>`)
	icTrash       = icon(`<path d="M3 6h18"/><path d="M8 6V4h8v2"/><path d="M19 6l-1 14H6L5 6"/>`)
	icPen         = icon(`<path d="M12 20h8"/><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4z"/>`)
	icDraft       = icon(`<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5M9 13h6M9 17h4"/>`)
	icBookmark    = icon(`<path d="M6 3h12v18l-6-4-6 4z"/><path d="M12 7v6M9 10h6"/>`)
	icHistory     = icon(`<path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5M12 7v5l3 2"/>`)
	icMonitor     = icon(`<path d="M4 5h16v11H4zM9 20h6M12 16v4"/>`)
	icSun         = icon(`<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>`)
	icMoon        = icon(`<path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5z"/>`)
	icWand        = icon(`<path d="m14 4 6 6L9 21l-6-6z"/><path d="M11 7l6 6"/>`)
	icFeather     = icon(`<path d="M20 4C11 4 5 10 5 19v1"/><path d="M5 15h8"/><path d="M20 4c0 7-4 11-11 11"/>`)
	icList        = icon(`<path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/>`)
	icGrid        = icon(`<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>`)
	icMore        = icon(`<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>`)
	icExpand      = icon(`<path d="M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7"/>`)
	icShrink      = icon(`<path d="M4 14h6v6M20 10h-6V4M14 10l7-7M3 21l7-7"/>`)
	icDownload    = icon(`<path d="M12 4v11M7 10l5 5 5-5"/><path d="M4 20h16"/>`)
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
	b := ui.ButtonBase(c).Size(28, 28).Center().Radius(rControl).Shrink(0).Label(label).Tooltip(label)
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
	b := ui.ButtonBase(c).Row().Center().Gap(6).Padding(8, 16).Radius(rControl).Shrink(0)
	bg := p.ink
	if b.Hovered() {
		bg = p.theme.AccentHover
	}
	b.Background(bg)
	if b.IsDisabled() {
		b.Opacity(0.3)
	}
	b.Children(func() { ui.Text(c, text).FontSize(fsControl).FontWeight(600).TextColor(p.onInk).SingleLine() })
	return b
}

// secondary is an outlined button.
func (p *palette) secondary(c *ui.Context, text string) *ui.Element {
	t := p.theme
	b := ui.ButtonBase(c).Row().Center().Gap(6).Padding(6, 14).Radius(rControl).Border(1, t.Border).Background(p.raised).Shrink(0)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	if b.IsDisabled() {
		b.Opacity(0.4)
	}
	b.Children(func() { ui.Text(c, text).FontSize(fsControl).SingleLine() })
	return b
}

// small is the compact button of a popover or a row, solid ink when it is
// the likely one, outlined otherwise.
func (p *palette) small(c *ui.Context, text string, primary bool) *ui.Element {
	t := p.theme
	b := ui.ButtonBase(c).Row().Padding(4, 10).Radius(rControl).Shrink(0)
	fg := t.Text
	if primary {
		bg := p.ink
		if b.Hovered() {
			bg = t.AccentHover
		}
		b.Background(bg)
		fg = p.onInk
	} else {
		b.Border(1, t.Border)
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
	}
	if b.IsDisabled() {
		b.Opacity(0.4)
	}
	b.Children(func() { ui.Text(c, text).FontSize(fsNote).TextColor(fg).SingleLine() })
	return b
}

// group is a rounded, hairline-bordered container whose rows are separated
// by hairlines, as in system settings.
func (p *palette) group(c *ui.Context) *ui.Element {
	return ui.Column(c).FillWidth().Radius(rCard).Border(1, p.theme.Border).Background(p.raised).Clip()
}

func (p *palette) hairline(c *ui.Context) {
	ui.Box(c).FillWidth().Height(1).Background(p.line).Shrink(0)
}

// cardBox is a padded group.
func (p *palette) cardBox(c *ui.Context) *ui.Element {
	return p.group(c).Padding(20, 20).Gap(12)
}

// action is a tinted button; strong fills it, for destructive confirmations.
func action(c *ui.Context, text string, color ui.Color, strong bool) *ui.Element {
	b := ui.ButtonBase(c).Row().Center().Padding(6, 12).Radius(rControl).Shrink(0)
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
	b.Children(func() { ui.Text(c, text).FontSize(fsNote).FontWeight(600).TextColor(fg).SingleLine() })
	return b
}
