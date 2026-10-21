package desk

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/editor"
	"writer/shelf"
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

func defaultPhone() shelf.Phone {
	return shelf.Phone{Mode: "flip", Screen: "standard", Size: phoneSizeDefault, Spacing: "medium", Theme: "day"}
}

// UI indexes never leave the desk. Persisted choices keep their meaning
// when controls are reordered; missing and unrecognized values default.
func phoneIndex(value string, choices []string) int {
	return max(0, slices.Index(choices, value))
}

func (a *App) phonePreferences() shelf.Phone {
	d := defaultPhone()
	if a.shelf.Phone == nil {
		return d
	}
	p := *a.shelf.Phone
	choice := func(value, fallback string, choices []string) string {
		if slices.Contains(choices, value) {
			return value
		}
		return fallback
	}
	p.Mode = choice(p.Mode, d.Mode, phoneModes)
	p.Screen = choice(p.Screen, d.Screen, phoneScreenChoices)
	p.Spacing = choice(p.Spacing, d.Spacing, phoneSpacings)
	p.Theme = choice(p.Theme, d.Theme, phoneThemes)
	if p.Size < phoneSizeMin || p.Size > phoneSizeMax {
		p.Size = d.Size
	}
	return p
}

type phoneShapeKey struct {
	text                  string
	font                  ui.Font
	width, height, indent float32
}

type phonePara struct {
	start int
	shape *editor.Paragraph
	rollY float32
}

type phoneAnchor struct {
	fraction, into float32
	paragraph      int
}

type phoneLine struct {
	para, line, page int // para -1 is the chapter title
	y, rollY         float32
}

type phoneState struct {
	prefs                        shelf.Phone
	look                         bool
	cache                        map[phoneShapeKey]*editor.Paragraph
	doc                          *editor.Doc
	version                      int
	measured                     shelf.Phone
	key, title                   string
	titleShape                   *editor.Paragraph
	paras                        []phonePara
	lines                        []phoneLine
	pages, at, caretPage         int
	pageH, textW, fullH, scrollY float32
	longPages                    []int
	longCount                    int
	anchor                       phoneAnchor
	anchored                     bool
	// Geometry is retained for input; no frame-owned UI handles escape.
	stage, body   ui.Rect
	ox, oy, scale float32
	hover         int
	pick          []ui.Rect
	caret         ui.Rect
}

func (a *App) openPhone() {
	if open, _ := a.chapterOpen(); !open {
		return
	}
	a.doc.hidden, a.doc.tab, a.write.focus = false, panePhone, false
	a.savePrefs()
}

func (s *phoneState) flip(d int) {
	if to := s.at + d; to >= 0 && to < s.pages {
		s.at = to
	}
}

func (s *phoneState) nextLong() {
	if len(s.longPages) == 0 {
		return
	}
	for _, page := range s.longPages {
		if page > s.at {
			s.at = page
			return
		}
	}
	s.at = s.longPages[0]
}

func (a *App) measurePhone() {
	s, ed := &a.phone, a.write.ed
	s.prefs = a.phonePreferences()
	n, ok := a.write.tree.Node(a.write.node)
	if !ok || n.Folder || ed == nil {
		return
	}
	title := strings.Replace(n.Title, "：", " ", 1)
	changed := s.doc != ed.Doc || s.version != ed.Doc.Version() || s.measured != s.prefs || s.title != title
	if changed {
		s.anchored = false
		s.doc, s.version, s.measured, s.title = ed.Doc, ed.Doc.Version(), s.prefs, title
		s.layout(ed.Doc.String())
	}
	page := s.pageOf(ed.Caret())
	key := a.path + "/" + a.write.node
	if s.key != key {
		s.key, s.at, s.caretPage = key, 0, page
	} else if s.prefs.Mode == "flip" && s.caretPage != page {
		s.caretPage, s.at = page, page
	}
	s.at = min(max(s.at, 0), s.pages-1)
}

