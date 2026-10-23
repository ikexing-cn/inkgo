package desk

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/workspace"
)

const planFlash = 1600 * time.Millisecond

type outlinePage struct {
	host                host
	loaded              bool
	loadErr             error
	checked             time.Time // when the outline was last read
	index               planIndex
	geometry            *planGeometry
	dragging            string
	mode                int
	query               string
	pane                bool
	outlineDraft        outlineDraft
	outlineHeld         bool
	lineID, lineName    string
	lineHeld, lineFocus bool
	cardOpen            bool
	card                workspace.Card
	after, lastTitle    string
	added               int
	timeOpen            bool
	column              int
	when                string
	seen                string
	seenAt              time.Time
	reveal              bool
	scroll              [2]ui.ScrollState
	viewport            ui.Rect
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

func (o *outlinePage) rebuildIndex() {
	content := o.host.content()
	x := planIndex{live: map[string]bool{}, cards: map[string][]workspace.Card{}, back: map[string]bool{}, counts: map[string]int{}, columns: content.Plan().Columns()}
	content.Tree().Walk(func(n *workspace.Node, _ int) {
		if !n.Folder {
			x.chapters = append(x.chapters, n)
			x.live[n.ID] = true
		}
	})
	first, last := map[string]int{}, map[string]int{}
	for i, e := range content.Plan().Cards {
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
	o.index = x
}

func (o *outlinePage) load() {
	o.loadErr = o.host.content().ReloadPlan()
	o.loaded = true
	o.geometry = nil
	o.rebuildIndex()
}

func (o *outlinePage) change(err error) {
	o.host.notices().SetError(err)
	if err == nil {
		o.loadErr = nil
		o.geometry = nil
		o.rebuildIndex()
	}
}

func (o *outlinePage) chapters() []*workspace.Node {
	return o.index.chapters
}

func (o *outlinePage) cardChapter(e workspace.Card) string {
	if o.index.live[e.Chapter] {
		return e.Chapter
	}
	return ""
}

func (o *outlinePage) cardsOf(id string) []workspace.Card {
	return o.index.cards[id]
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

func (o *outlinePage) back(id string) bool {
	return o.index.back[id]
}

func (o *outlinePage) cardFound(e workspace.Card) bool {
	q := kit.Trim(o.query)
	if q == "" {
		return true
	}
	title := ""
	if n, ok := o.host.content().Tree().Node(e.Chapter); ok {
		title = n.Title
	}
	return q == "" || slices.ContainsFunc([]string{e.Title, e.When, e.About, title}, func(s string) bool { return strings.Contains(s, q) })
}

func (o *outlinePage) nodeFound(n *workspace.Node) bool {
	q := kit.Trim(o.query)
	return q == "" || strings.Contains(n.Title, q) || strings.Contains(o.host.content().Outlines()[n.ID], q)
}

func (o *outlinePage) counts() (cards, nodes int) {
	content := o.host.content()
	for _, e := range content.Plan().Cards {
		if o.cardFound(e) {
			cards++
		}
	}
	if o.mode == 0 {
		content.Tree().Walk(func(n *workspace.Node, _ int) {
			if o.nodeFound(n) {
				nodes++
			}
		})
	}
	return
}

func (o *outlinePage) view(c *ui.Context) {
	content := o.host.content()
	if o.loadErr != nil && time.Since(o.checked) >= checkEvery {
		o.loaded = false // a broken outline may have been put right outside
	}
	if !o.loaded {
		o.checked = time.Now()
		o.loadErr = content.ReloadTree()
		o.loaded = true
		if o.loadErr == nil {
			o.load()
		}
		if o.loadErr == nil {
			o.loadErr = content.ReloadOutlines()
		}
	} else {
		o.rebuildIndex()
	}
	if o.loadErr != nil {
		o.broken(c)
		return
	}
	if !o.cardOpen && !o.timeOpen && o.outlineDraft.node == "" && o.lineID == "" && c.Shortcut(0, ui.KeyEscape) {
		o.query = ""
	}
	if o.seen != "" {
		if wait := planFlash - time.Since(o.seenAt); wait > 0 {
			c.After(wait)
		} else {
			o.seen = ""
		}
	}
	ui.Row(c).FillWidth().MinWidth(0).Grow(1).Shrink(1).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).FillHeight().Children(func() {
			o.top(c)
			cards, nodes := o.counts()
			if q := kit.Trim(o.query); q != "" && cards+nodes == 0 {
				about := "按章节搜的是章名、章纲、卷纲和卡片里的字。"
				if o.mode == 1 {
					about = "时间线上搜的是卡片：发生了什么、说明、故事中的时间和所在的章。"
				}
				o.host.colors().Blank(c, kit.IcSearch, "没有找到「"+q+"」", about, func() {
					if o.host.colors().Secondary(c, "清除搜索").Label("空页清除搜索").Clicked() {
						o.query = ""
					}
				})
			} else if o.mode == 0 {
				o.grid(c)
				o.tray(c)
			} else {
				o.timeline(c)
			}
		})
		if o.pane {
			o.host.settingColumn(c)
		}
	})
}

