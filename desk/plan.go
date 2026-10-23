package desk

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

const planFlash = 1600 * time.Millisecond

type planView struct {
	loaded                 bool
	loadErr                error
	checked                time.Time // when the outline was last read
	index                  planIndex
	geometry               *planGeometry
	dragging               string
	mode                   int
	query                  string
	pane                   bool
	outlineID, outlineText string
	outlineHeld            bool
	lineID, lineName       string
	lineHeld, lineFocus    bool
	cardOpen               bool
	card                   workspace.Card
	after, lastTitle       string
	added                  int
	timeOpen               bool
	column                 int
	when                   string
	seen                   string
	seenAt                 time.Time
	reveal                 bool
	scroll                 [2]ui.ScrollState
	viewport               ui.Rect
}

type dragCard string

// Rebuilt once at the start of each outline frame (and after a mutation).
// Each query below then touches only the cards in its chapter or time column.
type planIndex struct {
	chapters []*workspace.Node
	live     map[string]bool
	cards    map[string][]workspace.Card
	back     map[string]bool
	counts   map[string]int
	columns  []workspace.TimeColumn
}

func (a *App) indexPlan() {
	x := planIndex{live: map[string]bool{}, cards: map[string][]workspace.Card{}, back: map[string]bool{}, counts: map[string]int{}, columns: a.book.Plan().Columns()}
	a.book.Tree().Walk(func(n *workspace.Node, _ int) {
		if !n.Folder {
			x.chapters = append(x.chapters, n)
			x.live[n.ID] = true
		}
	})
	first, last := map[string]int{}, map[string]int{}
	for i, e := range a.book.Plan().Cards {
		id := e.Chapter
		if !x.live[id] {
			id = ""
		}
		x.cards[id] = append(x.cards[id], e)
		x.counts[e.Line]++
		if _, ok := first[id]; !ok {
			first[id] = i
		}
		last[id] = i
	}
	before := -1
	for _, n := range x.chapters {
		if i, ok := first[n.ID]; ok {
			x.back[n.ID] = i < before
			before = max(before, last[n.ID])
		}
	}
	a.plan.index = x
}

func (a *App) loadPlan() {
	a.plan.loadErr = a.book.ReloadPlan()
	a.plan.loaded = true
	a.plan.geometry = nil
	a.indexPlan()
}

func (a *App) planChange(err error) {
	a.SetError(err)
	if err == nil {
		a.plan.loadErr = nil
		a.plan.geometry = nil
		a.indexPlan()
	}
}

func (a *App) planChapters() []*workspace.Node {
	return a.plan.index.chapters
}

func (a *App) cardChapter(e workspace.Card) string {
	if a.plan.index.live[e.Chapter] {
		return e.Chapter
	}
	return ""
}

func (a *App) cardsOf(id string) []workspace.Card {
	return a.plan.index.cards[id]
}

func storyWhen(cards []workspace.Card) string {
	var w []string
	for _, e := range cards {
		if e.When != "" && !slices.Contains(w, e.When) {
			w = append(w, e.When)
		}
	}
	if len(w) > 1 {
		return w[0] + " — " + w[len(w)-1]
	}
	if len(w) == 1 {
		return w[0]
	}
	return ""
}

func (a *App) planBack(id string) bool {
	return a.plan.index.back[id]
}

func (a *App) cardFound(e workspace.Card) bool {
	q := trim(a.plan.query)
	if q == "" {
		return true
	}
	title := ""
	if n, ok := a.book.Tree().Node(e.Chapter); ok {
		title = n.Title
	}
	return q == "" || slices.ContainsFunc([]string{e.Title, e.When, e.About, title}, func(s string) bool { return strings.Contains(s, q) })
}

func (a *App) planNodeFound(n *workspace.Node) bool {
	q := trim(a.plan.query)
	return q == "" || strings.Contains(n.Title, q) || strings.Contains(a.book.Outlines()[n.ID], q)
}

func (a *App) planCounts() (cards, nodes int) {
	for _, e := range a.book.Plan().Cards {
		if a.cardFound(e) {
			cards++
		}
	}
	if a.plan.mode == 0 {
		a.book.Tree().Walk(func(n *workspace.Node, _ int) {
			if a.planNodeFound(n) {
				nodes++
			}
		})
	}
	return
}

func (a *App) outlinePage(c *ui.Context) {
	p := &a.plan
	if p.loadErr != nil && time.Since(p.checked) >= checkEvery {
		p.loaded = false // a broken outline may have been put right outside
	}
	if !p.loaded {
		p.checked = time.Now()
		p.loadErr = a.loadTree()
		p.loaded = true
		if p.loadErr == nil {
			a.loadPlan()
		}
		if p.loadErr == nil {
			p.loadErr = a.book.ReloadOutlines()
		}
	} else {
		a.indexPlan()
	}
	if p.loadErr != nil {
		a.planBroken(c)
		return
	}
	if !p.cardOpen && !p.timeOpen && p.outlineID == "" && p.lineID == "" && c.Shortcut(0, ui.KeyEscape) {
		p.query = ""
	}
	if p.seen != "" {
		if wait := planFlash - time.Since(p.seenAt); wait > 0 {
			c.After(wait)
		} else {
			p.seen = ""
		}
	}
	ui.Row(c).FillWidth().MinWidth(0).Grow(1).Shrink(1).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).FillHeight().Children(func() {
			a.planTop(c)
			cards, nodes := a.planCounts()
			if q := trim(p.query); q != "" && cards+nodes == 0 {
				about := "按章节搜的是章名、章纲、卷纲和卡片里的字。"
				if p.mode == 1 {
					about = "时间线上搜的是卡片：发生了什么、说明、故事中的时间和所在的章。"
				}
				a.pal.blank(c, icSearch, "没有找到「"+q+"」", about, func() {
					if a.pal.secondary(c, "清除搜索").Label("空页清除搜索").Clicked() {
						p.query = ""
					}
				})
			} else if p.mode == 0 {
				a.planGrid(c)
				a.planTray(c)
			} else {
				a.planTimeline(c)
			}
		})
		if p.pane {
			a.settingColumn(c)
		}
	})
}