func (s *phoneState) layout(text string) {
	p := s.prefs
	w, h := phoneScreens[p.Screen][0], phoneScreens[p.Screen][1]
	s.textW, s.pageH = w-2*phoneTextInset, h-phoneChromeHeight
	s.paras, s.lines, s.longPages, s.longCount = nil, nil, nil, 0
	used := map[phoneShapeKey]*editor.Paragraph{}
	shape := func(text string, font ui.Font, lh, indent float32) *editor.Paragraph {
		key := phoneShapeKey{text, font, s.textW, lh, indent}
		v := used[key]
		if v == nil {
			v = s.cache[key]
		}
		if v == nil {
			v = editor.LayoutParagraph(text, font, s.textW, lh, indent)
		}
		used[key] = v
		return v
	}
	fs := float32(p.Size)
	lh := float32(lhRead)
	switch p.Spacing {
	case "tight":
		lh = lhLoose
	case "loose":
		lh = lhPaper
	}
	titleSize := phoneTitleEM * fs
	s.titleShape = shape(s.title, ui.Font{Size: titleSize, Weight: 700}, lhTight, 0)
	y, rollY, page := titleSize*phoneTitleTopEM, titleSize*phoneTitleTopEM, 0
	add := func(para int, sh *editor.Paragraph) {
		for i := range sh.Lines {
			if y+sh.LineHeight > s.pageH {
				page++
				y = 0
			}
			s.lines = append(s.lines, phoneLine{para, i, page, y, rollY})
			y += sh.LineHeight
			rollY += sh.LineHeight
		}
	}
	add(-1, s.titleShape)
	y += titleSize * phoneTitleBottomEM
	rollY += titleSize * phoneTitleBottomEM
	start := 0
	for i, text := range strings.Split(text, "\n") {
		sh := shape(text, ui.Font{Size: fs}, lh, phoneIndentEM*fs)
		if y+sh.LineHeight > s.pageH {
			page++
			y = 0
		}
		s.paras = append(s.paras, phonePara{start, sh, rollY})
		if !p.NoLong && len(sh.Lines) > 5 {
			s.longCount++
			if len(s.longPages) == 0 || s.longPages[len(s.longPages)-1] != page {
				s.longPages = append(s.longPages, page)
			}
		}
		add(i, sh)
		y += sh.LineHeight * phoneParaGapEM
		rollY += sh.LineHeight * phoneParaGapEM
		start += len([]rune(text)) + 1
	}
	s.cache, s.fullH, s.pages = used, rollY, page+1
	if p.Mode == "scroll" {
		s.pages = max(1, int(math.Ceil(float64(s.fullH/s.pageH))))
	}
}

func (s *phoneState) pageOf(offset int) int {
	for _, l := range s.lines {
		if l.para < 0 {
			continue
		}
		p := s.paras[l.para]
		ln := p.shape.Lines[l.line]
		end := p.start + ln.End
		if offset < end || offset == end && l.line == len(p.shape.Lines)-1 {
			return l.page
		}
	}
	return max(s.pages-1, 0)
}