// broken stands in for the page while the outline does not read: one
// line, as when a search finds nothing, and no retry, as the file is read
// again as often as the open chapter is looked at on the disk.
func (o *outlinePage) broken(c *ui.Context) {
	pal := o.host.colors()
	c.After(checkEvery - time.Since(o.checked))
	title, about, why := "大纲读不出来", "", o.loadErr.Error()
	var bad *workspace.PlanError
	if errors.As(o.loadErr, &bad) {
		title, about, why = "大纲文件读不出来", "outline.json 可能在软件外被改过。正文和设定不受影响；改好文件后，这一页会自己重新读取。", ""
		switch {
		case bad.Line > 0:
			why = fmt.Sprintf("第 %d 行附近有错", bad.Line)
		case errors.Is(bad.Err, io.ErrUnexpectedEOF):
			why = "文件在结尾处断了"
		}
	}
	ui.Row(c).FillWidth().Height(kit.TitleHeight).Padding(0, 16, 0, 24).AlignItems(ui.Center).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(pal.Line).Children(func() {
		ui.Text(c, "大纲").FontSize(kit.FsBody).FontWeight(600).SingleLine()
	})
	pal.Blank(c, kit.IcAlert, title, about, func() {
		if why != "" {
			ui.Text(c, why).FontSize(kit.FsNote).TextColor(pal.Faint).TextAlign(ui.Center).MaxWidth(440)
		}
		_, reveal := o.host.dialogs()
		if reveal != nil && pal.Secondary(c, "打开所在文件夹").Clicked() {
			store, _ := o.host.workspace()
			o.host.notices().Warn(reveal(store.Root()))
		}
	})
}

func (o *outlinePage) top(c *ui.Context) {
	pal := o.host.colors()
	content := o.host.content()
	cards, nodes := o.counts()
	chs := o.chapters()
	loose := len(o.cardsOf(""))
	sum := fmt.Sprintf("%d 章 · %d 条情节线 · %d 张卡片", len(chs), len(content.Plan().Lines), len(content.Plan().Cards))
	if loose > 0 {
		sum += fmt.Sprintf("，%d 张未放进章节", loose)
	}
	if kit.Trim(o.query) != "" {
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
	ui.Row(c).FillWidth().Height(kit.TitleHeight).Padding(0, 16, 0, 24).Gap(12).AlignItems(ui.Center).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(pal.Line).Children(func() {
		ui.Text(c, "大纲").FontSize(kit.FsBody).FontWeight(600).SingleLine()
		ui.Text(c, sum).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
		hint := "搜索卡片和章纲"
		if o.mode == 1 {
			hint = "搜索卡片"
		}
		ui.Row(c).Width(planCardWidth).Height(kit.HControlSm).AlignItems(ui.Center).Gap(6).Padding(0, 8).Radius(kit.RControl).Border(1, pal.Theme.Border).Children(func() {
			kit.Glyph(c, kit.IcSearch, kit.IconSm, pal.Faint)
			ui.TextInputBase(c, &o.query).Label("搜索大纲").Placeholder(hint).FontSize(kit.FsControl).Grow(1).Shrink(1)
			if o.query != "" {
				if kit.IconButton(c, kit.IcX, "清除搜索", ui.Color{}).Size(kit.HTag, kit.HTag).Clicked() {
					o.query = ""
				}
			}
		})
		was := o.mode
		pal.Segments(c, &o.mode, "大纲的看法", []string{"按章节", "时间线"})
		if was != o.mode {
			o.finishOutline()
			o.finishLine()
			o.timeOpen = false
			o.seen = ""
		}
		if !o.pane {
			if kit.IconButton(c, kit.IcPanel, "展开右栏", ui.Color{}).Tooltip("展开右栏，查看设定").Clicked() {
				o.pane = true
			}
		}
	})
}

func (o *outlinePage) text(c *ui.Context, text string, small bool) ui.Element {
	size := float32(kit.FsBody)
	if small {
		size = kit.FsNote
	}
	return ui.RichText(c, kit.Marked(text, kit.Trim(o.query), o.host.colors().Ink)...).FontSize(size).LineHeight(kit.LhBody).FillWidth()
}

func (o *outlinePage) line(c *ui.Context, l workspace.PlotLine) {
	status := o.host.notices()
	content := o.host.content()
	t := o.host.colors().Theme
	if o.lineID == l.ID {
		in := ui.TextInputBase(c.Key("line-name-"+l.ID), &o.lineName).Label("情节线的名字").Placeholder("情节线的名字").FillWidth().Height(kit.HControlSm).FontSize(kit.FsBody).Padding(0, 6).Border(1, t.Border).Radius(kit.RControl).AutoFocus()
		if o.lineFocus {
			in.Focus().SetTextSelection(0, len([]rune(o.lineName)))
			o.lineFocus = false
		}
		switch {
		case in.Shortcut(0, ui.KeyEscape):
			o.lineID = ""
		case in.Submitted() || o.lineHeld && !in.Focused():
			o.finishLine()
		case in.Focused():
			o.lineHeld = true
		}
		return
	}
	ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
		b := ui.ButtonBase(c).Row().Justify(ui.Start).Grow(1).Shrink(1).MinWidth(0).Padding(2, 0).Radius(kit.RXs).Label(l.Name).Tooltip("点击改名")
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() {
			ui.Text(c, l.Name).FontSize(kit.FsBody).FontWeight(600).Grow(1).Shrink(1).MinWidth(0).SingleLine().Ellipsis("…")
		})
		if b.Clicked() {
			o.startLine(l)
		}
		n := o.index.counts[l.ID]
		ui.Textf(c, "%d", n).FontSize(kit.FsNote).TextColor(o.host.colors().Faint)
		more := kit.IconButton(c, kit.IcMore, "情节线「"+l.Name+"」的操作", ui.Color{})
		o.host.dropMenu(c, more, "plan-line-"+l.ID, func(panel ui.Element) { panel.Width(planLineMenuWidth) }, func(m *kit.Menu) {
			i := slices.IndexFunc(content.Plan().Lines, func(x workspace.PlotLine) bool { return x.ID == l.ID })
			up, down := "左移", "右移"
			if o.mode == 1 {
				up, down = "上移", "下移"
			}
			m.Item("改名").Do(func() { o.startLine(l) })
			m.Item(up).Disabled(i == 0).Do(func() { o.change(content.MoveLine(l.ID, -1)) })
			m.Item(down).Disabled(i == len(content.Plan().Lines)-1).Do(func() { o.change(content.MoveLine(l.ID, 1)) })
			m.Separator()
			m.Item("删除情节线").Do(func() {
				o.change(content.DeleteLine(l.ID))
				if status.ErrorText() == "" {
					status.Did("已删除情节线「" + l.Name + "」")
					if n > 0 {
						status.Hint(fmt.Sprintf("它的 %d 张卡片还在原来的章里，可在时间线上找到", n))
					}
				}
			})
		})
	})
}