// planBroken stands in for the page while the outline does not read: one
// line, as when a search finds nothing, and no retry, as the file is read
// again as often as the open chapter is looked at on the disk.
func (a *App) planBroken(c *ui.Context) {
	p := &a.plan
	c.After(checkEvery - time.Since(p.checked))
	title, about, why := "大纲读不出来", "", p.loadErr.Error()
	var bad *workspace.PlanError
	if errors.As(p.loadErr, &bad) {
		title, about, why = "大纲文件读不出来", "outline.json 可能在软件外被改过。正文和设定不受影响；改好文件后，这一页会自己重新读取。", ""
		switch {
		case bad.Line > 0:
			why = fmt.Sprintf("第 %d 行附近有错", bad.Line)
		case errors.Is(bad.Err, io.ErrUnexpectedEOF):
			why = "文件在结尾处断了"
		}
	}
	ui.Row(c).FillWidth().Height(titleHeight).Padding(0, 16, 0, 24).AlignItems(ui.Center).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(a.pal.line).Children(func() {
		ui.Text(c, "大纲").FontSize(fsBody).FontWeight(600).SingleLine()
	})
	a.pal.blank(c, icAlert, title, about, func() {
		if why != "" {
			ui.Text(c, why).FontSize(fsNote).TextColor(a.pal.faint).TextAlign(ui.Center).MaxWidth(440)
		}
		if a.Reveal != nil && a.pal.secondary(c, "打开所在文件夹").Clicked() {
			a.warn(a.Reveal(a.Store.Root()))
		}
	})
}

func (a *App) planTop(c *ui.Context) {
	p := &a.plan
	cards, nodes := a.planCounts()
	chs := a.planChapters()
	loose := len(a.cardsOf(""))
	sum := fmt.Sprintf("%d 章 · %d 条情节线 · %d 张卡片", len(chs), len(a.book.Plan().Lines), len(a.book.Plan().Cards))
	if loose > 0 {
		sum += fmt.Sprintf("，%d 张未放进章节", loose)
	}
	if trim(p.query) != "" {
		sum = "没有找到"
		var hits []string
		if nodes > 0 {
			hits = append(hits, fmt.Sprintf("%d 处章名或章纲", nodes))
		}
		if cards > 0 {
			hits = append(hits, fmt.Sprintf("%d 张卡片", cards))
		}
		if len(hits) > 0 {
			sum = "找到 " + strings.Join(hits, "、")
		}
	}
	ui.Row(c).FillWidth().Height(titleHeight).Padding(0, 16, 0, 24).Gap(12).AlignItems(ui.Center).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(a.pal.line).Children(func() {
		ui.Text(c, "大纲").FontSize(fsBody).FontWeight(600).SingleLine()
		ui.Text(c, sum).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
		hint := "搜索卡片和章纲"
		if p.mode == 1 {
			hint = "搜索卡片"
		}
		ui.Row(c).Width(planCardWidth).Height(hControlSm).AlignItems(ui.Center).Gap(6).Padding(0, 8).Radius(rControl).Border(1, a.pal.theme.Border).Children(func() {
			glyph(c, icSearch, iconSm, a.pal.faint)
			ui.TextInputBase(c, &p.query).Label("搜索大纲").Placeholder(hint).FontSize(fsControl).Grow(1).Shrink(1)
			if p.query != "" {
				if iconButton(c, icX, "清除搜索", ui.Color{}).Size(hTag, hTag).Clicked() {
					p.query = ""
				}
			}
		})
		was := p.mode
		a.pal.segments(c, &p.mode, "大纲的看法", []string{"按章节", "时间线"})
		if was != p.mode {
			a.finishPlanOutline()
			a.finishPlanLine()
			p.timeOpen = false
			p.seen = ""
		}
		if !p.pane {
			if iconButton(c, icPanel, "展开右栏", ui.Color{}).Tooltip("展开右栏，查看设定").Clicked() {
				p.pane = true
			}
		}
	})
}

func (a *App) planText(c *ui.Context, text string, small bool) ui.Element {
	size := float32(fsBody)
	if small {
		size = fsNote
	}
	return ui.RichText(c, marked(text, trim(a.plan.query), a.pal.ink)...).FontSize(size).LineHeight(lhBody).FillWidth()
}

