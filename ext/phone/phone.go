package phone

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/ext/phone/phonetext"
)

// Phone points and em ratios come from prototype/pane.js phoneView and
// prototype/index.html's phone preview styles. An em uses the current
// phone font size (title margins use its own 1.3em font). The popover's
// width is in desktop pixels; desktop controls keep the theme's tokens.
const (
	phoneSizeDefault         = 20
	phoneSizeMin             = 16
	phoneSizeMax             = 24
	phoneStatusHeight        = 44
	phoneStatusInset         = 28
	phoneTextInset           = 20
	phoneTextTop             = 8
	phoneFootHeight          = 34
	phoneChromeHeight        = phoneStatusHeight + phoneTextTop + phoneFootHeight
	phoneFrameEM             = 0.6
	phoneFrameRadiusEM       = 2.6
	phoneScreenRadiusEM      = 2
	phoneTitleEM             = 1.3
	phoneTitleTopEM          = 0.4
	phoneTitleBottomEM       = 1.2
	phoneIndentEM            = 2
	phoneParaGapEM           = 0.5
	phoneParaRadiusEM        = 0.2
	phoneBatteryWidthEM      = 1.7
	phoneBatteryHeightEM     = 0.8
	phoneBatteryRadiusEM     = 0.25
	phoneBatteryInsetEM      = 0.12
	phoneBatteryEmpty        = 0.35
	phoneBatteryFillRadiusEM = 0.1
	phoneStatusSizeEM        = 0.7
	phoneFootSizeEM          = 0.65
	phoneCaretWidthEM        = 0.1
	phonePopWidth            = 248
)

var (
	phoneScreens       = map[string][2]float32{"narrow": {360, 780}, "standard": {393, 852}, "wide": {430, 932}}
	phoneModes         = []string{"flip", "scroll"}
	phoneScreenChoices = []string{"narrow", "standard", "wide"}
	phoneSpacings      = []string{"tight", "medium", "loose"}
	phoneThemes        = []string{"day", "night", "eye"}
)

func defaultPhone() Typesetting {
	return Typesetting{Mode: "flip", Screen: "standard", Size: phoneSizeDefault, Spacing: "medium", Theme: "day"}
}

// UI indexes never leave the phone package. Persisted choices keep their meaning
// when controls are reordered; missing and unrecognized values default.
func phoneIndex(value string, choices []string) int {
	return max(0, slices.Index(choices, value))
}

// Preferences returns the local choices with defaults for invalid fields,
// without storing the defaults.
func (p *Panel) Preferences() Typesetting {
	d := defaultPhone()
	prefs := p.setting.Get()
	choice := func(value, fallback string, choices []string) string {
		if slices.Contains(choices, value) {
			return value
		}
		return fallback
	}
	prefs.Mode = choice(prefs.Mode, d.Mode, phoneModes)
	prefs.Screen = choice(prefs.Screen, d.Screen, phoneScreenChoices)
	prefs.Spacing = choice(prefs.Spacing, d.Spacing, phoneSpacings)
	prefs.Theme = choice(prefs.Theme, d.Theme, phoneThemes)
	if prefs.Size < phoneSizeMin || prefs.Size > phoneSizeMax {
		prefs.Size = d.Size
	}
	return prefs
}

type phoneAnchor struct {
	fraction, into float32
	paragraph      int
}

// Panel holds the phone preview, its reading position, and its layout.
type Panel struct {
	book    *ext.Ref[hostapi.Book]
	palette *ext.Ref[hostapi.Palette]
	setting *ext.Pref[Typesetting]
	session *ext.Session
	editor.PaginatedText
	prefs         Typesetting
	look          bool
	doc           *editor.Doc
	version       int
	measured      Typesetting
	key, title    string
	at, caretPage int
	scrollY       float32
	anchor        phoneAnchor
	anchored      bool
	// Geometry is retained for input; no frame-owned UI handles escape.
	stage, body   ui.Rect
	ox, oy, scale float32
	hover         int
	pick          []ui.Rect
	caret         ui.Rect
}