func (o *outlinePage) startLine(l workspace.PlotLine) {
	o.finishOutline()
	o.finishLine()
	o.lineID, o.lineName, o.lineHeld, o.lineFocus = l.ID, l.Name, false, true
}
func (o *outlinePage) finishLine() {
	if o.lineID == "" {
		return
	}
	id, name := o.lineID, kit.Trim(o.lineName)
	o.lineID = ""
	if name != "" {
		_, err := o.host.content().SaveLine(workspace.PlotLine{ID: id, Name: name})
		o.change(err)
	}
}
func (o *outlinePage) addLine(c *ui.Context) {
	pal := o.host.colors()
	b := ui.ButtonBase(c).Row().Height(kit.HControlSm).AlignItems(ui.Center).Gap(6).Padding(0, 8, 0, 4).Radius(kit.RControl).Label("新增情节线").Tooltip("多一条情节线")
	if b.Hovered() {
		b.Background(pal.Theme.SurfaceHover)
	}
	b.Children(func() {
		kit.Glyph(c, kit.IcPlus, kit.IconSm, pal.Theme.TextMuted)
		ui.Text(c, "新增情节线").FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted)
	})
	if b.Clicked() {
		o.finishLine()
		l, err := o.host.content().SaveLine(workspace.PlotLine{Name: "新情节线"})
		o.change(err)
		if err == nil {
			o.startLine(l)
			if o.mode == 0 {
				o.scroll[0].X = o.scroll[0].MaxX + planCardWidth
			} else {
				o.scroll[1].Y = o.scroll[1].MaxY + planRowMinHeight
			}
		}
	}
}

func (o *outlinePage) gist(c *ui.Context, n *workspace.Node) {
	pal := o.host.colors()
	content := o.host.content()
	label, hint := "章纲", "写章纲…"
	if n.Folder {
		label, hint = "卷纲", "写卷纲…"
	}
	if o.outlineDraft.node == n.ID {
		in := ui.TextAreaBase(c.Key("plan-outline-"+n.ID), &o.outlineDraft.text).Label(label).Placeholder(hint).FillWidth().Height(kit.HControl*3).FontSize(kit.FsBody).LineHeight(kit.LhBody).Padding(6, 8).Border(1, pal.Theme.Border).Radius(kit.RControl).AutoFocus()
		now := time.Now()
		if o.outlineDraft.change(now) {
			o.geometry = nil
		}
		if wait, due := o.outlineDraft.waits(saveAfter, now); due {
			if wait > 0 {
				c.After(wait)
			} else {
				o.saveOutline()
			}
		}
		if in.Shortcut(0, ui.KeyEscape) || o.outlineHeld && !in.Focused() {
			o.finishOutline()
		} else if in.Focused() {
			o.outlineHeld = true
		}
		return
	}
	text := kit.Trim(content.Outlines()[n.ID])
	empty := text == ""
	if empty {
		text = hint
	}
	b := ui.ButtonBase(c).FillWidth().Padding(2, 4).Radius(kit.RXs).Label(label + "-" + n.ID).Tooltip("点击修改")
	if b.Hovered() {
		b.Background(pal.Theme.SurfaceHover)
	}
	b.Children(func() {
		e := o.text(c, text, false).MaxHeight(kit.FsBody * kit.LhBody * 3).Clip()
		if empty {
			e.TextColor(pal.Faint)
		}
	})
	if b.Clicked() {
		o.finishLine()
		o.finishOutline()
		out := content.Outlines()[n.ID]
		o.outlineDraft = outlineDraft{node: n.ID, text: out, saved: out, seen: out}
		o.outlineHeld = false
	}
}

// saveOutline saves the outline page's draft and shows it on the
// writing page when it is the chapter shown.
func (o *outlinePage) saveOutline() {
	d := &o.outlineDraft
	if d.node == "" {
		return
	}
	if err := d.save(o.host.content()); err != nil {
		o.host.notices().Warn(err)
		return
	}
	o.host.syncPlanOutline(d.node, d.text)
}