func (a *App) planLine(c *ui.Context, l workspace.PlotLine) {
	p := &a.plan
	t := a.pal.theme
	if p.lineID == l.ID {
		in := ui.TextInputBase(c.Key("line-name-"+l.ID), &p.lineName).Label("情节线的名字").Placeholder("情节线的名字").FillWidth().Height(hControlSm).FontSize(fsBody).Padding(0, 6).Border(1, t.Border).Radius(rControl).AutoFocus()
		if p.lineFocus {
			in.Focus().SetTextSelection(0, len([]rune(p.lineName)))
			p.lineFocus = false
		}
		switch {
		case in.Shortcut(0, ui.KeyEscape):
			p.lineID = ""
		case in.Submitted() || p.lineHeld && !in.Focused():
			a.finishPlanLine()
		case in.Focused():
			p.lineHeld = true
		}
		return
	}
	ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
		b := ui.ButtonBase(c).Row().Justify(ui.Start).Grow(1).Shrink(1).MinWidth(0).Padding(2, 0).Radius(rXs).Label(l.Name).Tooltip("点击改名")
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() {
			ui.Text(c, l.Name).FontSize(fsBody).FontWeight(600).Grow(1).Shrink(1).MinWidth(0).SingleLine().Ellipsis("…")
		})
		if b.Clicked() {
			a.startPlanLine(l)
		}
		n := p.index.counts[l.ID]
		ui.Textf(c, "%d", n).FontSize(fsNote).TextColor(a.pal.faint)
		more := iconButton(c, icMore, "情节线「"+l.Name+"」的操作", ui.Color{})
		a.dropMenu(c, more, "plan-line-"+l.ID, func(panel ui.Element) { panel.Width(planLineMenuWidth) }, func(m *menu) {
			i := slices.IndexFunc(a.book.Plan().Lines, func(x workspace.PlotLine) bool { return x.ID == l.ID })
			up, down := "左移", "右移"
			if p.mode == 1 {
				up, down = "上移", "下移"
			}
			m.Item("改名").Do(func() { a.startPlanLine(l) })
			m.Item(up).Disabled(i == 0).Do(func() { a.planChange(a.book.MoveLine(l.ID, -1)) })
			m.Item(down).Disabled(i == len(a.book.Plan().Lines)-1).Do(func() { a.planChange(a.book.MoveLine(l.ID, 1)) })
			m.Separator()
			m.Item("删除情节线").Do(func() {
				a.planChange(a.book.DeleteLine(l.ID))
				if a.errorText() == "" {
					a.status.did("已删除情节线「" + l.Name + "」")
					if n > 0 {
						a.status.hint(fmt.Sprintf("它的 %d 张卡片还在原来的章里，可在时间线上找到", n))
					}
				}
			})
		})
	})
}

func (a *App) startPlanLine(l workspace.PlotLine) {
	a.finishPlanOutline()
	a.finishPlanLine()
	a.plan.lineID, a.plan.lineName, a.plan.lineHeld, a.plan.lineFocus = l.ID, l.Name, false, true
}
func (a *App) finishPlanLine() {
	p := &a.plan
	if p.lineID == "" {
		return
	}
	id, name := p.lineID, trim(p.lineName)
	p.lineID = ""
	if name != "" {
		_, err := a.book.SaveLine(workspace.PlotLine{ID: id, Name: name})
		a.planChange(err)
	}
}
func (a *App) addPlanLine(c *ui.Context) {
	b := ui.ButtonBase(c).Row().Height(hControlSm).AlignItems(ui.Center).Gap(6).Padding(0, 8, 0, 4).Radius(rControl).Label("新增情节线").Tooltip("多一条情节线")
	if b.Hovered() {
		b.Background(a.pal.theme.SurfaceHover)
	}
	b.Children(func() {
		glyph(c, icPlus, iconSm, a.pal.theme.TextMuted)
		ui.Text(c, "新增情节线").FontSize(fsNote).TextColor(a.pal.theme.TextMuted)
	})
	if b.Clicked() {
		a.finishPlanLine()
		l, err := a.book.SaveLine(workspace.PlotLine{Name: "新情节线"})
		a.planChange(err)
		if err == nil {
			a.startPlanLine(l)
			if a.plan.mode == 0 {
				a.plan.scroll[0].X = a.plan.scroll[0].MaxX + planCardWidth
			} else {
				a.plan.scroll[1].Y = a.plan.scroll[1].MaxY + planRowMinHeight
			}
		}
	}
}

func (a *App) planGist(c *ui.Context, n *workspace.Node) {
	p := &a.plan
	label, hint := "章纲", "写章纲…"
	if n.Folder {
		label, hint = "卷纲", "写卷纲…"
	}
	if p.outlineID == n.ID {
		in := ui.TextAreaBase(c.Key("plan-outline-"+n.ID), &p.outlineText).Label(label).Placeholder(hint).FillWidth().Height(hControl*3).FontSize(fsBody).LineHeight(lhBody).Padding(6, 8).Border(1, a.pal.theme.Border).Radius(rControl).AutoFocus()
		if p.outlineText != a.book.Outlines()[n.ID] {
			p.geometry = nil
			a.warn(a.book.SaveOutline(n.ID, p.outlineText))
			if a.errorText() == "" {
				a.syncPlanOutline(n.ID, p.outlineText)
			}
		}
		if in.Shortcut(0, ui.KeyEscape) || p.outlineHeld && !in.Focused() {
			a.finishPlanOutline()
		} else if in.Focused() {
			p.outlineHeld = true
		}
		return
	}
	text := trim(a.book.Outlines()[n.ID])
	empty := text == ""
	if empty {
		text = hint
	}
	b := ui.ButtonBase(c).FillWidth().Padding(2, 4).Radius(rXs).Label(label + "-" + n.ID).Tooltip("点击修改")
	if b.Hovered() {
		b.Background(a.pal.theme.SurfaceHover)
	}
	b.Children(func() {
		e := a.planText(c, text, false).MaxHeight(fsBody * lhBody * 3).Clip()
		if empty {
			e.TextColor(a.pal.faint)
		}
	})
	if b.Clicked() {
		a.finishPlanLine()
		a.finishPlanOutline()
		p.outlineID, p.outlineText, p.outlineHeld = n.ID, a.book.Outlines()[n.ID], false
	}
}
func (a *App) syncPlanOutline(id, text string) {
	if a.write.node == id {
		a.write.cur().outline, a.write.cur().outlineSaved, a.write.cur().outlinePrev = text, text, text
	}
}
func (a *App) finishPlanOutline() {
	p := &a.plan
	if p.outlineID == "" {
		return
	}
	id := p.outlineID
	p.outlineID = ""
	if a.book.Outlines()[id] != p.outlineText {
		a.warn(a.book.SaveOutline(id, p.outlineText))
		if a.errorText() == "" {
			a.syncPlanOutline(id, p.outlineText)
		}
	}
}

