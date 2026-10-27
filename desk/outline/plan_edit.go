package outline

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"slices"
	"time"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/workspace"
)

func (p *Page) line(c *ui.Context, l workspace.PlotLine) {
	status := p.Host.Notices()
	content := p.Host.Content()
	t := p.Host.Colors().Theme
	if p.lineID == l.ID {
		in := ui.TextInputBase(c.Key("line-name-"+l.ID), &p.lineName).Label("情节线的名字").Placeholder("情节线的名字").FillWidth().Height(kit.HControlSm).FontSize(kit.FsBody).Padding(0, 6).Border(1, t.Border).Radius(kit.RControl).AutoFocus()
		if p.lineFocus {
			in.Focus().SetTextSelection(0, len([]rune(p.lineName)))
			p.lineFocus = false
		}
		switch {
		case in.Shortcut(0, ui.KeyEscape):
			p.lineID = ""
		case in.Submitted() || p.lineHeld && !in.Focused():
			p.finishLine()
		case in.Focused():
			p.lineHeld = true
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
			p.startLine(l)
		}
		n := p.index.counts[l.ID]
		ui.Textf(c, "%d", n).FontSize(kit.FsNote).TextColor(p.Host.Colors().Faint)
		more := kit.IconButton(c, kit.IcMore, "情节线「"+l.Name+"」的操作", ui.Color{})
		p.Host.DropMenu(c, more, "plan-line-"+l.ID, func(panel ui.Element) { panel.Width(planLineMenuWidth) }, func(m *kit.Menu) {
			i := slices.IndexFunc(content.Plan().Lines, func(x workspace.PlotLine) bool { return x.ID == l.ID })
			up, down := "左移", "右移"
			if p.mode == 1 {
				up, down = "上移", "下移"
			}
			m.Item("改名").Do(func() { p.startLine(l) })
			m.Item(up).Disabled(i == 0).Do(func() { p.change(content.MoveLine(l.ID, -1)) })
			m.Item(down).Disabled(i == len(content.Plan().Lines)-1).Do(func() { p.change(content.MoveLine(l.ID, 1)) })
			m.Separator()
			m.Item("删除情节线").Do(func() {
				p.change(content.DeleteLine(l.ID))
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

func (p *Page) startLine(l workspace.PlotLine) {
	p.FinishOutline()
	p.finishLine()
	p.lineID, p.lineName, p.lineHeld, p.lineFocus = l.ID, l.Name, false, true
}
func (p *Page) finishLine() {
	if p.lineID == "" {
		return
	}
	id, name := p.lineID, kit.Trim(p.lineName)
	p.lineID = ""
	if name != "" {
		_, err := p.Host.Content().SaveLine(workspace.PlotLine{ID: id, Name: name})
		p.change(err)
	}
}
func (p *Page) addLine(c *ui.Context) {
	pal := p.Host.Colors()
	b := ui.ButtonBase(c).Row().Height(kit.HControlSm).AlignItems(ui.Center).Gap(6).Padding(0, 8, 0, 4).Radius(kit.RControl).Label("新增情节线").Tooltip("多一条情节线")
	if b.Hovered() {
		b.Background(pal.Theme.SurfaceHover)
	}
	b.Children(func() {
		kit.Glyph(c, kit.IcPlus, kit.IconSm, pal.Theme.TextMuted)
		ui.Text(c, "新增情节线").FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted)
	})
	if b.Clicked() {
		p.finishLine()
		l, err := p.Host.Content().SaveLine(workspace.PlotLine{Name: "新情节线"})
		p.change(err)
		if err == nil {
			p.startLine(l)
			if p.mode == 0 {
				p.scroll[0].X = p.scroll[0].MaxX + planCardWidth
			} else {
				p.scroll[1].Y = p.scroll[1].MaxY + planRowMinHeight
			}
		}
	}
}

func (p *Page) gist(c *ui.Context, n *workspace.Node) {
	pal := p.Host.Colors()
	content := p.Host.Content()
	label, hint := "章纲", "写章纲…"
	if n.Folder {
		label, hint = "卷纲", "写卷纲…"
	}
	if p.outlineDraft.Node == n.ID {
		in := ui.TextAreaBase(c.Key("plan-outline-"+n.ID), &p.outlineDraft.Text).Label(label).Placeholder(hint).FillWidth().Height(kit.HControl*3).FontSize(kit.FsBody).LineHeight(kit.LhBody).Padding(6, 8).Border(1, pal.Theme.Border).Radius(kit.RControl).AutoFocus()
		now := time.Now()
		if p.outlineDraft.Change(now) {
			p.geometry = nil
		}
		if wait, due := p.outlineDraft.Waits(p.Host.Content().Timing.SaveAfter, now); due {
			if wait > 0 {
				c.After(wait)
			} else {
				p.saveOutline()
			}
		}
		if in.Shortcut(0, ui.KeyEscape) || p.outlineHeld && !in.Focused() {
			p.FinishOutline()
		} else if in.Focused() {
			p.outlineHeld = true
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
		e := p.text(c, text, false).MaxHeight(kit.FsBody * kit.LhBody * 3).Clip()
		if empty {
			e.TextColor(pal.Faint)
		}
	})
	if b.Clicked() {
		p.finishLine()
		p.FinishOutline()
		out := content.Outlines()[n.ID]
		p.outlineDraft = book.OutlineDraft{Node: n.ID, Text: out, Saved: out, Seen: out}
		p.outlineHeld = false
	}
}

// saveOutline saves the outline page's draft and shows it on the
// writing page when it is the chapter shown.
func (p *Page) saveOutline() {
	d := &p.outlineDraft
	if d.Node == "" {
		return
	}
	if err := d.Save(p.Host.Content()); err != nil {
		p.Host.Notices().Warn(err)
		return
	}
	p.Host.SyncPlanOutline(d.Node, d.Text)
}

// FinishOutline saves the 章纲 being typed and closes it.
func (p *Page) FinishOutline() {
	if p.outlineDraft.Node == "" {
		return
	}
	p.saveOutline()
	p.outlineDraft = book.OutlineDraft{}
	p.outlineHeld = false
}

func (p *Page) newCard(chapter, line, when, after string) {
	p.FinishOutline()
	p.finishLine()
	p.cardOpen, p.card, p.after, p.added, p.lastTitle = true, workspace.Card{Chapter: chapter, Line: line, When: when}, after, 0, ""
}

func (p *Page) openCard(e workspace.Card) { p.newCard("", "", "", ""); p.card = e }

func (p *Page) seeCard(id string) { p.seen, p.seenAt, p.reveal = id, time.Now(), true }

func (p *Page) saveCard(more bool) {
	if kit.Trim(p.card.Title) == "" {
		return
	}
	fresh := p.card.ID == ""
	e, err := p.Host.Content().SaveCard(p.card, p.after)
	p.change(err)
	if err != nil {
		return
	}
	if fresh {
		p.seeCard(e.ID)
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

// Dialog draws the card form while a card is open, over every page.
func (p *Page) Dialog(c *ui.Context) {
	pal := p.Host.Colors()
	status := p.Host.Notices()
	content := p.Host.Content()
	if !p.cardOpen {
		return
	}
	pal.Modal(c, &p.cardOpen, func() {
		ui.Column(c).Width(kit.DialogSmall).Padding(kit.DialogPad).Gap(16).Children(func() {
			title, about := "新卡片", "一张卡片记一件事。只有「发生了什么」要写，别的都可以以后再补。"
			if p.card.ID != "" {
				title, about = "修改卡片", ""
			} else if p.added > 0 {
				about = "已添加「" + p.lastTitle + "」，接着记下一张。"
			}
			pal.DialogHead(c, title, about)
			var enter bool
			kit.Labeled(c, "发生了什么", func() {
				in := ui.TextInput(c.Key(fmt.Sprintf("card-title-%d", p.added)), &p.card.Title).Label("事件").Placeholder("一句话，例如：林晚下井").FillWidth().AutoFocus()
				enter = in.Submitted()
			})
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).Children(func() {
					kit.Labeled(c, "写在哪一章", func() {
						names, ids := []string{"未放进章节"}, map[string]string{}
						chosen := names[0]
						for _, n := range p.chapters() {
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
						if pal.Select(c, &chosen, names).Label("卡片章节").FillWidth().MinWidth(0).Changed() {
							p.card.Chapter = ids[chosen]
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
							if l.ID == p.card.Line {
								chosen = name
							}
						}
						if pal.Select(c, &chosen, names).Label("卡片情节线").FillWidth().MinWidth(0).Changed() {
							p.card.Line = ids[chosen]
						}
					})
				})
			})
			kit.Labeled(c, "故事中的时间（选填）", func() {
				in := ui.TextInput(c, &p.card.When).Label("故事中的时间").Placeholder("例如：十月六日夜。不写也行，按先后排").FillWidth()
				enter = enter || in.Submitted()
			})
			kit.Labeled(c, "说明（选填）", func() { ui.TextArea(c, &p.card.About).Label("说明").FillWidth().Height(kit.HControl * 3) })
			save, more, drop := enter && kit.Trim(p.card.Title) != "", false, false
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if p.card.ID != "" {
					if pal.Ghost(c, "删除卡片", pal.Theme.Danger).Clicked() {
						drop = true
					}
					ui.Spacer(c)
				}
				if pal.Secondary(c, "取消").Label("取消卡片").Clicked() {
					p.cardOpen = false
				}
				if p.card.ID == "" {
					if pal.Secondary(c, "继续添加").Disabled(kit.Trim(p.card.Title) == "").Clicked() {
						more = true
					}
				}
				verb := "添加"
				if p.card.ID != "" {
					verb = "保存"
				}
				if pal.Primary(c, verb).Label("保存卡片").Disabled(kit.Trim(p.card.Title) == "").Clicked() {
					save = true
				}
			})
			if drop {
				p.change(content.DeleteCard(p.card.ID))
				if status.ErrorText() == "" {
					status.Did("已删除卡片「" + p.card.Title + "」")
					p.cardOpen = false
				}
			} else if save || more {
				p.saveCard(more)
			}
		})
	})
}