func (o *outlinePage) finishOutline() {
	if o.outlineDraft.node == "" {
		return
	}
	o.saveOutline()
	o.outlineDraft = outlineDraft{}
	o.outlineHeld = false
}

// The heading is outside the vertical scroller. Body side cells are
// painted last and translated back by its horizontal offset, so neither
// axis can carry the chapter names or the heading out of view.
func (o *outlinePage) head(c *ui.Context, left float32, heads func()) {
	pal := o.host.colors()
	height := float32(kit.TitleHeight)
	if o.mode == 1 {
		height += kit.HTag
	}
	ui.Row(c).FillWidth().MinWidth(0).Height(height).Shrink(0).Clip().Children(func() {
		word := "章节"
		if o.mode == 1 {
			word = "情节线"
		}
		ui.Column(c).Width(left).FillHeight().Padding(0, 12, 0, 24).Justify(ui.Center).Shrink(0).Background(pal.Canvas).Children(func() { ui.Text(c, word).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted) })
		ui.Box(c).Grow(1).Shrink(1).MinWidth(0).FillHeight().Clip().Children(func() {
			ui.Row(c).Height(height).Translate(-o.scroll[o.mode].X, 0).Children(heads)
		})
		if o.mode == 0 {
			ui.Box(c).Width(planEndWidth).FillHeight().Center().Shrink(0).Background(pal.Canvas).BorderWidth(0, 0, 0, 1).BorderColor(pal.Line).Children(func() { o.addLine(c) })
		}
	}).BorderWidth(0, 0, 1, 0).BorderColor(pal.Theme.Border)
}

func (o *outlinePage) side(c *ui.Context, left float32, label string, fn func()) {
	bottom := float32(1)
	if o.mode == 1 {
		bottom = 0
	} // the entire lane draws its bottom border
	ui.Column(c).Absolute().Left(0).Top(0).Bottom(0).Width(left).Translate(o.scroll[o.mode].X, 0).Padding(16, 12, 16, 24).Gap(4).Background(o.host.colors().Canvas).BorderWidth(0, 1, bottom, 0).BorderColor(o.host.colors().Line).Label(label).Children(fn)
}

func (o *outlinePage) grid(c *ui.Context) {
	content := o.host.content()
	lines := slices.Clone(content.Plan().Lines)
	g := o.layout(c)
	width, height := o.viewport.W, o.viewport.H
	if width <= 0 || height <= 0 {
		width, height = c.Size()
	}
	o.revealCard(g, width, height)
	o.head(c, planChapterWidth, func() {
		for _, l := range lines {
			ui.Column(c.Key(l.ID)).Width(planCardWidth).FillHeight().Padding(0, 12).Justify(ui.Center).Shrink(0).Children(func() { o.line(c, l) })
		}
	})
	body := ui.ScrollBoth(c.Key("plan-grid")).Grow(1).Shrink(1).FillWidth().TrackScroll(&o.scroll[0])
	o.viewport = body.Bounds()
	if o.viewport.H > 0 {
		height = o.viewport.H
	}
	// Clamp before selecting rows, including when a search shrinks the result.
	o.scroll[0].Y = min(o.scroll[0].Y, max(0, g.height+24-height))
	body.TrackScroll(&o.scroll[0])
	body.Children(func() {
		ui.Column(c).Width(float32(planChapterWidth+planCardWidth*len(lines)+planEndWidth)).Padding(0, 0, 24, 0).Children(func() {
			gap := float32(0)
			held, _ := content.Plan().Card(o.dragging)
			for i, row := range g.rows {
				pinned := row.node != nil && row.node.ID == o.cardChapter(held) && o.dragging != ""
				if !pinned && (row.y+row.height < o.scroll[0].Y-height || row.y > o.scroll[0].Y+2*height) {
					gap += row.height
					continue
				}
				if gap > 0 {
					ui.Box(c.Key(fmt.Sprintf("gap-%d", i))).Height(gap).Shrink(0)
					gap = 0
				}
				o.gridRow(c.Key(fmt.Sprintf("row-%d", i)), row, lines)
			}
			if gap > 0 {
				ui.Box(c.Key("gap-end")).Height(gap).Shrink(0)
			}
			if len(g.rows) == 0 {
				ui.Column(c).Width(planGistWidth).Padding(32, 24).Gap(8).Children(func() {
					ui.Text(c, "章节里没有「"+kit.Trim(o.query)+"」").FontSize(kit.FsDialog).FontWeight(600)
					ui.Text(c, "找到的卡片在下面「未放进章节」里。").FontSize(kit.FsNote).TextColor(o.host.colors().Theme.TextMuted)
				})
			}
		})
	})
}