// The heading is outside the vertical scroller. Body side cells are
// painted last and translated back by its horizontal offset, so neither
// axis can carry the chapter names or the heading out of view.
func (a *App) planHead(c *ui.Context, left float32, heads func()) {
	p := &a.plan
	height := float32(titleHeight)
	if p.mode == 1 {
		height += hTag
	}
	ui.Row(c).FillWidth().MinWidth(0).Height(height).Shrink(0).Clip().Children(func() {
		word := "章节"
		if p.mode == 1 {
			word = "情节线"
		}
		ui.Column(c).Width(left).FillHeight().Padding(0, 12, 0, 24).Justify(ui.Center).Shrink(0).Background(a.pal.canvas).Children(func() { ui.Text(c, word).FontSize(fsNote).TextColor(a.pal.theme.TextMuted) })
		ui.Box(c).Grow(1).Shrink(1).MinWidth(0).FillHeight().Clip().Children(func() {
			ui.Row(c).Height(height).Translate(-p.scroll[p.mode].X, 0).Children(heads)
		})
		if p.mode == 0 {
			ui.Box(c).Width(planEndWidth).FillHeight().Center().Shrink(0).Background(a.pal.canvas).BorderWidth(0, 0, 0, 1).BorderColor(a.pal.line).Children(func() { a.addPlanLine(c) })
		}
	}).BorderWidth(0, 0, 1, 0).BorderColor(a.pal.theme.Border)
}

func (a *App) planSide(c *ui.Context, left float32, label string, fn func()) {
	p := &a.plan
	bottom := float32(1)
	if p.mode == 1 {
		bottom = 0
	} // the entire lane draws its bottom border
	ui.Column(c).Absolute().Left(0).Top(0).Bottom(0).Width(left).Translate(p.scroll[p.mode].X, 0).Padding(16, 12, 16, 24).Gap(4).Background(a.pal.canvas).BorderWidth(0, 1, bottom, 0).BorderColor(a.pal.line).Label(label).Children(fn)
}

func (a *App) planGrid(c *ui.Context) {
	p := &a.plan
	lines := slices.Clone(a.book.Plan().Lines)
	g := a.planGeometry(c)
	width, height := p.viewport.W, p.viewport.H
	if width <= 0 || height <= 0 {
		width, height = c.Size()
	}
	a.planReveal(g, width, height)
	a.planHead(c, planChapterWidth, func() {
		for _, l := range lines {
			ui.Column(c.Key(l.ID)).Width(planCardWidth).FillHeight().Padding(0, 12).Justify(ui.Center).Shrink(0).Children(func() { a.planLine(c, l) })
		}
	})
	body := ui.ScrollBoth(c.Key("plan-grid")).Grow(1).Shrink(1).FillWidth().TrackScroll(&p.scroll[0])
	p.viewport = body.Bounds()
	if p.viewport.H > 0 {
		height = p.viewport.H
	}
	// Clamp before selecting rows, including when a search shrinks the result.
	p.scroll[0].Y = min(p.scroll[0].Y, max(0, g.height+24-height))
	body.TrackScroll(&p.scroll[0])
	body.Children(func() {
		ui.Column(c).Width(float32(planChapterWidth+planCardWidth*len(lines)+planEndWidth)).Padding(0, 0, 24, 0).Children(func() {
			gap := float32(0)
			held, _ := a.book.Plan().Card(p.dragging)
			for i, row := range g.rows {
				pinned := row.node != nil && row.node.ID == a.cardChapter(held) && p.dragging != ""
				if !pinned && (row.y+row.height < p.scroll[0].Y-height || row.y > p.scroll[0].Y+2*height) {
					gap += row.height
					continue
				}
				if gap > 0 {
					ui.Box(c.Key(fmt.Sprintf("gap-%d", i))).Height(gap).Shrink(0)
					gap = 0
				}
				a.planGridRow(c.Key(fmt.Sprintf("row-%d", i)), row, lines)
			}
			if gap > 0 {
				ui.Box(c.Key("gap-end")).Height(gap).Shrink(0)
			}
			if len(g.rows) == 0 {
				ui.Column(c).Width(planGistWidth).Padding(32, 24).Gap(8).Children(func() {
					ui.Text(c, "章节里没有「"+trim(p.query)+"」").FontSize(fsDialog).FontWeight(600)
					ui.Text(c, "找到的卡片在下面「未放进章节」里。").FontSize(fsNote).TextColor(a.pal.theme.TextMuted)
				})
			}
		})
	})
}