func (p *Panel) flip(d int) {
	if to := p.at + d; to >= 0 && to < p.Pages {
		p.at = to
	}
}

func (p *Panel) nextLong() {
	if len(p.LongPages) == 0 {
		return
	}
	for _, page := range p.LongPages {
		if page > p.at {
			p.at = page
			return
		}
	}
	p.at = p.LongPages[0]
}

func phonePagination(p Typesetting) editor.PaginationOptions {
	fs := float32(p.Size)
	lh := float32(kit.LhRead)
	switch p.Spacing {
	case "tight":
		lh = kit.LhLoose
	case "loose":
		lh = kit.LhPaper
	}
	titleSize := phoneTitleEM * fs
	return editor.PaginationOptions{
		Width:  phoneScreens[p.Screen][0] - 2*phoneTextInset,
		Height: phoneScreens[p.Screen][1] - phoneChromeHeight,
		Font:   ui.Font{Size: fs}, TitleFont: ui.Font{Size: titleSize, Weight: 700},
		LineHeight: lh, TitleLineHeight: kit.LhTight,
		Indent: phoneIndentEM * fs, ParagraphGap: phoneParaGapEM,
		TitleTop: titleSize * phoneTitleTopEM, TitleBottom: titleSize * phoneTitleBottomEM,
		MarkLong: !p.NoLong, Scroll: p.Mode == "scroll",
	}
}

func (p *Panel) point(x, y float32) (para, offset int, ok bool) {
	if p.scale <= 0 {
		return
	}
	x, y = (x-p.ox)/p.scale, (y-p.oy)/p.scale
	if x < p.body.X || x > p.body.X+p.body.W || y < p.body.Y || y > p.body.Y+p.body.H {
		return
	}
	x, y = x-p.body.X, y-p.body.Y
	return p.Hit(x, y, p.at, p.scrollY)
}

func (p *Panel) measure() {
	id, ed, _ := p.book.Get().Chapter()
	p.prefs = p.Preferences()
	name, ok := p.book.Get().ChapterTitle(id)
	if !ok || ed == nil {
		return
	}
	title := strings.Replace(name, "：", " ", 1)
	changed := p.doc != ed.Doc || p.version != ed.Doc.Version() || p.measured != p.prefs || p.title != title
	if changed {
		p.anchored = false
		p.doc, p.version, p.measured, p.title = ed.Doc, ed.Doc.Version(), p.prefs, title
		p.Layout(ed.Doc.String(), p.title, phonePagination(p.prefs))
	}
	page := p.PageOf(ed.Caret())
	key := p.session.Book().Dir + "/" + id
	if p.key != key {
		p.key, p.at, p.caretPage = key, 0, page
	} else if p.prefs.Mode == "flip" && p.caretPage != page {
		p.caretPage, p.at = page, page
	}
	p.at = min(max(p.at, 0), p.Pages-1)
}

