package kit

import (
	_ "embed"
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Sizes of the text: the default, and the least and most it goes to.
const (
	TextSizeDefault = 17
	TextSizeMin     = 13
	TextSizeMax     = 28
)

// subhead titles a block of the settings.
func (p *Palette) Subhead(c *ui.Context, title, caption string) {
	ui.Column(c).FillWidth().Gap(2).Padding(12, 0, 0, 0).Children(func() {
		ui.Text(c, title).FontSize(FsGroup).FontWeight(600)
		if caption != "" {
			ui.Text(c, caption).FontSize(FsNote).TextColor(p.Theme.TextMuted).LineHeight(LhBody)
		}
	})
}

// tickNote explains the tick box over it, in line with the box's words.
func (p *Palette) TickNote(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(FsNote).TextColor(p.Theme.TextMuted).LineHeight(LhBody).Margin(-4, 0, 4, tickIndent)
}

// titleHeight is the height of the title bar, and of the top row of each
// pane under it. The window buttons fill the title bar (cmd/inkgo gives the
// window the same title bar height), so their hover covers the whole row.
const TitleHeight = HBar

// palette is the desk's colors and shadows, light or dark. They come from
// tokens.css, which the prototype in prototype/ styles itself from too: the
// two name each color the same way, so one changed there changes in both and
// nothing else moves. What each is for is written in DESIGN.md alone.
type Palette struct {
	Theme                        *ui.Theme
	Canvas, Side, Raised, Bubble ui.Color
	Line, Faint, Ink, OnInk      ui.Color
	Control, Dim, Amber          ui.Color
	WarnBg, WarnBd               ui.Color
	PhoneEye                     ui.Color
	Raise, Pop, Dialog           shadow
	Scrim                        ui.Color
}

// The sizes of DESIGN.md under its names: the --fs-*, --r-*, --lh-*,
// --icon-*, --h-* and --opacity-* of tokens.css, which a test holds them to.
// What each is for is written in DESIGN.md alone. The desk writes these
// names, never a number; an alpha is a number on the --alpha scale there.
const (
	FsDisplay = 26
	FsTitle   = 22
	FsDialog  = 17
	FsGroup   = 15
	FsBody    = 13.5
	FsControl = 13
	FsNote    = 12.5
	FsSmall   = 11.5
	FsRead    = 14.5

	RXs      = 4
	RControl = 6
	RCard    = 10
	RDialog  = 12
	RRound   = 999

	LhTight = 1.4
	LhBody  = 1.6
	LhLoose = 1.7
	LhRead  = 1.8
	LhPaper = 1.95

	IconXs = 12
	IconSm = 14
	IconMd = 16
	IconLg = 24

	OpacityDisabled = 0.4

	HControl   = 32
	HControlSm = 28
	HRow       = 30
	HSeg       = 26
	HTag       = 20
	HBar       = 40
)

// The dialogs' sizes: most are small; the shortcuts, taking a setting from
// a passage and the snapshots have their own.
const (
	DialogAsk    = 360 // one question and its answers
	DialogSmall  = 480
	DialogKeys   = 600
	DialogSource = 620
	DialogWide   = 860
	DialogTall   = 580
	DialogPad    = 24
)

// modal is a dialog as the design draws one: the window shaded behind it,
// its corners and its shadow. What fn makes first gives it its size, with
// dialogPad around what it holds.
func (p *Palette) Modal(c *ui.Context, open *bool, fn func()) {
	if !*open {
		return
	}
	w, h := c.Size()
	ui.DialogBase(c, open, func(back, panel ui.Element) {
		back.Background(p.Scrim)
		panel.MaxWidth(w - 2*DialogPad).MaxHeight(h - 2*DialogPad).Radius(RDialog).Background(p.Raised)
		p.Dialog.On(panel)
		fn()
	})
}

// blank is what a page shows when it has nothing: a faint picture, what is
// missing, a line or two on what to do about it, and more, if any, under
// them. It stands in the middle of the room it is given.
func (p *Palette) Blank(c *ui.Context, pic *ui.SVG, title, about string, more func()) ui.Element {
	return ui.Column(c).Grow(1).FillWidth().Center().Gap(10).Children(func() {
		Glyph(c, pic, IconLg, p.Faint)
		ui.Text(c, title).FontSize(FsDialog).FontWeight(700).TextAlign(ui.Center)
		if about != "" {
			ui.Text(c, about).FontSize(FsBody).TextColor(p.Theme.TextMuted).LineHeight(LhLoose).TextAlign(ui.Center).MaxWidth(440)
		}
		if more != nil {
			more()
		}
	})
}

// disabledByKit is what the toolkit multiplies the opacity of a disabled
// element by on its own.
const disabledByKit = 0.5

// pale gives b, a disabled control, the design's opacity for one.
func Pale(b ui.Element) ui.Element { return b.Opacity(OpacityDisabled / disabledByKit) }

// dialogHead is what a dialog opens with: what it is, and under it, in a
// line or two, what it does.
func (p *Palette) DialogHead(c *ui.Context, title, about string) {
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, title).FontSize(FsDialog).FontWeight(700)
		if about != "" {
			ui.Text(c, about).FontSize(FsNote).TextColor(p.Theme.TextMuted).LineHeight(LhBody).FillWidth()
		}
	})
}