func (a *App) phonePanel(c *ui.Context) {
	paneTop(c).Children(func() {
		a.panelTabs(c)
		ui.Spacer(c)
		if iconButton(c, icPanel, "收起右栏", ui.Color{}).Tooltip("收起右栏").Clicked() {
			a.setDocHidden(true)
		}
	})
	if open, _ := a.chapterOpen(); !open {
		a.phone.key = ""
		a.phone.look = false
		a.paneEmpty(c, "打开一章即可预览", "")
		return
	}
	a.measurePhone()
	s := &a.phone
	before := s.prefs
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Padding(0, 20, 12, 20).Shrink(0).Children(func() {
		mode := phoneIndex(s.prefs.Mode, phoneModes)
		a.pal.segments(c, &mode, "看法", []string{"翻页", "滚动"})
		s.prefs.Mode = phoneModes[mode]
		ui.Spacer(c)
		b := ui.ButtonBase(c).Height(hControlSm).Padding(0, 6).Center().Radius(rControl).KeepFocus().Label("Aa").Tooltip("排版")
		if b.Hovered() || s.look {
			b.Background(a.pal.theme.SurfacePressed)
		}
		b.Children(func() { ui.Text(c, "Aa").FontSize(fsControl).FontWeight(600) })
		if b.Clicked() {
			s.look = !s.look
		}
		ui.PopoverBase(c, b, &s.look, func(panel ui.Element) {
			panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(8, 0, 0, 0).Width(phonePopWidth).Column().Padding(4).Gap(2).Radius(rCard).Background(a.pal.raised).Border(1, a.pal.theme.Border)
			a.pal.pop.on(panel)
			a.phoneLook(c)
		})
	})
	if before != s.prefs {
		p := s.prefs
		a.shelf.Phone = &p
		a.saveShelf()
		a.measurePhone()
	}
	a.pal.hairline(c)
	stage := ui.Box(c).FillWidth().Grow(1).Shrink(1).Clip().Label("手机预览正文")
	s.stage = stage.Bounds()
	s.hover = -1
	if x, y, ok := stage.PointerPosition(); ok {
		if para, _, ok := s.hit(x, y); ok {
			s.hover = para
			stage.Cursor(ui.CursorPointer)
			if !s.prefs.NoLong && len(s.paras[para].shape.Lines) > 5 {
				stage.Tooltip(fmt.Sprintf("手机上 %d 行", len(s.paras[para].shape.Lines)))
			}
		}
	}
	stage.HandleInput(a.phoneInput).Draw(a.paintPhone)
	a.pal.hairline(c)
	ui.Row(c).FillWidth().Height(hControl).Padding(0, 20).AlignItems(ui.Center).Justify(ui.SpaceBetween).Shrink(0).Children(func() {
		if s.prefs.Mode == "flip" {
			ui.Box(c).Disabled(s.at == 0).Children(func() {
				if a.pal.ghost(c, "‹ 上一屏", ui.Color{}).Label("‹ 上一屏").KeepFocus().Clicked() {
					s.flip(-1)
				}
			})
			ui.Textf(c, "第 %d 屏，共 %d 屏", s.at+1, s.pages).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).SingleLine()
			ui.Box(c).Disabled(s.at == s.pages-1).Children(func() {
				if a.pal.ghost(c, "下一屏 ›", ui.Color{}).Label("下一屏 ›").KeepFocus().Clicked() {
					s.flip(1)
				}
			})
		} else {
			ui.Spacer(c)
			ui.Textf(c, "约 %d 屏", s.pages).FontSize(fsNote).TextColor(a.pal.theme.TextMuted)
			ui.Spacer(c)
		}
	})
	ui.Row(c).FillWidth().MinHeight(hControl).Padding(0, 20).Justify(ui.End).Shrink(0).Children(func() {
		if s.longCount > 0 {
			text := fmt.Sprintf("%d 段超过 5 行", s.longCount)
			if s.prefs.Mode == "flip" {
				text += "，看下一处"
			}
			if a.pal.ghost(c, text, ui.Color{}).Label("下一处长段").KeepFocus().Clicked() && s.prefs.Mode == "flip" {
				s.nextLong()
			}
		}
	})
}