// View draws the preview on the frame thread without retaining frame handles.
func (p *Panel) View(c *ui.Context) {
	pal := p.palette.Get().Colors()
	if _, _, open := p.book.Get().Chapter(); !open {
		p.key = ""
		p.look = false
		pal.PaneEmpty(c, phonetext.PreviewOpenChapter(), "")
		return
	}
	p.measure()
	before := p.prefs
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Padding(0, 20, 12, 20).Shrink(0).Children(func() {
		mode := phoneIndex(p.prefs.Mode, phoneModes)
		pal.Segments(c, &mode, phonetext.PreviewViewMode(), []string{phonetext.PreviewPaged(), phonetext.PreviewScroll()})
		p.prefs.Mode = phoneModes[mode]
		ui.Spacer(c)
		b := ui.ButtonBase(c).Height(kit.HControlSm).Padding(0, 6).Center().Radius(kit.RControl).KeepFocus().Label("Aa").Tooltip(phonetext.PreviewLayout())
		if b.Hovered() || p.look {
			b.Background(pal.Theme.SurfacePressed)
		}
		b.Children(func() { ui.Text(c, "Aa").FontSize(kit.FsControl).FontWeight(600) })
		if b.Clicked() {
			p.look = !p.look
		}
		kit.Popover(c, b, &p.look, 1, 0, func(panel ui.Element) {
			panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(8, 0, 0, 0).Width(phonePopWidth).Column().Padding(4).Gap(2).Radius(kit.RCard).Background(pal.Raised).Border(1, pal.Theme.Border)
			pal.Pop.On(panel)
			p.typesetting(c)
		})
	})
	if before != p.prefs {
		prefs := p.prefs
		p.setting.Set(prefs)
		p.measure()
	}
	pal.Hairline(c)
	stage := ui.Box(c).FillWidth().Grow(1).Shrink(1).Clip().Label("手机预览正文")
	p.stage = stage.Bounds()
	p.hover = -1
	if x, y, ok := stage.PointerPosition(); ok {
		if para, _, ok := p.point(x, y); ok {
			p.hover = para
			stage.Cursor(ui.CursorPointer)
			if !p.prefs.NoLong && len(p.Paras[para].Shape.Lines) > 5 {
				stage.Tooltip(phonetext.PreviewLinesTip(len(p.Paras[para].Shape.Lines)))
			}
		}
	}
	stage.HandleInput(p.input).Draw(p.paint)
	pal.Hairline(c)
	ui.Row(c).FillWidth().Height(kit.HControl).Padding(0, 20).AlignItems(ui.Center).Justify(ui.SpaceBetween).Shrink(0).Children(func() {
		if p.prefs.Mode == "flip" {
			ui.Box(c).Disabled(p.at == 0).Children(func() {
				if pal.Ghost(c, "‹ "+phonetext.PreviewPrev(), ui.Color{}).Label("‹ " + phonetext.PreviewPrev()).KeepFocus().Clicked() {
					p.flip(-1)
				}
			})
			ui.Text(c, phonetext.PreviewScreenOf(p.at+1, p.Pages)).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted).SingleLine()
			ui.Box(c).Disabled(p.at == p.Pages-1).Children(func() {
				if pal.Ghost(c, phonetext.PreviewNext()+" ›", ui.Color{}).Label(phonetext.PreviewNext() + " ›").KeepFocus().Clicked() {
					p.flip(1)
				}
			})
		} else {
			ui.Spacer(c)
			ui.Text(c, phonetext.PreviewAbout(p.Pages)).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted)
			ui.Spacer(c)
		}
	})
	ui.Row(c).FillWidth().MinHeight(kit.HControl).Padding(0, 20).Justify(ui.End).Shrink(0).Children(func() {
		if p.LongCount > 0 {
			text := phonetext.PreviewLong(p.LongCount, 5)
			if p.prefs.Mode == "flip" {
				text = phonetext.PreviewLongNext(p.LongCount, 5)
			}
			if pal.Ghost(c, text, ui.Color{}).Label("下一处长段").KeepFocus().Clicked() && p.prefs.Mode == "flip" {
				p.nextLong()
			}
		}
	})
}