// tickIndent is from a tick box's left edge to its words: what explains
// the box starts under the words.
const tickIndent = IconMd + 8

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

// colorTokens reads the colors of tokens.css: the light ones, and the dark ones
// over them.
func colorTokens(css string) (light, dark map[string]ui.Color) {
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
func (s shadow) On(e ui.Element) ui.Element {
	return e.Shadow(s.x, s.y, s.blur, s.spread, s.color)
}

var shadowsLight, shadowsDark = shadows(tokensCSS)

// shadowOf is the named shadow of the theme c draws with, for what draws
// without the App's palette.
func ShadowOf(c *ui.Context, name string) shadow {
	if c.Theme().Dark {
		return shadowsDark[name]
	}
	return shadowsLight[name]
}

// shadows reads the --shadow-* of tokens.css as tokens does the colors.
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

// scrims reads the --scrim of tokens.css, the light one and the dark.
func scrims(css string) (light, dark ui.Color) {
	decl := regexp.MustCompile(`--scrim\s*:\s*rgba\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*([0-9.]+)\s*\)`)
	f := func(s string) float32 {
		v, _ := strconv.ParseFloat(s, 32)
		return float32(v)
	}
	var found [2]bool
	blocks(css, func(isDark bool, body string) {
		if d := decl.FindStringSubmatch(body); d != nil {
			c := ui.RGBA(uint8(f(d[1])), uint8(f(d[2])), uint8(f(d[3])), f(d[4]))
			if isDark {
				dark, found[1] = c, true
			} else {
				light, found[0] = c, true
			}
		}
	})
	if !found[1] {
		dark = light
	}
	return light, dark
}

func LightPalette() *Palette {
	k, _ := colorTokens(tokensCSS)
	s, _ := shadows(tokensCSS)
	scrim, _ := scrims(tokensCSS)
	t := ui.LightTheme()
	themeOf(t, k)
	t.Selection = ui.RGBA(50, 120, 215, 0.2) // --sel over the page, so the text shows through
	t.Focus = k["ink"].Alpha(0.35)
	t.Scrollbar = ui.RGBA(0, 0, 0, 0.2)
	return paletteOf(t, k, s, scrim)
}

func DarkPalette() *Palette {
	_, k := colorTokens(tokensCSS)
	_, s := shadows(tokensCSS)
	_, scrim := scrims(tokensCSS)
	t := ui.DarkTheme()
	themeOf(t, k)
	t.Selection = ui.RGBA(90, 140, 220, 0.35)
	t.Focus = k["ink"].Alpha(0.55)
	t.Scrollbar = ui.RGBA(255, 255, 255, 0.2)
	return paletteOf(t, k, s, scrim)
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
	t.Radius = RControl
}

func paletteOf(t *ui.Theme, k map[string]ui.Color, s map[string]shadow, scrim ui.Color) *Palette {
	return &Palette{Theme: t,
		Canvas: k["bg"], Side: k["side"], Raised: k["raised"], Bubble: k["hover"],
		Line: k["line"], Faint: k["faint"], Ink: k["ink"], OnInk: k["on-ink"],
		Dim: k["dim"], Control: k["control"], PhoneEye: k["phone-eye"],
		Amber: k["amber"], WarnBg: k["warn-bg"], WarnBd: k["warn-bd"],
		Raise: s["raise"], Pop: s["pop"], Dialog: s["dialog"], Scrim: scrim}
}

// dot is a small status disc.
func Dot(c *ui.Context, color ui.Color, size float32) ui.Element {
	return ui.Box(c).Size(size, size).Radius(RRound).Background(color).Shrink(0)
}

// ghost is the text button, the one look of words to click that stand in no
// box: ink over a thin line. Under the pointer it has a fill and the line
// takes the color of the words. color, when given, is for the words: the
// danger color, for what cannot be taken back.
func (p *Palette) Ghost(c *ui.Context, text string, color ui.Color) ui.Element {
	b := ui.ButtonBase(c).Column().Padding(2, 6).Radius(RXs).Shrink(0)
	fg, line := color, p.Control
	if fg.A == 0 {
		fg = p.Ink
	}
	if b.Hovered() {
		b.Background(p.Theme.SurfacePressed)
		line = fg
	}
	if b.IsDisabled() {
		Pale(b)
	}
	b.Children(func() {
		ui.Text(c, text).FontSize(FsNote).TextColor(fg).SingleLine().Ellipsis("…")
		ui.Box(c).FillWidth().Height(1).Background(line).Shrink(0)
	})
	return b
}

// chip is a choice turned on and off: outlined, and solid ink when on.
func (p *Palette) Chip(c *ui.Context, text string, on bool) ui.Element {
	t := p.Theme
	b := ui.ButtonBase(c).Row().Center().Height(HSeg).Padding(0, 10).Radius(RControl).Shrink(0)
	bg, edge, fg := p.Canvas, t.Border, t.TextMuted
	switch {
	case on && b.Hovered():
		bg, edge, fg = t.AccentHover, t.AccentHover, p.OnInk
	case on:
		bg, edge, fg = p.Ink, p.Ink, p.OnInk
	case b.Hovered():
		bg = t.SurfaceHover
	}
	b.Background(bg).Border(1, edge)
	b.Children(func() { ui.Text(c, text).FontSize(FsNote).TextColor(fg).SingleLine() })
	return b
}

// icon parses a 24×24 stroked icon drawn in the text color.
func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	IcOutline     = icon(`<path d="M5 7h14M5 12h9M5 17h11"/>`)
	IcChat        = icon(`<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>`)
	IcTag         = icon(`<path d="M20.6 13.4 13.4 20.6a2 2 0 0 1-2.8 0L3 13V3h10l7.6 7.6a2 2 0 0 1 0 2.8z"/><circle cx="7.5" cy="7.5" r="1.2"/>`)
	IcGear        = icon(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/>`)
	IcPlus        = icon(`<path d="M12 5v14M5 12h14"/>`)
	IcSend        = icon(`<path d="M12 19V5M5 12l7-7 7 7"/>`)
	IcSparkle     = icon(`<path d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8z"/><path d="M19 17l.7 1.8 1.8.7-1.8.7L19 22l-.7-1.8-1.8-.7 1.8-.7z"/>`)
	IcQuote       = icon(`<path d="M4 6h16M4 12h10M4 18h7"/>`)
	IcCopy        = icon(`<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/>`)
	IcPanel       = icon(`<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M15 4v16"/>`)
	IcSide        = icon(`<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16"/>`)
	icPencil      = icon(`<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z"/>`)
	icUndo        = icon(`<path d="M9 14 4 9l5-5"/><path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11"/>`)
	IcEye         = icon(`<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>`)
	icRetry       = icon(`<path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/>`)
	IcDone        = icon(`<circle cx="12" cy="12" r="9"/><path d="m8 12.5 2.8 2.8 5.7-5.8"/>`)
	IcCheck       = icon(`<path d="M20 6 9 17l-5-5"/>`)
	IcX           = icon(`<path d="M18 6 6 18M6 6l12 12"/>`)
	icClock       = icon(`<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`)
	IcSearch      = icon(`<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>`)
	IcAlert       = icon(`<path d="M12 9v4M12 17h.01"/><path d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/>`)
	IcChevron     = icon(`<path d="m9 18 6-6-6-6"/>`)
	IcBack        = icon(`<path d="m15 6-6 6 6 6"/>`)
	IcForward     = icon(`<path d="m9 6 6 6-6 6"/>`)
	IcChevronUp   = icon(`<path d="m18 15-6-6-6 6"/>`)
	icChip        = icon(`<path d="M9 3v3M15 3v3M9 18v3M15 18v3M3 9h3M3 15h3M18 9h3M18 15h3M7 6h10a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1zM10 10h4v4h-4z"/>`)
	IcChevronDown = icon(`<path d="m6 9 6 6 6-6"/>`)
	IcBook        = icon(`<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/>`)
	IcFolder      = icon(`<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>`)
	IcFolderPlus  = icon(`<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><path d="M12 10.5v5M9.5 13h5"/>`)
	IcDoc         = icon(`<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/>`)
	IcTrash       = icon(`<path d="M3 6h18"/><path d="M8 6V4h8v2"/><path d="M19 6l-1 14H6L5 6"/>`)
	IcPen         = icon(`<path d="M12 20h8"/><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4z"/>`)
	IcFlag        = icon(`<path d="M6 20V4h10l-2 4 2 4H6"/>`)
	IcLink        = icon(`<path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1"/><path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1"/>`)
	IcBookmark    = icon(`<path d="M6 3h12v18l-6-4-6 4z"/><path d="M12 7v6M9 10h6"/>`)
	IcMonitor     = icon(`<path d="M4 5h16v11H4zM9 20h6M12 16v4"/>`)
	IcSun         = icon(`<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>`)
	IcMoon        = icon(`<path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5z"/>`)
	icWand        = icon(`<path d="m14 4 6 6L9 21l-6-6z"/><path d="M11 7l6 6"/>`)
	icFeather     = icon(`<path d="M20 4C11 4 5 10 5 19v1"/><path d="M5 15h8"/><path d="M20 4c0 7-4 11-11 11"/>`)
	IcList        = icon(`<path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/>`)
	IcGrid        = icon(`<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>`)
	IcMore        = icon(`<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>`)
	IcExpand      = icon(`<path d="M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7"/>`)
	IcShrink      = icon(`<path d="M4 14h6v6M20 10h-6V4M14 10l7-7M3 21l7-7"/>`)
)

// glyph shows an icon at size in color (zero: inherit the text color).
func Glyph(c *ui.Context, s *ui.SVG, size float32, color ui.Color) ui.Element {
	e := ui.Icon(c, s).Size(size, size).Shrink(0)
	if color.A != 0 {
		e.TextColor(color)
	}
	return e
}

// iconButton is a square borderless button with a tooltip; label also names
// it for tests and assistive technology.
func IconButton(c *ui.Context, s *ui.SVG, label string, color ui.Color) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Size(HControlSm, HControlSm).Center().Radius(RControl).Shrink(0).Label(label).Tooltip(label)
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
		Pale(b)
	}
	b.Children(func() { Glyph(c, s, IconMd, fg) })
	return b
}