func (a *App) planGridRow(c *ui.Context, row planRow, lines []workspace.PlotLine) {
	p := &a.plan
	n := row.node
	if row.kind == 2 {
		ui.Text(c, "不在卷中").FontSize(fsGroup).FontWeight(600).Padding(16, 24, 12, 24).Height(row.height).Shrink(0).Translate(p.scroll[0].X, 0).FillWidth().Background(a.pal.side)
		return
	}
	if row.kind == 1 {
		ui.Column(c).FillWidth().Height(row.height).Shrink(0).Padding(16, 0, 12, 0).Background(a.pal.side).BorderWidth(0, 0, 1, 0).BorderColor(a.pal.line).Children(func() {
			ui.Column(c).Width(planGistWidth).Margin(0, 0, 0, 24).Translate(p.scroll[0].X, 0).Gap(4).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.RichText(c, marked(n.Title, trim(p.query), a.pal.ink)...).FontSize(fsGroup).FontWeight(600).SingleLine()
					text := fmt.Sprintf("%d 章", row.count)
					if row.when != "" {
						text += " · " + row.when
					}
					ui.Text(c, text).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).SingleLine()
				})
				a.planGist(c, n)
			})
		})
		return
	}
	ui.Row(c).FillWidth().Height(row.height).Shrink(0).AlignItems(ui.Stretch).Children(func() {
		ui.Box(c).Width(planChapterWidth).Shrink(0)
		for _, l := range lines {
			a.planCell(c.Key(n.ID+"|"+l.ID), n.ID, l.ID, -2, "", "")
		}
		ui.Box(c).Grow(1).BorderWidth(0, 0, 1, 0).BorderColor(a.pal.line)
		a.planSide(c, planChapterWidth, "大纲章节-"+n.ID, func() {
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				b := ui.ButtonBase(c).Grow(1).Shrink(1).Radius(rXs).Label("打开正文-" + n.ID).Tooltip("打开正文")
				if b.Hovered() {
					b.Background(a.pal.theme.SurfaceHover)
				}
				b.Children(func() { a.planText(c, n.Title, false).FontWeight(600).SingleLine().Ellipsis("…") })
				if b.Clicked() {
					a.page = pageWrite
					a.openNode(n.ID)
				}
				if a.planBack(n.ID) {
					ui.Text(c, "回溯").FontSize(fsSmall).Padding(2, 4).Radius(rXs).Border(1, a.pal.line).Tooltip("这一章写到了比前面各章更早的事")
				}
			})
			if row.when != "" {
				ui.Text(c, "故事中 "+row.when).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).SingleLine().Ellipsis("…")
			}
			a.planGist(c, n)
			stray := 0
			for _, e := range a.cardsOf(n.ID) {
				if e.Line == "" {
					stray++
				}
			}
			if stray > 0 {
				ui.Textf(c, "另有 %d 张不在任何情节线", stray).FontSize(fsNote).TextColor(a.pal.faint)
			}
		})
	})
}

func (a *App) planCell(c *ui.Context, chapter, line string, col int, when, after string) {
	p := &a.plan
	cell := ui.Column(c).Width(planCardWidth).Shrink(0).Padding(10, 12).Gap(8).BorderWidth(0, 1, 1, 0).BorderColor(a.pal.line).Label(fmt.Sprintf("大纲格-%s|%s|%d", chapter, line, col))
	if col != -2 {
		cell.BorderWidth(0, 1, 0, 0)
	}
	if _, over := ui.DragOver[dragCard](cell); over {
		cell.Background(a.pal.theme.SurfaceHover)
	}
	if id, ok := ui.Drop[dragCard](cell); ok {
		if col == -2 {
			a.planChange(a.book.PlaceCard(string(id), chapter, &line))
		} else {
			a.planChange(a.book.DropCard(string(id), col, line))
		}
	}
	cell.Children(func() {
		var cards []workspace.Card
		if col == -2 {
			for _, e := range a.cardsOf(chapter) {
				if e.Line == line {
					cards = append(cards, e)
				}
			}
		} else if col >= 0 {
			cols := p.index.columns
			if col < len(cols) {
				for _, e := range cols[col].Cards {
					if e.Line == line {
						cards = append(cards, e)
					}
				}
			}
		}
		for _, e := range cards {
			a.planCard(c.Key("card-"+e.ID), e, col != -2, false)
		}
		b := ui.ButtonBase(c).FillWidth().Height(hControl).Center().Gap(6).Row().Border(1, a.pal.theme.Border).BorderStyle(ui.BorderDashed).Radius(rControl).Label(fmt.Sprintf("添加卡片-%s|%s|%d", chapter, line, col)).Tooltip("添加卡片")
		if b.Hovered() {
			b.Background(a.pal.theme.SurfaceHover)
		}
		if !cell.Hovered() && !b.Hovered() {
			b.Opacity(0)
		}
		b.Children(func() {
			glyph(c, icPlus, iconSm, a.pal.theme.TextMuted)
			ui.Text(c, "添加").FontSize(fsNote).TextColor(a.pal.theme.TextMuted)
		})
		if b.Clicked() {
			a.newPlanCard(chapter, line, when, after)
		}
	})
}