func (p *Panel) typesetting(c *ui.Context) {
	pal := p.palette.Get().Colors()
	prefs := &p.prefs
	row := func(label string, value *string, choices, names []string) {
		at := phoneIndex(*value, choices)
		pal.PopHead(c, label)
		ui.Box(c).FillWidth().Padding(2, 6, 6, 6).Children(func() {
			p.segments(c, &at, label, names)
		})
		*value = choices[at]
	}
	row(phonetext.PreviewScreen(), &prefs.Screen, phoneScreenChoices, []string{phonetext.PreviewNarrow(), phonetext.PreviewStandard(), phonetext.PreviewWide()})
	pal.PopHead(c, phonetext.PreviewFont())
	ui.Box(c).FillWidth().Padding(2, 6, 6, 6).Children(func() {
		ui.Row(c).FillWidth().Padding(2).Radius(kit.RControl).Background(pal.Theme.SurfacePressed).AlignItems(ui.Center).Children(func() {
			step := func(label, tip string, off bool, d int) {
				b := ui.ButtonBase(c).Grow(1).Height(kit.HSeg).Center().Radius(kit.RXs).Disabled(off).Label(label).Tooltip(tip)
				if b.Hovered() {
					b.Background(pal.Theme.SurfaceHover)
				}
				if off {
					kit.Pale(b)
				}
				b.Children(func() { ui.Text(c, label).FontSize(kit.FsControl) })
				if b.Clicked() {
					prefs.Size += d
				}
			}
			step("A−", phonetext.PreviewFontDown(), prefs.Size <= phoneSizeMin, -1)
			ui.Textf(c, "%d", prefs.Size).Grow(1).TextAlign(ui.Center).FontSize(kit.FsControl)
			step("A+", phonetext.PreviewFontUp(), prefs.Size >= phoneSizeMax, 1)
		})
	})
	row(phonetext.PreviewLineSpacing(), &prefs.Spacing, phoneSpacings, []string{phonetext.PreviewTight(), phonetext.PreviewMedium(), phonetext.PreviewLoose()})
	row(phonetext.PreviewBackground(), &prefs.Theme, phoneThemes, []string{phonetext.PreviewDay(), phonetext.PreviewNight(), phonetext.PreviewEyeCare()})
	pal.PopLine(c)
	ui.Column(c).FillWidth().Gap(6).Padding(6, 10).Children(func() {
		frame, long := !prefs.NoFrame, !prefs.NoLong
		ui.Checkbox(c, &frame, phonetext.PreviewFrame())
		ui.Checkbox(c, &long, phonetext.PreviewLongTick(5))
		prefs.NoFrame, prefs.NoLong = !frame, !long
	})
	ui.Row(c).FillWidth().Justify(ui.End).Padding(0, 10, 6, 10).Children(func() {
		if pal.Ghost(c, phonetext.PreviewReset(), ui.Color{}).Clicked() {
			mode := prefs.Mode
			*prefs = defaultPhone()
			prefs.Mode = mode
		}
	})
}

func (p *Panel) segments(c *ui.Context, at *int, label string, names []string) {
	pal := p.palette.Get().Colors()
	t := pal.Theme
	seg := ui.SegmentedBase(c, at, len(names))
	seg.Track.Label(label).FillWidth().Gap(2).Padding(2).Radius(kit.RControl).Background(t.SurfacePressed).Children(func() {
		for i, name := range names {
			s := seg.Segment(i).Grow(1).Shrink(1).Height(kit.HSeg).Center().Padding(0, 10).Radius(kit.RXs)
			color, weight := t.TextMuted, 400
			if i == *at {
				pal.Raise.On(s.Background(pal.Canvas))
				color, weight = t.Text, 600
			}
			s.Children(func() { ui.Text(c, name).FontSize(kit.FsControl).FontWeight(weight).TextColor(color).SingleLine() })
		}
	})
}

func (p *Panel) input(ev ui.InputEvent) bool {
	if ev.Kind == ui.InputScroll && p.prefs.Mode == "scroll" && p.scale > 0 {
		p.scrollY = min(max(p.scrollY+ev.DY/p.scale, 0), max(p.Height-p.PageHeight, 0))
		return true
	}
	if ev.Kind != ui.InputPointerDown || ev.Button != 0 {
		return false
	}
	if _, offset, ok := p.point(ev.X, ev.Y); ok {
		_, ed, _ := p.book.Get().Chapter()
		ed.Select(offset, offset)
		ed.Focus()
		return true
	}
	return false
}