// primary is the one solid ink button of a view.
func (p *Palette) Primary(c *ui.Context, text string) ui.Element {
	b := ui.ButtonBase(c).Row().Center().Gap(6).Height(HControlSm).Padding(0, 14).Radius(RControl).Shrink(0)
	bg := p.Ink
	if b.Hovered() {
		bg = p.Theme.AccentHover
	}
	b.Background(bg)
	if b.IsDisabled() {
		Pale(b)
	}
	b.Children(func() { ui.Text(c, text).FontSize(FsControl).TextColor(p.OnInk).SingleLine() })
	return b
}

// secondary is an outlined button.
// danger is the button for what cannot be taken back, once the author has
// asked for it.
func (p *Palette) Danger(c *ui.Context, text string) ui.Element {
	t := p.Theme
	b := ui.ButtonBase(c).Row().Center().Gap(6).Height(HControlSm).Padding(0, 12).Radius(RControl).Background(t.Danger).Shrink(0)
	if b.Hovered() {
		b.Background(t.Danger.Alpha(0.85))
	}
	if b.IsDisabled() {
		Pale(b)
	}
	b.Children(func() { ui.Text(c, text).FontSize(FsControl).TextColor(p.OnInk).SingleLine() })
	return b
}

// soft is the button that lets go of what the AI does or proposed, in its
// line under a chapter's title and after each proposal: on a pressed fill,
// with no line round it, a step darker under the pointer.
func (p *Palette) Soft(c *ui.Context, text string) ui.Element {
	t := p.Theme
	b := ui.ButtonBase(c).Row().Center().Gap(6).Height(HControlSm).Padding(0, 12).Radius(RControl).Background(t.SurfacePressed).Shrink(0)
	if b.Hovered() {
		b.Background(p.Dim)
	}
	if b.IsDisabled() {
		Pale(b)
	}
	b.Children(func() { ui.Text(c, text).FontSize(FsControl).SingleLine() })
	return b
}