func (a *App) planCard(c *ui.Context, e workspace.Card, timeline, tray bool) {
	p := &a.plan
	assigned := a.cardChapter(e) != ""
	b := ui.ButtonBase(c).Column().FillWidth().Padding(6, 10).Gap(2).Radius(rControl).Background(a.pal.raised).Border(1, a.pal.theme.Border).Label("卡片-" + e.ID).Tooltip("点击修改；拖到别的格子挪过去").Drag(dragCard(e.ID))
	if b.Hovered() {
		b.Border(1, a.pal.control).Translate(0, -2)
		a.pal.pop.on(b)
	} else {
		a.pal.raise.on(b)
	}
	if b.Dragging() {
		p.dragging = e.ID
	} else if p.dragging == e.ID {
		p.dragging = ""
	}
	if !assigned {
		b.BorderStyle(ui.BorderDashed).Background(a.pal.canvas)
	}
	if p.seen == e.ID {
		b.Border(1, a.pal.ink)
		a.pal.pop.on(b)
		if p.reveal {
			b.ScrollIntoView()
			p.reveal = false
		}
	}
	if !a.cardFound(e) {
		b.Opacity(opacityDisabled)
	}
	b.Children(func() {
		if e.When != "" && !timeline {
			text := e.When
			if tray {
				for _, l := range a.book.Plan().Lines {
					if l.ID == e.Line {
						text += " · " + l.Name
					}
				}
			}
			a.planText(c, text, true).TextColor(a.pal.theme.TextMuted).SingleLine().Ellipsis("…")
		}
		a.planText(c, e.Title, false).MaxHeight(fsBody * lhBody * 2).Clip()
		if timeline {
			text := "未放进章节"
			if assigned {
				n, _ := a.book.Tree().Node(e.Chapter)
				text = n.Title
			}
			a.planText(c, text, true).TextColor(a.pal.theme.TextMuted).SingleLine().Ellipsis("…")
		}
	})
	if b.Clicked() {
		a.openPlanCard(e)
	}
}

func (a *App) planTray(c *ui.Context) {
	cards := a.cardsOf("")
	tray := ui.Scroll(c.Key("plan-tray")).FillWidth().MaxHeight(planTrayHeight).Shrink(0).Padding(10, 24, 14, 24).Gap(8).Background(a.pal.side).BorderWidth(1, 0, 0, 0).BorderColor(a.pal.theme.Border).Label("大纲托盘")
	if _, over := ui.DragOver[dragCard](tray); over {
		tray.Background(a.pal.theme.SurfaceHover)
	}
	if id, ok := ui.Drop[dragCard](tray); ok {
		a.planChange(a.book.PlaceCard(string(id), "", nil))
	}
	tray.Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "未放进章节").FontSize(fsGroup).FontWeight(600)
			note := "想到了、还没定写在哪的事，先记在这里"
			if len(cards) > 0 {
				note = fmt.Sprintf("%d 张 · 拖进上面的格子就放进那一章", len(cards))
			}
			ui.Text(c, note).FontSize(fsNote).TextColor(a.pal.theme.TextMuted).Grow(1).Shrink(1)
			if a.pal.ghost(c, "记一张", ui.Color{}).Clicked() {
				a.newPlanCard("", "", "", "")
			}
		})
		if len(cards) > 0 {
			ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Start).Children(func() {
				for _, e := range cards {
					ui.Box(c.Key(e.ID)).Width(planCardWidth).Children(func() { a.planCard(c, e, false, true) })
				}
			})
		}
	})
}

func (a *App) planTimeline(c *ui.Context) {
	p := &a.plan
	cols := p.index.columns
	g := a.planGeometry(c)
	width, height := p.viewport.W, p.viewport.H
	if width <= 0 || height <= 0 {
		width, height = c.Size()
	}
	a.planReveal(g, width, height)
	viewWidth := max(planCardWidth, width-planLaneWidth)
	p.scroll[1].X = min(p.scroll[1].X, max(0, float32(planLaneWidth+planCardWidth*(len(cols)+1))-width))
	first := max(0, int((p.scroll[1].X-viewWidth)/planCardWidth))
	end := min(len(cols)+1, int((p.scroll[1].X+2*viewWidth)/planCardWidth)+1)
	held := -1
	if p.dragging != "" {
		for i, col := range cols {
			if slices.ContainsFunc(col.Cards, func(e workspace.Card) bool { return e.ID == p.dragging }) {
				held = i
				break
			}
		}
	}
	lanes := slices.Clone(a.book.Plan().Lines)
	if slices.ContainsFunc(a.book.Plan().Cards, func(e workspace.Card) bool { return e.Line == "" }) {
		lanes = append(lanes, workspace.PlotLine{Name: "不分情节线"})
	}
	ui.Row(c).FillWidth().Padding(10, 24, 0, 24).Gap(16).Shrink(0).Children(func() {
		for _, loose := range []bool{false, true} {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
				key := ui.Box(c).Size(iconMd, rCard).Radius(rXs).Border(1, a.pal.control).Background(a.pal.raised)
				text := "已放进章节"
				if loose {
					key.BorderStyle(ui.BorderDashed).Background(a.pal.canvas)
					text = "未放进章节"
				}
				ui.Text(c, text).FontSize(fsNote).TextColor(a.pal.theme.TextMuted)
			})
		}
		ui.Text(c, "时间可以不写，左右的先后就是故事里的先后").FontSize(fsNote).TextColor(a.pal.faint).Grow(1).Shrink(1)
		var off []string
		for _, n := range a.planChapters() {
			if len(a.cardsOf(n.ID)) == 0 {
				off = append(off, n.Title)
			}
		}
		if len(off) > 0 {
			word := fmt.Sprintf("%d 章", len(off))
			if len(off) == 1 {
				word = "「" + off[0] + "」"
			}
			ui.Text(c, word+"还没有卡片").FontSize(fsNote).TextColor(a.pal.theme.TextMuted).SingleLine()
		}
	})
	a.planHead(c, planLaneWidth, func() {
		planColumns(c, len(cols)+1, first, end, held, func(i int) {
			if i < len(cols) {
				a.planTimeHead(c.Key(i), i, cols[i])
			} else {
				a.planTimeHead(c.Key("end"), -1, workspace.TimeColumn{When: "之后"})
			}
		})
	})
	body := ui.ScrollBoth(c.Key("plan-timeline")).Grow(1).Shrink(1).FillWidth().TrackScroll(&p.scroll[1])
	p.viewport = body.Bounds()
	body.Children(func() {
		ui.Column(c).Width(max(width, float32(planLaneWidth+planCardWidth*(len(cols)+1)))).Padding(0, 0, 24, 0).Children(func() {
			for _, l := range lanes {
				ui.Row(c.Key("lane-"+l.ID)).FillWidth().Height(max(planLaneMinHeight, g.lanes[l.ID])).Shrink(0).AlignItems(ui.Stretch).BorderWidth(0, 0, 1, 0).BorderColor(a.pal.line).Children(func() {
					ui.Box(c).Width(planLaneWidth).Shrink(0)
					planColumns(c, len(cols)+1, first, end, held, func(i int) {
						if i < len(cols) {
							col := cols[i]
							a.planCell(c.Key(fmt.Sprintf("%d|%s", i, l.ID)), "", l.ID, i, col.When, col.Cards[len(col.Cards)-1].ID)
						} else {
							after := ""
							if len(a.book.Plan().Cards) > 0 {
								after = a.book.Plan().Cards[len(a.book.Plan().Cards)-1].ID
							}
							a.planCell(c.Key("end-"+l.ID), "", l.ID, -1, "", after)
						}
					})
					a.planSide(c, planLaneWidth, "大纲泳道-"+l.ID, func() {
						if l.ID != "" {
							a.planLine(c, l)
						} else {
							ui.Text(c, l.Name).FontSize(fsBody).FontWeight(600)
						}
					})
				})
			}
			ui.Row(c).FillWidth().Height(titleHeight).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(a.pal.line).Label("新增情节线整行").Children(func() { a.planSide(c, planLaneWidth, "新增情节线行", func() { a.addPlanLine(c) }) })
		})
	})
}