func (p *Panel) sync() {
	_, ed, _ := p.book.Get().Chapter()
	f, n, into := ed.ScrollAnchor()
	anchor := phoneAnchor{f, into, n}
	if p.anchored && anchor == p.anchor {
		return
	}
	p.anchor, p.anchored = anchor, true
	most := max(p.Height-p.PageHeight, 0)
	switch {
	case f <= 0:
		p.scrollY = 0
	case f >= 1 || n >= len(p.Paras):
		p.scrollY = most
	default:
		para := p.Paras[n]
		p.scrollY = min(max(para.RollY+into*para.Shape.Height-f*p.PageHeight, 0), most)
	}
}

func (p *Panel) paint(painter *ui.Painter, r ui.Rect) {
	if p.doc == nil {
		return
	}
	if p.prefs.Mode == "scroll" {
		p.sync()
	}
	fs := float32(p.prefs.Size)
	w, h := phoneScreens[p.prefs.Screen][0], phoneScreens[p.prefs.Screen][1]
	pad, radius := float32(0), float32(0)
	if !p.prefs.NoFrame {
		pad = phoneFrameEM * fs
		radius = phoneFrameRadiusEM * fs
	}
	wfull, hfull := w+2*pad+2, h+2*pad+2
	k := max(0, min(1, min(r.W/wfull, r.H/hfull)))
	p.ox, p.oy, p.scale = (r.W-wfull*k)/2, (r.H-hfull*k)/2, k
	p.body = ui.Rect{X: pad + 1 + phoneTextInset, Y: pad + 1 + phoneStatusHeight + phoneTextTop, W: p.TextWidth, H: p.PageHeight}
	light, dark := p.palette.Get().ScreenColors()
	bg, ink, muted, active, hover := light.Canvas, light.Ink, light.Faint, light.Theme.SurfacePressed, light.Theme.SurfaceHover
	if p.prefs.Theme == "night" {
		bg, ink, muted, active, hover = dark.Canvas, dark.Ink, dark.Faint, dark.Theme.SurfacePressed, dark.Theme.SurfaceHover
	}
	if p.prefs.Theme == "eye" {
		bg = light.PhoneEye
	}
	p.pick, p.caret = p.pick[:0], ui.Rect{}
	painter.Clip(r, 0, func() {
		painter.Transform(ui.Scaling(k, k).Then(ui.Translation(r.X+p.ox, r.Y+p.oy)), 0, 0, func() {
			painter.Fill(ui.Rect{W: wfull, H: hfull}, dark.Theme.Border, radius)
			screen := ui.Rect{X: pad + 1, Y: pad + 1, W: w, H: h}
			innerRadius := float32(0)
			if !p.prefs.NoFrame {
				innerRadius = phoneScreenRadiusEM * fs
			}
			painter.Clip(screen, innerRadius, func() {
				painter.Fill(screen, bg, innerRadius)
				statusSize := phoneStatusSizeEM * fs
				statusMetrics := (ui.Font{Size: statusSize}).Metrics()
				painter.Text(screen.X+phoneStatusInset, screen.Y+(phoneStatusHeight-statusMetrics.Ascent-statusMetrics.Descent)/2, time.Now().Format("15:04"), statusSize, ink)
				battery := ui.Rect{X: screen.X + w - phoneStatusInset - fs*phoneBatteryWidthEM, Y: screen.Y + (phoneStatusHeight-fs*phoneBatteryHeightEM)/2, W: fs * phoneBatteryWidthEM, H: fs * phoneBatteryHeightEM}
				painter.Stroke(battery, ink, fs*phoneBatteryRadiusEM, 1)
				inset := fs * phoneBatteryInsetEM
				painter.Fill(ui.Rect{X: battery.X + inset, Y: battery.Y + inset, W: battery.W*(1-phoneBatteryEmpty) - inset, H: battery.H - 2*inset}, ink, fs*phoneBatteryFillRadiusEM)
				painter.Clip(p.body, 0, func() {
					p.paintText(painter, ink, active, hover)
				})
				footSize := phoneFootSizeEM * fs
				foot := ui.Rect{X: screen.X + phoneTextInset, Y: screen.Y + h - phoneFootHeight, W: w - 2*phoneTextInset, H: phoneFootHeight}
				painter.Clip(foot, 0, func() {
					painter.Text(foot.X, foot.Y+8, kit.Clip(p.title, 20), footSize, muted)
				})
				if p.prefs.Mode == "flip" {
					at := fmt.Sprintf("%d/%d", p.at+1, p.Pages)
					gs := ui.Shape(at, ui.Font{Size: footSize})
					width := float32(0)
					for _, g := range gs {
						width = max(width, g.X+g.Advance)
					}
					box := ui.Rect{X: screen.X + w - phoneTextInset - width, Y: foot.Y, W: width + 2, H: foot.H}
					painter.Fill(box, bg, 0)
					painter.Text(box.X, foot.Y+8, at, footSize, muted)
				}
			})
		})
	})
}