func (a *App) phoneLook(c *ui.Context) {
	p := &a.phone.prefs
	row := func(label string, value *string, choices, names []string) {
		at := phoneIndex(*value, choices)
		a.pal.popHead(c, label)
		ui.Box(c).FillWidth().Padding(2, 6, 6, 6).Children(func() {
			a.phoneSegments(c, &at, label, names)
		})
		*value = choices[at]
	}
	row("屏幕", &p.Screen, phoneScreenChoices, []string{"窄屏", "标准", "宽屏"})
	a.pal.popHead(c, "字号")
	ui.Box(c).FillWidth().Padding(2, 6, 6, 6).Children(func() {
		ui.Row(c).FillWidth().Padding(2).Radius(rControl).Background(a.pal.theme.SurfacePressed).AlignItems(ui.Center).Children(func() {
			step := func(label, tip string, off bool, d int) {
				b := ui.ButtonBase(c).Grow(1).Height(hSeg).Center().Radius(rXs).Disabled(off).Label(label).Tooltip(tip)
				if b.Hovered() {
					b.Background(a.pal.theme.SurfaceHover)
				}
				if off {
					pale(b)
				}
				b.Children(func() { ui.Text(c, label).FontSize(fsControl) })
				if b.Clicked() {
					p.Size += d
				}
			}
			step("A−", "字号减小", p.Size <= phoneSizeMin, -1)
			ui.Textf(c, "%d", p.Size).Grow(1).TextAlign(ui.Center).FontSize(fsControl)
			step("A+", "字号增大", p.Size >= phoneSizeMax, 1)
		})
	})
	row("行距", &p.Spacing, phoneSpacings, []string{"紧凑", "适中", "宽松"})
	row("背景", &p.Theme, phoneThemes, []string{"白天", "夜间", "护眼"})
	a.pal.popLine(c)
	ui.Column(c).FillWidth().Gap(6).Padding(6, 10).Children(func() {
		frame, long := !p.NoFrame, !p.NoLong
		ui.Checkbox(c, &frame, "显示手机外框")
		ui.Checkbox(c, &long, "标出超过 5 行的段落")
		p.NoFrame, p.NoLong = !frame, !long
	})
	ui.Row(c).FillWidth().Justify(ui.End).Padding(0, 10, 6, 10).Children(func() {
		if a.pal.ghost(c, "恢复默认", ui.Color{}).Clicked() {
			mode := p.Mode
			*p = defaultPhone()
			p.Mode = mode
		}
	})
}

func (a *App) phoneSegments(c *ui.Context, at *int, label string, names []string) {
	t := a.pal.theme
	seg := ui.SegmentedBase(c, at, len(names))
	seg.Track.Label(label).FillWidth().Gap(2).Padding(2).Radius(rControl).Background(t.SurfacePressed).Children(func() {
		for i, name := range names {
			s := seg.Segment(i).Grow(1).Shrink(1).Height(hSeg).Center().Padding(0, 10).Radius(rXs)
			color, weight := t.TextMuted, 400
			if i == *at {
				a.pal.raise.on(s.Background(a.pal.canvas))
				color, weight = t.Text, 600
			}
			s.Children(func() { ui.Text(c, name).FontSize(fsControl).FontWeight(weight).TextColor(color).SingleLine() })
		}
	})
}

func (a *App) phoneInput(ev ui.InputEvent) bool {
	if ev.Kind == ui.InputScroll && a.phone.prefs.Mode == "scroll" && a.phone.scale > 0 {
		s := &a.phone
		s.scrollY = min(max(s.scrollY+ev.DY/s.scale, 0), max(s.fullH-s.pageH, 0))
		return true
	}
	if ev.Kind != ui.InputPointerDown || ev.Button != 0 {
		return false
	}
	if _, offset, ok := a.phone.hit(ev.X, ev.Y); ok {
		a.write.ed.Select(offset, offset)
		a.write.ed.Focus()
		return true
	}
	return false
}

func (s *phoneState) lineY(l phoneLine) float32 {
	if s.prefs.Mode == "scroll" {
		return l.rollY - s.scrollY
	}
	return l.y
}