func (a *App) planTimeHead(c *ui.Context, i int, col workspace.TimeColumn) {
	p := &a.plan
	ui.Column(c).Width(planCardWidth).Height(titleHeight+hTag).Padding(12, 12, 0, 12).Gap(6).Shrink(0).Children(func() {
		label := col.When
		if label == "" {
			label = "未写时间"
		}
		b := ui.ButtonBase(c).FillWidth().Padding(2, 6).Radius(rXs).Label(fmt.Sprintf("时间列-%d", i)).Tooltip("点击修改这一列的时间").Disabled(i < 0)
		if b.Hovered() {
			b.Background(a.pal.theme.SurfaceHover)
		}
		b.Children(func() { a.planText(c, label, false).FontWeight(600).SingleLine().Ellipsis("…") })
		if b.Clicked() {
			a.finishPlanOutline()
			a.finishPlanLine()
			p.column, p.when, p.timeOpen = i, col.When, true
		}
		ui.Box(c).FillWidth().Height(hTag).Draw(func(painter *ui.Painter, r ui.Rect) {
			end := r.X + r.W + 12
			if i < 0 {
				end = r.X + r.W/2
			}
			start := r.X - 12
			if i == 0 {
				start = r.X
			}
			painter.Line(start, r.Y+r.H/2, end, r.Y+r.H/2, 1, a.pal.theme.Border)
			if i >= 0 {
				assigned := slices.ContainsFunc(col.Cards, func(e workspace.Card) bool { return a.cardChapter(e) != "" })
				color := a.pal.ink
				if !assigned {
					color = a.pal.control
				}
				dot := ui.Rect{X: r.X, Y: r.Y + (r.H-rCard)/2, W: rCard, H: rCard}
				painter.Fill(dot, color, rRound)
				if !assigned {
					dot.X += 2
					dot.Y += 2
					dot.W -= 4
					dot.H -= 4
					painter.Fill(dot, a.pal.canvas, rRound)
				}
			}
		})
		open := p.timeOpen && p.column == i
		ui.PopoverBase(c, b, &open, func(panel ui.Element) {
			panel.Width(planTimePopWidth).Padding(10).Gap(8).Radius(rCard).Background(a.pal.raised).Border(1, a.pal.theme.Border)
			a.pal.pop.on(panel)
			ui.Textf(c, "这一列 %d 张卡片在故事中的时间", len(col.Cards)).FontSize(fsNote).TextColor(a.pal.theme.TextMuted)
			in := ui.TextInput(c, &p.when).Label("这一列的时间").Placeholder("例如：十月六日夜").FillWidth().AutoFocus()
			save := in.Submitted()
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if a.pal.secondary(c, "取消").Label("取消时间").Clicked() {
					open = false
				}
				if a.pal.primary(c, "保存").Label("保存时间").Clicked() {
					save = true
				}
			})
			if save {
				a.planChange(a.book.SaveTime(p.column, p.when))
				if a.errorText() == "" {
					open = false
				}
			}
		})
		if p.column == i {
			p.timeOpen = open
		}
	})
}