func (p *Panel) paintText(painter *ui.Painter, ink, active, hover ui.Color) {
	_, ed, _ := p.book.Get().Chapter()
	from, to := ed.Selection()
	// A paragraph is one continuous patch, including when its lines are
	// split across pages. Selection is drawn over it, before the glyphs.
	var patch ui.Rect
	previous := -1
	flush := func() {
		if previous < 0 {
			return
		}
		color := active
		if p.hover == previous {
			color = hover
		} else if p.prefs.NoLong || len(p.Paras[previous].Shape.Lines) <= 5 {
			return
		}
		painter.Fill(patch, color, float32(p.prefs.Size)*phoneParaRadiusEM)
	}
	for _, l := range p.Lines {
		if l.Para < 0 || p.prefs.Mode == "flip" && l.Page != p.at {
			continue
		}
		y, h := p.LineY(l, p.scrollY), p.Paras[l.Para].Shape.LineHeight
		if y+h <= 0 || y >= p.PageHeight {
			continue
		}
		if previous != l.Para {
			flush()
			previous = l.Para
			patch = ui.Rect{X: p.body.X, Y: p.body.Y + y, W: p.TextWidth, H: h}
		} else {
			patch.H = p.body.Y + y + h - patch.Y
		}
	}
	flush()
	for _, l := range p.Lines {
		if p.prefs.Mode == "flip" && l.Page != p.at {
			continue
		}
		y := p.LineY(l, p.scrollY)
		sh := p.TitleShape
		if l.Para >= 0 {
			sh = p.Paras[l.Para].Shape
		}
		if y+sh.LineHeight <= 0 || y >= p.PageHeight {
			continue
		}
		x, y := p.body.X, p.body.Y+y
		if l.Para >= 0 {
			para := p.Paras[l.Para]
			ln := sh.Lines[l.Line]
			lo, hi := max(from-para.Start, ln.Start), min(to-para.Start, ln.End)
			if from != to && lo < hi {
				x0, x1 := sh.X(l.Line, lo), sh.X(l.Line, hi)
				pick := ui.Rect{X: x + x0, Y: y + 2, W: x1 - x0, H: sh.LineHeight - 4}
				p.pick = append(p.pick, pick)
				selection := p.palette.Get().Colors().Theme.Selection
				if !ed.Focused() {
					selection = p.palette.Get().Colors().Theme.TextMuted.Alpha(0.2)
				}
				painter.Fill(pick, selection, 0)
			}
			at := ed.Caret() - para.Start
			if from == to && ed.Focused() && at >= ln.Start && (at < ln.End || at == ln.End && l.Line == len(sh.Lines)-1) {
				p.caret = ui.Rect{X: x + sh.X(l.Line, at), Y: y, W: float32(p.prefs.Size) * phoneCaretWidthEM, H: sh.LineHeight}
				painter.Fill(p.caret, ink, 0)
			}
		}
		sh.DrawLine(painter, l.Line, x, y, ink)
	}
}