func (p *Palette) Secondary(c *ui.Context, text string) ui.Element {
	t := p.Theme
	b := ui.ButtonBase(c).Row().Center().Gap(6).Height(HControlSm).Padding(0, 12).Radius(RControl).Border(1, t.Border).Background(p.Canvas).Shrink(0)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	if b.IsDisabled() {
		Pale(b)
	}
	b.Children(func() { ui.Text(c, text).FontSize(FsControl).SingleLine() })
	return b
}

// segments is the segmented control: the choice stands raised on a darker
// track. label names the control, names its choices; at is the one chosen.
// The track returned takes the width it is given, its choices only their
// words'.
func (p *Palette) Segments(c *ui.Context, at *int, label string, names []string) ui.Element {
	t := p.Theme
	seg := ui.SegmentedBase(c, at, len(names))
	return seg.Track.Label(label).Justify(ui.Start).Gap(2).Padding(2).Radius(RControl).Background(t.SurfacePressed).Shrink(0).Children(func() {
		for i, name := range names {
			s := seg.Segment(i).Height(HSeg).Center().Padding(0, 10).Radius(RXs)
			fg, weight := t.TextMuted, 400
			if i == *at {
				p.Raise.On(s.Background(p.Canvas))
				fg, weight = t.Text, 600
			}
			s.Children(func() { ui.Text(c, name).FontSize(FsControl).FontWeight(weight).TextColor(fg).SingleLine() })
		}
	})
}