func (o *outlinePage) gridRow(c *ui.Context, row planRow, lines []workspace.PlotLine) {
	pal := o.host.colors()
	n := row.node
	if row.kind == 2 {
		ui.Text(c, "不在卷中").FontSize(kit.FsGroup).FontWeight(600).Padding(16, 24, 12, 24).Height(row.height).Shrink(0).Translate(o.scroll[0].X, 0).FillWidth().Background(pal.Side)
		return
	}
	if row.kind == 1 {
		ui.Column(c).FillWidth().Height(row.height).Shrink(0).Padding(16, 0, 12, 0).Background(pal.Side).BorderWidth(0, 0, 1, 0).BorderColor(pal.Line).Children(func() {
			ui.Column(c).Width(planGistWidth).Margin(0, 0, 0, 24).Translate(o.scroll[0].X, 0).Gap(4).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.RichText(c, kit.Marked(n.Title, kit.Trim(o.query), pal.Ink)...).FontSize(kit.FsGroup).FontWeight(600).SingleLine()
					text := fmt.Sprintf("%d 章", row.count)
					if row.when != "" {
						text += " · " + row.when
					}
					ui.Text(c, text).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted).SingleLine()
				})
				o.gist(c, n)
			})
		})
		return
	}
	ui.Row(c).FillWidth().Height(row.height).Shrink(0).AlignItems(ui.Stretch).Children(func() {
		ui.Box(c).Width(planChapterWidth).Shrink(0)
		for _, l := range lines {
			o.cell(c.Key(n.ID+"|"+l.ID), n.ID, l.ID, -2, "", "")
		}
		ui.Box(c).Grow(1).BorderWidth(0, 0, 1, 0).BorderColor(pal.Line)
		o.side(c, planChapterWidth, "大纲章节-"+n.ID, func() {
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				b := ui.ButtonBase(c).Grow(1).Shrink(1).Radius(kit.RXs).Label("打开正文-" + n.ID).Tooltip("打开正文")
				if b.Hovered() {
					b.Background(pal.Theme.SurfaceHover)
				}
				b.Children(func() { o.text(c, n.Title, false).FontWeight(600).SingleLine().Ellipsis("…") })
				if b.Clicked() {
					o.host.openWriting(n.ID)
				}
				if o.back(n.ID) {
					ui.Text(c, "回溯").FontSize(kit.FsSmall).Padding(2, 4).Radius(kit.RXs).Border(1, pal.Line).Tooltip("这一章写到了比前面各章更早的事")
				}
			})
			if row.when != "" {
				ui.Text(c, "故事中 "+row.when).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted).SingleLine().Ellipsis("…")
			}
			o.gist(c, n)
			stray := 0
			for _, e := range o.cardsOf(n.ID) {
				if e.Line == "" {
					stray++
				}
			}
			if stray > 0 {
				ui.Textf(c, "另有 %d 张不在任何情节线", stray).FontSize(kit.FsNote).TextColor(pal.Faint)
			}
		})
	})
}

func (o *outlinePage) cell(c *ui.Context, chapter, line string, col int, when, after string) {
	pal := o.host.colors()
	content := o.host.content()
	cell := ui.Column(c).Width(planCardWidth).Shrink(0).Padding(10, 12).Gap(8).BorderWidth(0, 1, 1, 0).BorderColor(pal.Line).Label(fmt.Sprintf("大纲格-%s|%s|%d", chapter, line, col))
	if col != -2 {
		cell.BorderWidth(0, 1, 0, 0)
	}
	if _, over := ui.DragOver[dragCard](cell); over {
		cell.Background(pal.Theme.SurfaceHover)
	}
	if id, ok := ui.Drop[dragCard](cell); ok {
		if col == -2 {
			o.change(content.PlaceCard(string(id), chapter, &line))
		} else {
			o.change(content.DropCard(string(id), col, line))
		}
	}
	cell.Children(func() {
		var cards []workspace.Card
		if col == -2 {
			for _, e := range o.cardsOf(chapter) {
				if e.Line == line {
					cards = append(cards, e)
				}
			}
		} else if col >= 0 {
			cols := o.index.columns
			if col < len(cols) {
				for _, e := range cols[col].Cards {
					if e.Line == line {
						cards = append(cards, e)
					}
				}
			}
		}
		for _, e := range cards {
			o.cardView(c.Key("card-"+e.ID), e, col != -2, false)
		}
		b := ui.ButtonBase(c).FillWidth().Height(kit.HControl).Center().Gap(6).Row().Border(1, pal.Theme.Border).BorderStyle(ui.BorderDashed).Radius(kit.RControl).Label(fmt.Sprintf("添加卡片-%s|%s|%d", chapter, line, col)).Tooltip("添加卡片")
		if b.Hovered() {
			b.Background(pal.Theme.SurfaceHover)
		}
		if !cell.Hovered() && !b.Hovered() {
			b.Opacity(0)
		}
		b.Children(func() {
			kit.Glyph(c, kit.IcPlus, kit.IconSm, pal.Theme.TextMuted)
			ui.Text(c, "添加").FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted)
		})
		if b.Clicked() {
			o.newCard(chapter, line, when, after)
		}
	})
}

