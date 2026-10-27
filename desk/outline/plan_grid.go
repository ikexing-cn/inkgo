package outline

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"slices"
	"writer/desk/kit"
	"writer/workspace"
)

// The heading is outside the vertical scroller. Body side cells are
// painted last and translated back by its horizontal offset, so neither
// axis can carry the chapter names or the heading out of view.
func (p *Page) head(c *ui.Context, left float32, heads func()) {
	pal := p.Host.Colors()
	height := float32(kit.TitleHeight)
	if p.mode == 1 {
		height += kit.HTag
	}
	ui.Row(c).FillWidth().MinWidth(0).Height(height).Shrink(0).Clip().Children(func() {
		word := "章节"
		if p.mode == 1 {
			word = "情节线"
		}
		ui.Column(c).Width(left).FillHeight().Padding(0, 12, 0, 24).Justify(ui.Center).Shrink(0).Background(pal.Canvas).Children(func() { ui.Text(c, word).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted) })
		ui.Box(c).Grow(1).Shrink(1).MinWidth(0).FillHeight().Clip().Children(func() {
			ui.Row(c).Height(height).Translate(-p.scroll[p.mode].X, 0).Children(heads)
		})
		if p.mode == 0 {
			ui.Box(c).Width(planEndWidth).FillHeight().Center().Shrink(0).Background(pal.Canvas).BorderWidth(0, 0, 0, 1).BorderColor(pal.Line).Children(func() { p.addLine(c) })
		}
	}).BorderWidth(0, 0, 1, 0).BorderColor(pal.Theme.Border)
}

func (p *Page) side(c *ui.Context, left float32, label string, fn func()) {
	bottom := float32(1)
	if p.mode == 1 {
		bottom = 0
	} // the entire lane draws its bottom border
	ui.Column(c).Absolute().Left(0).Top(0).Bottom(0).Width(left).Translate(p.scroll[p.mode].X, 0).Padding(16, 12, 16, 24).Gap(4).Background(p.Host.Colors().Canvas).BorderWidth(0, 1, bottom, 0).BorderColor(p.Host.Colors().Line).Label(label).Children(fn)
}

func (p *Page) grid(c *ui.Context) {
	content := p.Host.Content()
	lines := slices.Clone(content.Plan().Lines)
	g := p.layout(c)
	width, height := p.viewport.W, p.viewport.H
	if width <= 0 || height <= 0 {
		width, height = c.Size()
	}
	p.revealCard(g, width, height)
	p.head(c, planChapterWidth, func() {
		for _, l := range lines {
			ui.Column(c.Key(l.ID)).Width(planCardWidth).FillHeight().Padding(0, 12).Justify(ui.Center).Shrink(0).Children(func() { p.line(c, l) })
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
			held, _ := content.Plan().Card(p.dragging)
			for i, row := range g.rows {
				pinned := row.node != nil && row.node.ID == p.cardChapter(held) && p.dragging != ""
				if !pinned && (row.y+row.height < p.scroll[0].Y-height || row.y > p.scroll[0].Y+2*height) {
					gap += row.height
					continue
				}
				if gap > 0 {
					ui.Box(c.Key(fmt.Sprintf("gap-%d", i))).Height(gap).Shrink(0)
					gap = 0
				}
				p.gridRow(c.Key(fmt.Sprintf("row-%d", i)), row, lines)
			}
			if gap > 0 {
				ui.Box(c.Key("gap-end")).Height(gap).Shrink(0)
			}
			if len(g.rows) == 0 {
				ui.Column(c).Width(planGistWidth).Padding(32, 24).Gap(8).Children(func() {
					ui.Text(c, "章节里没有「"+kit.Trim(p.query)+"」").FontSize(kit.FsDialog).FontWeight(600)
					ui.Text(c, "找到的卡片在下面「未放进章节」里。").FontSize(kit.FsNote).TextColor(p.Host.Colors().Theme.TextMuted)
				})
			}
		})
	})
}