func (s *phoneState) hit(x, y float32) (para, offset int, ok bool) {
	if s.scale <= 0 {
		return
	}
	x, y = (x-s.ox)/s.scale, (y-s.oy)/s.scale
	if x < s.body.X || x > s.body.X+s.body.W || y < s.body.Y || y > s.body.Y+s.body.H {
		return
	}
	x, y = x-s.body.X, y-s.body.Y
	for _, l := range s.lines {
		if l.para < 0 || s.prefs.Mode == "flip" && l.page != s.at {
			continue
		}
		p := s.paras[l.para]
		ly := s.lineY(l)
		if y >= ly && y < ly+p.shape.LineHeight {
			return l.para, p.start + p.shape.HitAfter(l.line, x), true
		}
	}
	return
}

func (a *App) syncPhone() {
	s := &a.phone
	f, n, into := a.write.ed.ScrollAnchor()
	anchor := phoneAnchor{f, into, n}
	if s.anchored && anchor == s.anchor {
		return
	}
	s.anchor, s.anchored = anchor, true
	most := max(s.fullH-s.pageH, 0)
	switch {
	case f <= 0:
		s.scrollY = 0
	case f >= 1 || n >= len(s.paras):
		s.scrollY = most
	default:
		p := s.paras[n]
		s.scrollY = min(max(p.rollY+into*p.shape.Height-f*s.pageH, 0), most)
	}
}

func (a *App) paintPhone(p *ui.Painter, r ui.Rect) {
	s := &a.phone
	if s.doc == nil {
		return
	}
	if s.prefs.Mode == "scroll" {
		a.syncPhone()
	}
	fs := float32(s.prefs.Size)
	w, h := phoneScreens[s.prefs.Screen][0], phoneScreens[s.prefs.Screen][1]
	pad, radius := float32(0), float32(0)
	if !s.prefs.NoFrame {
		pad = phoneFrameEM * fs
		radius = phoneFrameRadiusEM * fs
	}
	wfull, hfull := w+2*pad+2, h+2*pad+2
	k := max(0, min(1, min(r.W/wfull, r.H/hfull)))
	s.ox, s.oy, s.scale = (r.W-wfull*k)/2, (r.H-hfull*k)/2, k
	s.body = ui.Rect{X: pad + 1 + phoneTextInset, Y: pad + 1 + phoneStatusHeight + phoneTextTop, W: s.textW, H: s.pageH}
	bg, ink, muted, active, hover := a.light.canvas, a.light.ink, a.light.faint, a.light.theme.SurfacePressed, a.light.theme.SurfaceHover
	if s.prefs.Theme == "night" {
		bg, ink, muted, active, hover = a.dark.canvas, a.dark.ink, a.dark.faint, a.dark.theme.SurfacePressed, a.dark.theme.SurfaceHover
	}
	if s.prefs.Theme == "eye" {
		bg = a.light.phoneEye
	}
	s.pick, s.caret = s.pick[:0], ui.Rect{}
	p.Clip(r, 0, func() {
		p.Transform(ui.Scaling(k, k).Then(ui.Translation(r.X+s.ox, r.Y+s.oy)), 0, 0, func() {
			p.Fill(ui.Rect{W: wfull, H: hfull}, a.dark.theme.Border, radius)
			screen := ui.Rect{X: pad + 1, Y: pad + 1, W: w, H: h}
			innerRadius := float32(0)
			if !s.prefs.NoFrame {
				innerRadius = phoneScreenRadiusEM * fs
			}
			p.Clip(screen, innerRadius, func() {
				p.Fill(screen, bg, innerRadius)
				statusSize := phoneStatusSizeEM * fs
				statusMetrics := (ui.Font{Size: statusSize}).Metrics()
				p.Text(screen.X+phoneStatusInset, screen.Y+(phoneStatusHeight-statusMetrics.Ascent-statusMetrics.Descent)/2, time.Now().Format("15:04"), statusSize, ink)
				battery := ui.Rect{X: screen.X + w - phoneStatusInset - fs*phoneBatteryWidthEM, Y: screen.Y + (phoneStatusHeight-fs*phoneBatteryHeightEM)/2, W: fs * phoneBatteryWidthEM, H: fs * phoneBatteryHeightEM}
				p.Stroke(battery, ink, fs*phoneBatteryRadiusEM, 1)
				inset := fs * phoneBatteryInsetEM
				p.Fill(ui.Rect{X: battery.X + inset, Y: battery.Y + inset, W: battery.W*(1-phoneBatteryEmpty) - inset, H: battery.H - 2*inset}, ink, fs*phoneBatteryFillRadiusEM)
				p.Clip(s.body, 0, func() {
					a.paintPhoneText(p, ink, active, hover)
				})
				footSize := phoneFootSizeEM * fs
				foot := ui.Rect{X: screen.X + phoneTextInset, Y: screen.Y + h - phoneFootHeight, W: w - 2*phoneTextInset, H: phoneFootHeight}
				p.Clip(foot, 0, func() {
					p.Text(foot.X, foot.Y+8, clip(s.title, 20), footSize, muted)
				})
				if s.prefs.Mode == "flip" {
					at := fmt.Sprintf("%d/%d", s.at+1, s.pages)
					gs := ui.Shape(at, ui.Font{Size: footSize})
					width := float32(0)
					for _, g := range gs {
						width = max(width, g.X+g.Advance)
					}
					box := ui.Rect{X: screen.X + w - phoneTextInset - width, Y: foot.Y, W: width + 2, H: foot.H}
					p.Fill(box, bg, 0)
					p.Text(box.X, foot.Y+8, at, footSize, muted)
				}
			})
		})
	})
}