func (o *outlinePage) cardView(c *ui.Context, e workspace.Card, timeline, tray bool) {
	pal := o.host.colors()
	content := o.host.content()
	assigned := o.cardChapter(e) != ""
	b := ui.ButtonBase(c).Column().FillWidth().Padding(6, 10).Gap(2).Radius(kit.RControl).Background(pal.Raised).Border(1, pal.Theme.Border).Label("卡片-" + e.ID).Tooltip("点击修改；拖到别的格子挪过去").Drag(dragCard(e.ID))
	if b.Hovered() {
		b.Border(1, pal.Control).Translate(0, -2)
		pal.Pop.On(b)
	} else {
		pal.Raise.On(b)
	}
	if b.Dragging() {
		o.dragging = e.ID
	} else if o.dragging == e.ID {
		o.dragging = ""
	}
	if !assigned {
		b.BorderStyle(ui.BorderDashed).Background(pal.Canvas)
	}
	if o.seen == e.ID {
		b.Border(1, pal.Ink)
		pal.Pop.On(b)
		if o.reveal {
			b.ScrollIntoView()
			o.reveal = false
		}
	}
	if !o.cardFound(e) {
		b.Opacity(kit.OpacityDisabled)
	}
	b.Children(func() {
		if e.When != "" && !timeline {
			text := e.When
			if tray {
				for _, l := range content.Plan().Lines {
					if l.ID == e.Line {
						text += " · " + l.Name
					}
				}
			}
			o.text(c, text, true).TextColor(pal.Theme.TextMuted).SingleLine().Ellipsis("…")
		}
		o.text(c, e.Title, false).MaxHeight(kit.FsBody * kit.LhBody * 2).Clip()
		if timeline {
			text := "未放进章节"
			if assigned {
				n, _ := content.Tree().Node(e.Chapter)
				text = n.Title
			}
			o.text(c, text, true).TextColor(pal.Theme.TextMuted).SingleLine().Ellipsis("…")
		}
	})
	if b.Clicked() {
		o.openCard(e)
	}
}

func (o *outlinePage) tray(c *ui.Context) {
	pal := o.host.colors()
	cards := o.cardsOf("")
	tray := ui.Scroll(c.Key("plan-tray")).FillWidth().MaxHeight(planTrayHeight).Shrink(0).Padding(10, 24, 14, 24).Gap(8).Background(pal.Side).BorderWidth(1, 0, 0, 0).BorderColor(pal.Theme.Border).Label("大纲托盘")
	if _, over := ui.DragOver[dragCard](tray); over {
		tray.Background(pal.Theme.SurfaceHover)
	}
	if id, ok := ui.Drop[dragCard](tray); ok {
		o.change(o.host.content().PlaceCard(string(id), "", nil))
	}
	tray.Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "未放进章节").FontSize(kit.FsGroup).FontWeight(600)
			note := "想到了、还没定写在哪的事，先记在这里"
			if len(cards) > 0 {
				note = fmt.Sprintf("%d 张 · 拖进上面的格子就放进那一章", len(cards))
			}
			ui.Text(c, note).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted).Grow(1).Shrink(1)
			if pal.Ghost(c, "记一张", ui.Color{}).Clicked() {
				o.newCard("", "", "", "")
			}
		})
		if len(cards) > 0 {
			ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Start).Children(func() {
				for _, e := range cards {
					ui.Box(c.Key(e.ID)).Width(planCardWidth).Children(func() { o.cardView(c, e, false, true) })
				}
			})
		}
	})
}

func (o *outlinePage) timeline(c *ui.Context) {
	pal := o.host.colors()
	content := o.host.content()
	cols := o.index.columns
	g := o.layout(c)
	width, height := o.viewport.W, o.viewport.H
	if width <= 0 || height <= 0 {
		width, height = c.Size()
	}
	o.revealCard(g, width, height)
	viewWidth := max(planCardWidth, width-planLaneWidth)
	o.scroll[1].X = min(o.scroll[1].X, max(0, float32(planLaneWidth+planCardWidth*(len(cols)+1))-width))
	first := max(0, int((o.scroll[1].X-viewWidth)/planCardWidth))
	end := min(len(cols)+1, int((o.scroll[1].X+2*viewWidth)/planCardWidth)+1)
	held := -1
	if o.dragging != "" {
		for i, col := range cols {
			if slices.ContainsFunc(col.Cards, func(e workspace.Card) bool { return e.ID == o.dragging }) {
				held = i
				break
			}
		}
	}
	lanes := slices.Clone(content.Plan().Lines)
	if slices.ContainsFunc(content.Plan().Cards, func(e workspace.Card) bool { return e.Line == "" }) {
		lanes = append(lanes, workspace.PlotLine{Name: "不分情节线"})
	}
	ui.Row(c).FillWidth().Padding(10, 24, 0, 24).Gap(16).Shrink(0).Children(func() {
		for _, loose := range []bool{false, true} {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
				key := ui.Box(c).Size(kit.IconMd, kit.RCard).Radius(kit.RXs).Border(1, pal.Control).Background(pal.Raised)
				text := "已放进章节"
				if loose {
					key.BorderStyle(ui.BorderDashed).Background(pal.Canvas)
					text = "未放进章节"
				}
				ui.Text(c, text).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted)
			})
		}
		ui.Text(c, "时间可以不写，左右的先后就是故事里的先后").FontSize(kit.FsNote).TextColor(pal.Faint).Grow(1).Shrink(1)
		var off []string
		for _, n := range o.chapters() {
			if len(o.cardsOf(n.ID)) == 0 {
				off = append(off, n.Title)
			}
		}
		if len(off) > 0 {
			word := fmt.Sprintf("%d 章", len(off))
			if len(off) == 1 {
				word = "「" + off[0] + "」"
			}
			ui.Text(c, word+"还没有卡片").FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted).SingleLine()
		}
	})
	o.head(c, planLaneWidth, func() {
		planColumns(c, len(cols)+1, first, end, held, func(i int) {
			if i < len(cols) {
				o.timeHead(c.Key(i), i, cols[i])
			} else {
				o.timeHead(c.Key("end"), -1, workspace.TimeColumn{When: "之后"})
			}
		})
	})
	body := ui.ScrollBoth(c.Key("plan-timeline")).Grow(1).Shrink(1).FillWidth().TrackScroll(&o.scroll[1])
	o.viewport = body.Bounds()
	body.Children(func() {
		ui.Column(c).Width(max(width, float32(planLaneWidth+planCardWidth*(len(cols)+1)))).Padding(0, 0, 24, 0).Children(func() {
			for _, l := range lanes {
				ui.Row(c.Key("lane-"+l.ID)).FillWidth().Height(max(planLaneMinHeight, g.lanes[l.ID])).Shrink(0).AlignItems(ui.Stretch).BorderWidth(0, 0, 1, 0).BorderColor(pal.Line).Children(func() {
					ui.Box(c).Width(planLaneWidth).Shrink(0)
					planColumns(c, len(cols)+1, first, end, held, func(i int) {
						if i < len(cols) {
							col := cols[i]
							o.cell(c.Key(fmt.Sprintf("%d|%s", i, l.ID)), "", l.ID, i, col.When, col.Cards[len(col.Cards)-1].ID)
						} else {
							after := ""
							if len(content.Plan().Cards) > 0 {
								after = content.Plan().Cards[len(content.Plan().Cards)-1].ID
							}
							o.cell(c.Key("end-"+l.ID), "", l.ID, -1, "", after)
						}
					})
					o.side(c, planLaneWidth, "大纲泳道-"+l.ID, func() {
						if l.ID != "" {
							o.line(c, l)
						} else {
							ui.Text(c, l.Name).FontSize(kit.FsBody).FontWeight(600)
						}
					})
				})
			}
			ui.Row(c).FillWidth().Height(kit.TitleHeight).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(pal.Line).Label("新增情节线整行").Children(func() { o.side(c, planLaneWidth, "新增情节线行", func() { o.addLine(c) }) })
		})
	})
}