func (a *App) newPlanCard(chapter, line, when, after string) {
	a.finishPlanOutline()
	a.finishPlanLine()
	p := &a.plan
	p.cardOpen, p.card, p.after, p.added, p.lastTitle = true, workspace.Card{Chapter: chapter, Line: line, When: when}, after, 0, ""
}
func (a *App) openPlanCard(e workspace.Card) { a.newPlanCard("", "", "", ""); a.plan.card = e }
func (a *App) seePlanCard(id string)         { p := &a.plan; p.seen, p.seenAt, p.reveal = id, time.Now(), true }
func (a *App) savePlanCard(more bool) {
	p := &a.plan
	if trim(p.card.Title) == "" {
		return
	}
	fresh := p.card.ID == ""
	e, err := a.book.SaveCard(p.card, p.after)
	a.planChange(err)
	if err != nil {
		return
	}
	if fresh {
		a.seePlanCard(e.ID)
	}
	if more && fresh {
		p.card.Title, p.card.About = "", ""
		p.card.When = e.When
		p.after = e.ID
		p.lastTitle = e.Title
		p.added++
	} else {
		p.cardOpen = false
	}
}

func (a *App) planDialog(c *ui.Context) {
	p := &a.plan
	if !p.cardOpen {
		return
	}
	a.modal(c, &p.cardOpen, func() {
		ui.Column(c).Width(dialogSmall).Padding(dialogPad).Gap(16).Children(func() {
			title, about := "新卡片", "一张卡片记一件事。只有「发生了什么」要写，别的都可以以后再补。"
			if p.card.ID != "" {
				title, about = "修改卡片", ""
			} else if p.added > 0 {
				about = "已添加「" + p.lastTitle + "」，接着记下一张。"
			}
			a.pal.dialogHead(c, title, about)
			var enter bool
			labeled(c, "发生了什么", func() {
				in := ui.TextInput(c.Key(fmt.Sprintf("card-title-%d", p.added)), &p.card.Title).Label("事件").Placeholder("一句话，例如：林晚下井").FillWidth().AutoFocus()
				enter = in.Submitted()
			})
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).Children(func() {
					labeled(c, "写在哪一章", func() {
						names, ids := []string{"未放进章节"}, map[string]string{}
						chosen := names[0]
						for _, n := range a.planChapters() {
							name := n.Title
							for count := 2; ids[name] != ""; count++ {
								name = fmt.Sprintf("%s (%d)", n.Title, count)
							}
							names = append(names, name)
							ids[name] = n.ID
							if n.ID == p.card.Chapter {
								chosen = name
							}
						}
						if ui.Select(c, &chosen, names).Label("卡片章节").FillWidth().MinWidth(0).Changed() {
							p.card.Chapter = ids[chosen]
						}
					})
				})
				ui.Column(c).Grow(1).Shrink(1).Children(func() {
					labeled(c, "情节线", func() {
						names, ids := []string{"不分情节线"}, map[string]string{}
						chosen := names[0]
						for _, l := range a.book.Plan().Lines {
							name := l.Name
							for count := 2; ids[name] != ""; count++ {
								name = fmt.Sprintf("%s (%d)", l.Name, count)
							}
							names = append(names, name)
							ids[name] = l.ID
							if l.ID == p.card.Line {
								chosen = name
							}
						}
						if ui.Select(c, &chosen, names).Label("卡片情节线").FillWidth().MinWidth(0).Changed() {
							p.card.Line = ids[chosen]
						}
					})
				})
			})
			labeled(c, "故事中的时间（选填）", func() {
				in := ui.TextInput(c, &p.card.When).Label("故事中的时间").Placeholder("例如：十月六日夜。不写也行，按先后排").FillWidth()
				enter = enter || in.Submitted()
			})
			labeled(c, "说明（选填）", func() { ui.TextArea(c, &p.card.About).Label("说明").FillWidth().Height(hControl * 3) })
			save, more, drop := enter && trim(p.card.Title) != "", false, false
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if p.card.ID != "" {
					if a.pal.ghost(c, "删除卡片", a.pal.theme.Danger).Clicked() {
						drop = true
					}
					ui.Spacer(c)
				}
				if a.pal.secondary(c, "取消").Label("取消卡片").Clicked() {
					p.cardOpen = false
				}
				if p.card.ID == "" {
					if a.pal.secondary(c, "继续添加").Disabled(trim(p.card.Title) == "").Clicked() {
						more = true
					}
				}
				verb := "添加"
				if p.card.ID != "" {
					verb = "保存"
				}
				if a.pal.primary(c, verb).Label("保存卡片").Disabled(trim(p.card.Title) == "").Clicked() {
					save = true
				}
			})
			if drop {
				a.planChange(a.book.DeleteCard(p.card.ID))
				if a.errorText() == "" {
					a.status.did("已删除卡片「" + p.card.Title + "」")
					p.cardOpen = false
				}
			} else if save || more {
				a.savePlanCard(more)
			}
		})
	})
}

func (a *App) chapterStory(c *ui.Context) {
	if !a.plan.loaded {
		a.loadPlan()
	} else {
		a.indexPlan()
	}
	if a.plan.loadErr != nil {
		return
	}
	cards := a.cardsOf(a.write.node)
	when := storyWhen(cards)
	if when == "" {
		return
	}
	if a.pal.ghost(c, "故事中 "+when, ui.Color{}).Clicked() {
		a.page = pagePlan
		a.plan.mode = 1
		a.plan.query = ""
		a.plan.loaded = false
		for _, e := range cards {
			if e.When != "" {
				a.seePlanCard(e.ID)
				break
			}
		}
	}
}