func (p *Page) gridRow(c *ui.Context, row planRow, lines []workspace.PlotLine) {
	pal := p.Host.Colors()
	n := row.node
	if row.kind == 2 {
		ui.Text(c, "不在卷中").FontSize(kit.FsGroup).FontWeight(600).Padding(16, 24, 12, 24).Height(row.height).Shrink(0).Translate(p.scroll[0].X, 0).FillWidth().Background(pal.Side)
		return
	}
	if row.kind == 1 {
		ui.Column(c).FillWidth().Height(row.height).Shrink(0).Padding(16, 0, 12, 0).Background(pal.Side).BorderWidth(0, 0, 1, 0).BorderColor(pal.Line).Children(func() {
			ui.Column(c).Width(planGistWidth).Margin(0, 0, 0, 24).Translate(p.scroll[0].X, 0).Gap(4).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.RichText(c, kit.Marked(n.Title, kit.Trim(p.query), pal.Ink)...).FontSize(kit.FsGroup).FontWeight(600).SingleLine()
					text := fmt.Sprintf("%d 章", row.count)
					if row.when != "" {
						text += " · " + row.when
					}
					ui.Text(c, text).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted).SingleLine()
				})
				p.gist(c, n)
			})
		})
		return
	}
	ui.Row(c).FillWidth().Height(row.height).Shrink(0).AlignItems(ui.Stretch).Children(func() {
		ui.Box(c).Width(planChapterWidth).Shrink(0)
		for _, l := range lines {
			p.cell(c.Key(n.ID+"|"+l.ID), n.ID, l.ID, -2, "", "")
		}
		ui.Box(c).Grow(1).BorderWidth(0, 0, 1, 0).BorderColor(pal.Line)
		p.side(c, planChapterWidth, "大纲章节-"+n.ID, func() {
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				b := ui.ButtonBase(c).Grow(1).Shrink(1).Radius(kit.RXs).Label("打开正文-" + n.ID).Tooltip("打开正文")
				if b.Hovered() {
					b.Background(pal.Theme.SurfaceHover)
				}
				b.Children(func() { p.text(c, n.Title, false).FontWeight(600).SingleLine().Ellipsis("…") })
				if b.Clicked() {
					p.Host.OpenWriting(n.ID)
				}
				if p.back(n.ID) {
					ui.Text(c, "回溯").FontSize(kit.FsSmall).Padding(2, 4).Radius(kit.RXs).Border(1, pal.Line).Tooltip("这一章写到了比前面各章更早的事")
				}
			})
			if row.when != "" {
				ui.Text(c, "故事中 "+row.when).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted).SingleLine().Ellipsis("…")
			}
			p.gist(c, n)
			stray := 0
			for _, e := range p.cardsOf(n.ID) {
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

func (p *Page) cell(c *ui.Context, chapter, line string, col int, when, after string) {
	pal := p.Host.Colors()
	content := p.Host.Content()
	cell := ui.Column(c).Width(planCardWidth).Shrink(0).Padding(10, 12).Gap(8).BorderWidth(0, 1, 1, 0).BorderColor(pal.Line).Label(fmt.Sprintf("大纲格-%s|%s|%d", chapter, line, col))
	if col != -2 {
		cell.BorderWidth(0, 1, 0, 0)
	}
	if _, over := ui.DragOver[dragCard](cell); over {
		cell.Background(pal.Theme.SurfaceHover)
	}
	if id, ok := ui.Drop[dragCard](cell); ok {
		if col == -2 {
			p.change(content.PlaceCard(string(id), chapter, &line))
		} else {
			p.change(content.DropCard(string(id), col, line))
		}
	}
	cell.Children(func() {
		var cards []workspace.Card
		if col == -2 {
			for _, e := range p.cardsOf(chapter) {
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
			p.cardView(c.Key("card-"+e.ID), e, col != -2, false)
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
			p.newCard(chapter, line, when, after)
		}
	})
}

func (p *Page) cardView(c *ui.Context, e workspace.Card, timeline, tray bool) {
	pal := p.Host.Colors()
	content := p.Host.Content()
	assigned := p.cardChapter(e) != ""
	b := ui.ButtonBase(c).Column().FillWidth().Padding(6, 10).Gap(2).Radius(kit.RControl).Background(pal.Raised).Border(1, pal.Theme.Border).Label("卡片-" + e.ID).Tooltip("点击修改；拖到别的格子挪过去").Drag(dragCard(e.ID))
	if b.Hovered() {
		b.Border(1, pal.Control).Translate(0, -2)
		pal.Pop.On(b)
	}
	if b.Dragging() {
		pal.Pop.On(b)
		p.dragging = e.ID
	} else if p.dragging == e.ID {
		p.dragging = ""
	}
	if !assigned {
		b.BorderStyle(ui.BorderDashed).Background(pal.Canvas)
	}
	if p.seen == e.ID {
		b.Border(1, pal.Ink)
		pal.Pop.On(b)
		if p.reveal {
			b.ScrollIntoView()
			p.reveal = false
		}
	}
	if !p.cardFound(e) {
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
			p.text(c, text, true).TextColor(pal.Theme.TextMuted).SingleLine().Ellipsis("…")
		}
		p.text(c, e.Title, false).MaxHeight(kit.FsBody * kit.LhBody * 2).Clip()
		if timeline {
			text := "未放进章节"
			if assigned {
				n, _ := content.Tree().Node(e.Chapter)
				text = n.Title
			}
			p.text(c, text, true).TextColor(pal.Theme.TextMuted).SingleLine().Ellipsis("…")
		}
	})
	if b.Clicked() {
		p.openCard(e)
	}
}

func (p *Page) tray(c *ui.Context) {
	pal := p.Host.Colors()
	cards := p.cardsOf("")
	tray := ui.Scroll(c.Key("plan-tray")).FillWidth().MaxHeight(planTrayHeight).Shrink(0).Padding(10, 24, 14, 24).Gap(8).Background(pal.Side).BorderWidth(1, 0, 0, 0).BorderColor(pal.Theme.Border).Label("大纲托盘")
	if _, over := ui.DragOver[dragCard](tray); over {
		tray.Background(pal.Theme.SurfaceHover)
	}
	if id, ok := ui.Drop[dragCard](tray); ok {
		p.change(p.Host.Content().PlaceCard(string(id), "", nil))
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
				p.newCard("", "", "", "")
			}
		})
		if len(cards) > 0 {
			ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Start).Children(func() {
				for _, e := range cards {
					ui.Box(c.Key(e.ID)).Width(planCardWidth).Children(func() { p.cardView(c, e, false, true) })
				}
			})
		}
	})
}