func (o *outlinePage) timeHead(c *ui.Context, i int, col workspace.TimeColumn) {
	pal := o.host.colors()
	ui.Column(c).Width(planCardWidth).Height(kit.TitleHeight+kit.HTag).Padding(12, 12, 0, 12).Gap(6).Shrink(0).Children(func() {
		label := col.When
		if label == "" {
			label = "未写时间"
		}
		b := ui.ButtonBase(c).FillWidth().Padding(2, 6).Radius(kit.RXs).Label(fmt.Sprintf("时间列-%d", i)).Tooltip("点击修改这一列的时间").Disabled(i < 0)
		if b.Hovered() {
			b.Background(pal.Theme.SurfaceHover)
		}
		b.Children(func() { o.text(c, label, false).FontWeight(600).SingleLine().Ellipsis("…") })
		if b.Clicked() {
			o.finishOutline()
			o.finishLine()
			o.column, o.when, o.timeOpen = i, col.When, true
		}
		ui.Box(c).FillWidth().Height(kit.HTag).Draw(func(painter *ui.Painter, r ui.Rect) {
			end := r.X + r.W + 12
			if i < 0 {
				end = r.X + r.W/2
			}
			start := r.X - 12
			if i == 0 {
				start = r.X
			}
			painter.Line(start, r.Y+r.H/2, end, r.Y+r.H/2, 1, pal.Theme.Border)
			if i >= 0 {
				assigned := slices.ContainsFunc(col.Cards, func(e workspace.Card) bool { return o.cardChapter(e) != "" })
				color := pal.Ink
				if !assigned {
					color = pal.Control
				}
				dot := ui.Rect{X: r.X, Y: r.Y + (r.H-kit.RCard)/2, W: kit.RCard, H: kit.RCard}
				painter.Fill(dot, color, kit.RRound)
				if !assigned {
					dot.X += 2
					dot.Y += 2
					dot.W -= 4
					dot.H -= 4
					painter.Fill(dot, pal.Canvas, kit.RRound)
				}
			}
		})
		open := o.timeOpen && o.column == i
		ui.PopoverBase(c, b, &open, func(panel ui.Element) {
			panel.Width(planTimePopWidth).Padding(10).Gap(8).Radius(kit.RCard).Background(pal.Raised).Border(1, pal.Theme.Border)
			pal.Pop.On(panel)
			ui.Textf(c, "这一列 %d 张卡片在故事中的时间", len(col.Cards)).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted)
			in := ui.TextInput(c, &o.when).Label("这一列的时间").Placeholder("例如：十月六日夜").FillWidth().AutoFocus()
			save := in.Submitted()
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if pal.Secondary(c, "取消").Label("取消时间").Clicked() {
					open = false
				}
				if pal.Primary(c, "保存").Label("保存时间").Clicked() {
					save = true
				}
			})
			if save {
				o.change(o.host.content().SaveTime(o.column, o.when))
				if o.host.notices().ErrorText() == "" {
					open = false
				}
			}
		})
		if o.column == i {
			o.timeOpen = open
		}
	})
}

func (o *outlinePage) newCard(chapter, line, when, after string) {
	o.finishOutline()
	o.finishLine()
	o.cardOpen, o.card, o.after, o.added, o.lastTitle = true, workspace.Card{Chapter: chapter, Line: line, When: when}, after, 0, ""
}

func (o *outlinePage) openCard(e workspace.Card) { o.newCard("", "", "", ""); o.card = e }

func (o *outlinePage) seeCard(id string) { o.seen, o.seenAt, o.reveal = id, time.Now(), true }