func (a *App) paintPhoneText(p *ui.Painter, ink, active, hover ui.Color) {
	s, ed := &a.phone, a.write.ed
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
		if s.hover == previous {
			color = hover
		} else if s.prefs.NoLong || len(s.paras[previous].shape.Lines) <= 5 {
			return
		}
		p.Fill(patch, color, float32(s.prefs.Size)*phoneParaRadiusEM)
	}
	for _, l := range s.lines {
		if l.para < 0 || s.prefs.Mode == "flip" && l.page != s.at {
			continue
		}
		y, h := s.lineY(l), s.paras[l.para].shape.LineHeight
		if y+h <= 0 || y >= s.pageH {
			continue
		}
		if previous != l.para {
			flush()
			previous = l.para
			patch = ui.Rect{X: s.body.X, Y: s.body.Y + y, W: s.textW, H: h}
		} else {
			patch.H = s.body.Y + y + h - patch.Y
		}
	}
	flush()
	for _, l := range s.lines {
		if s.prefs.Mode == "flip" && l.page != s.at {
			continue
		}
		y := s.lineY(l)
		sh := s.titleShape
		if l.para >= 0 {
			sh = s.paras[l.para].shape
		}
		if y+sh.LineHeight <= 0 || y >= s.pageH {
			continue
		}
		x, y := s.body.X, s.body.Y+y
		if l.para >= 0 {
			para := s.paras[l.para]
			ln := sh.Lines[l.line]
			lo, hi := max(from-para.start, ln.Start), min(to-para.start, ln.End)
			if from != to && lo < hi {
				x0, x1 := sh.X(l.line, lo), sh.X(l.line, hi)
				pick := ui.Rect{X: x + x0, Y: y + 2, W: x1 - x0, H: sh.LineHeight - 4}
				s.pick = append(s.pick, pick)
				selection := a.pal.theme.Selection
				if !ed.Focused() {
					selection = a.pal.theme.TextMuted.Alpha(0.2)
				}
				p.Fill(pick, selection, 0)
			}
			at := ed.Caret() - para.start
			if from == to && ed.Focused() && at >= ln.Start && (at < ln.End || at == ln.End && l.line == len(sh.Lines)-1) {
				s.caret = ui.Rect{X: x + sh.X(l.line, at), Y: y, W: float32(s.prefs.Size) * phoneCaretWidthEM, H: sh.LineHeight}
				p.Fill(s.caret, ink, 0)
			}
		}
		sh.DrawLine(p, l.line, x, y, ink)
	}
}