func (p *Page) timeline(c *ui.Context) {
	pal := p.Host.Colors()
	content := p.Host.Content()
	cols := p.index.columns
	g := p.layout(c)
	width, height := p.viewport.W, p.viewport.H
	if width <= 0 || height <= 0 {
		width, height = c.Size()
	}
	p.revealCard(g, width, height)
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
		for _, n := range p.chapters() {
			if len(p.cardsOf(n.ID)) == 0 {
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
	p.head(c, planLaneWidth, func() {
		planColumns(c, len(cols)+1, first, end, held, func(i int) {
			if i < len(cols) {
				p.timeHead(c.Key(i), i, cols[i])
			} else {
				p.timeHead(c.Key("end"), -1, workspace.TimeColumn{When: "之后"})
			}
		})
	})
	body := ui.ScrollBoth(c.Key("plan-timeline")).Grow(1).Shrink(1).FillWidth().TrackScroll(&p.scroll[1])
	p.viewport = body.Bounds()
	body.Children(func() {
		ui.Column(c).Width(max(width, float32(planLaneWidth+planCardWidth*(len(cols)+1)))).Padding(0, 0, 24, 0).Children(func() {
			for _, l := range lanes {
				ui.Row(c.Key("lane-"+l.ID)).FillWidth().Height(max(planLaneMinHeight, g.lanes[l.ID])).Shrink(0).AlignItems(ui.Stretch).BorderWidth(0, 0, 1, 0).BorderColor(pal.Line).Children(func() {
					ui.Box(c).Width(planLaneWidth).Shrink(0)
					planColumns(c, len(cols)+1, first, end, held, func(i int) {
						if i < len(cols) {
							col := cols[i]
							p.cell(c.Key(fmt.Sprintf("%d|%s", i, l.ID)), "", l.ID, i, col.When, col.Cards[len(col.Cards)-1].ID)
						} else {
							after := ""
							if len(content.Plan().Cards) > 0 {
								after = content.Plan().Cards[len(content.Plan().Cards)-1].ID
							}
							p.cell(c.Key("end-"+l.ID), "", l.ID, -1, "", after)
						}
					})
					p.side(c, planLaneWidth, "大纲泳道-"+l.ID, func() {
						if l.ID != "" {
							p.line(c, l)
						} else {
							ui.Text(c, l.Name).FontSize(kit.FsBody).FontWeight(600)
						}
					})
				})
			}
			ui.Row(c).FillWidth().Height(kit.TitleHeight).Shrink(0).BorderWidth(0, 0, 1, 0).BorderColor(pal.Line).Label("新增情节线整行").Children(func() { p.side(c, planLaneWidth, "新增情节线行", func() { p.addLine(c) }) })
		})
	})
}

func (p *Page) timeHead(c *ui.Context, i int, col workspace.TimeColumn) {
	pal := p.Host.Colors()
	ui.Column(c).Width(planCardWidth).Height(kit.TitleHeight+kit.HTag).Padding(12, 12, 0, 12).Gap(6).Shrink(0).Children(func() {
		label := col.When
		if label == "" {
			label = "未写时间"
		}
		b := ui.ButtonBase(c).FillWidth().Padding(2, 6).Radius(kit.RXs).Label(fmt.Sprintf("时间列-%d", i)).Tooltip("点击修改这一列的时间").Disabled(i < 0)
		if b.Hovered() {
			b.Background(pal.Theme.SurfaceHover)
		}
		b.Children(func() { p.text(c, label, false).FontWeight(600).SingleLine().Ellipsis("…") })
		if b.Clicked() {
			p.FinishOutline()
			p.finishLine()
			p.column, p.when, p.timeOpen = i, col.When, true
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
				assigned := slices.ContainsFunc(col.Cards, func(e workspace.Card) bool { return p.cardChapter(e) != "" })
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
		open := p.timeOpen && p.column == i
		kit.Popover(c, b, &open, 0, 0, func(panel ui.Element) {
			panel.Width(planTimePopWidth).Padding(10).Gap(8).Radius(kit.RCard).Background(pal.Raised).Border(1, pal.Theme.Border)
			pal.Pop.On(panel)
			ui.Textf(c, "这一列 %d 张卡片在故事中的时间", len(col.Cards)).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted)
			in := ui.TextInput(c, &p.when).Label("这一列的时间").Placeholder("例如：十月六日夜").FillWidth().AutoFocus()
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
				p.change(p.Host.Content().SaveTime(p.column, p.when))
				if p.Host.Notices().ErrorText() == "" {
					open = false
				}
			}
		})
		if p.column == i {
			p.timeOpen = open
		}
	})
}