func (o *outlinePage) saveCard(more bool) {
	if kit.Trim(o.card.Title) == "" {
		return
	}
	fresh := o.card.ID == ""
	e, err := o.host.content().SaveCard(o.card, o.after)
	o.change(err)
	if err != nil {
		return
	}
	if fresh {
		o.seeCard(e.ID)
	}
	if more && fresh {
		o.card.Title, o.card.About = "", ""
		o.card.When = e.When
		o.after = e.ID
		o.lastTitle = e.Title
		o.added++
	} else {
		o.cardOpen = false
	}
}

func (o *outlinePage) dialog(c *ui.Context) {
	pal := o.host.colors()
	status := o.host.notices()
	content := o.host.content()
	if !o.cardOpen {
		return
	}
	pal.Modal(c, &o.cardOpen, func() {
		ui.Column(c).Width(kit.DialogSmall).Padding(kit.DialogPad).Gap(16).Children(func() {
			title, about := "新卡片", "一张卡片记一件事。只有「发生了什么」要写，别的都可以以后再补。"
			if o.card.ID != "" {
				title, about = "修改卡片", ""
			} else if o.added > 0 {
				about = "已添加「" + o.lastTitle + "」，接着记下一张。"
			}
			pal.DialogHead(c, title, about)
			var enter bool
			kit.Labeled(c, "发生了什么", func() {
				in := ui.TextInput(c.Key(fmt.Sprintf("card-title-%d", o.added)), &o.card.Title).Label("事件").Placeholder("一句话，例如：林晚下井").FillWidth().AutoFocus()
				enter = in.Submitted()
			})
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).Children(func() {
					kit.Labeled(c, "写在哪一章", func() {
						names, ids := []string{"未放进章节"}, map[string]string{}
						chosen := names[0]
						for _, n := range o.chapters() {
							name := n.Title
							for count := 2; ids[name] != ""; count++ {
								name = fmt.Sprintf("%s (%d)", n.Title, count)
							}
							names = append(names, name)
							ids[name] = n.ID
							if n.ID == o.card.Chapter {
								chosen = name
							}
						}
						if ui.Select(c, &chosen, names).Label("卡片章节").FillWidth().MinWidth(0).Changed() {
							o.card.Chapter = ids[chosen]
						}
					})
				})
				ui.Column(c).Grow(1).Shrink(1).Children(func() {
					kit.Labeled(c, "情节线", func() {
						names, ids := []string{"不分情节线"}, map[string]string{}
						chosen := names[0]
						for _, l := range content.Plan().Lines {
							name := l.Name
							for count := 2; ids[name] != ""; count++ {
								name = fmt.Sprintf("%s (%d)", l.Name, count)
							}
							names = append(names, name)
							ids[name] = l.ID
							if l.ID == o.card.Line {
								chosen = name
							}
						}
						if ui.Select(c, &chosen, names).Label("卡片情节线").FillWidth().MinWidth(0).Changed() {
							o.card.Line = ids[chosen]
						}
					})
				})
			})
			kit.Labeled(c, "故事中的时间（选填）", func() {
				in := ui.TextInput(c, &o.card.When).Label("故事中的时间").Placeholder("例如：十月六日夜。不写也行，按先后排").FillWidth()
				enter = enter || in.Submitted()
			})
			kit.Labeled(c, "说明（选填）", func() { ui.TextArea(c, &o.card.About).Label("说明").FillWidth().Height(kit.HControl * 3) })
			save, more, drop := enter && kit.Trim(o.card.Title) != "", false, false
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if o.card.ID != "" {
					if pal.Ghost(c, "删除卡片", pal.Theme.Danger).Clicked() {
						drop = true
					}
					ui.Spacer(c)
				}
				if pal.Secondary(c, "取消").Label("取消卡片").Clicked() {
					o.cardOpen = false
				}
				if o.card.ID == "" {
					if pal.Secondary(c, "继续添加").Disabled(kit.Trim(o.card.Title) == "").Clicked() {
						more = true
					}
				}
				verb := "添加"
				if o.card.ID != "" {
					verb = "保存"
				}
				if pal.Primary(c, verb).Label("保存卡片").Disabled(kit.Trim(o.card.Title) == "").Clicked() {
					save = true
				}
			})
			if drop {
				o.change(content.DeleteCard(o.card.ID))
				if status.ErrorText() == "" {
					status.Did("已删除卡片「" + o.card.Title + "」")
					o.cardOpen = false
				}
			} else if save || more {
				o.saveCard(more)
			}
		})
	})
}

func (o *outlinePage) chapterStory(c *ui.Context) {
	if !o.loaded {
		o.load()
	} else {
		o.rebuildIndex()
	}
	if o.loadErr != nil {
		return
	}
	id, _, _ := o.host.chapter()
	cards := o.cardsOf(id)
	when := storyWhen(cards)
	if when == "" {
		return
	}
	if o.host.colors().Ghost(c, "故事中 "+when, ui.Color{}).Clicked() {
		o.host.showOutline()
		o.mode = 1
		o.query = ""
		o.loaded = false
		for _, e := range cards {
			if e.When != "" {
				o.seeCard(e.ID)
				break
			}
		}
	}
}

func (o *outlinePage) enter() { o.loaded = false }

func (o *outlinePage) leave() {
	o.finishOutline()
	o.finishLine()
}