// dots is a one-of-several choice in a form: a row of radio buttons, each
// as wide as its words, choosing among values into at; names are what
// they say, label names the row. It says whether one was chosen.
func Dots[T comparable](c *ui.Context, at *T, label string, values []T, names []string) (changed bool) {
	ui.RadioGroup(c, func() {
		for i, v := range values {
			if ui.Radio(c, at, v, names[i]).Changed() {
				changed = true
			}
		}
	}).Row().Gap(24).MinHeight(HControl).AlignItems(ui.Center).Label(label)
	return changed
}

// group is a rounded, hairline-bordered container whose rows are separated
// by hairlines, as in system settings.
func (p *Palette) group(c *ui.Context) ui.Element {
	return ui.Column(c).FillWidth().Radius(RCard).Border(1, p.Theme.Border).Background(p.Raised).Clip()
}

func (p *Palette) Hairline(c *ui.Context) {
	ui.Box(c).FillWidth().Height(1).Background(p.Line).Shrink(0)
}

// action is a tinted button; strong fills it, for destructive confirmations.
func Action(c *ui.Context, text string, color ui.Color, strong bool) ui.Element {
	b := ui.ButtonBase(c).Row().Center().Padding(6, 12).Radius(RControl).Shrink(0)
	bg, fg := color.Alpha(0.12), color
	if strong {
		bg, fg = color, ui.Hex("#ffffff")
	}
	if b.Hovered() {
		bg = bg.Mix(color, 0.2)
	}
	b.Background(bg)
	if b.IsDisabled() {
		Pale(b)
	}
	b.Children(func() { ui.Text(c, text).FontSize(FsNote).FontWeight(600).TextColor(fg).SingleLine() })
	return b
}
