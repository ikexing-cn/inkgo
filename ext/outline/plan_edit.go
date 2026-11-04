package outline

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"slices"
	"time"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/ext/outline/outlinetext"
	"writer/workspace"

	"writer/locales"
)

func (p *Page) line(c *ui.Context, l workspace.PlotLine) {
	status := p.Services.Notices.Notices()
	content := p.Services.Book.Content()
	t := p.Services.Palette.Colors().Theme
	if p.lineID == l.ID {
		in := ui.TextInputBase(c.Key("line-name-"+l.ID), &p.lineName).Label(outlinetext.PlanLineNameLabel()).Placeholder(outlinetext.PlanLineNameLabel()).FillWidth().Height(kit.HControlSm).FontSize(kit.FsBody).Padding(0, 6).Border(1, t.Border).Radius(kit.RControl).AutoFocus()
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
		b := ui.ButtonBase(c).Row().Justify(ui.Start).Grow(1).Shrink(1).MinWidth(0).Padding(2, 0).Radius(kit.RXs).Label(l.Name).Tooltip(outlinetext.PlanLineRenameHint())
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
		ui.Textf(c, "%d", n).FontSize(kit.FsNote).TextColor(p.Services.Palette.Colors().Faint)
		more := kit.IconButton(c, kit.IcMore, outlinetext.PlanLineActions(l.Name), ui.Color{})
		p.Services.Desktop.DropMenu(c, more, "plan-line-"+l.ID, func(panel ui.Element) { panel.Width(planLineMenuWidth) }, func(m *kit.Menu) {
			i := slices.IndexFunc(content.Plan().Lines, func(x workspace.PlotLine) bool { return x.ID == l.ID })
			up, down := outlinetext.PlanLineMoveLeft(), outlinetext.PlanLineMoveRight()
			if p.mode == 1 {
				up, down = outlinetext.CommonMoveUp(), outlinetext.CommonMoveDown()
			}
			m.Item(outlinetext.PlanLineRename()).Do(func() { p.startLine(l) })
			m.Item(up).Disabled(i == 0).Do(func() { p.change(content.MoveLine(l.ID, -1)) })
			m.Item(down).Disabled(i == len(content.Plan().Lines)-1).Do(func() { p.change(content.MoveLine(l.ID, 1)) })
			m.Separator()
			m.Item(outlinetext.PlanLineDelete()).Do(func() {
				p.change(content.DeleteLine(l.ID))
				if status.ErrorText() == "" {
					status.Did(outlinetext.PlanLineDeleted(l.Name))
					if n > 0 {
						status.Hint(outlinetext.PlanLineDeletedCards(n))
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
		_, err := p.Services.Book.Content().SaveLine(workspace.PlotLine{ID: id, Name: name})
		p.change(err)
	}
}

func (p *Page) addLine(c *ui.Context) {
	pal := p.Services.Palette.Colors()
	b := ui.ButtonBase(c).Row().Height(kit.HControlSm).AlignItems(ui.Center).Gap(6).Padding(0, 8, 0, 4).Radius(kit.RControl).Label(outlinetext.PlanLineAdd()).Tooltip(outlinetext.PlanLineAddHint())
	if b.Hovered() {
		b.Background(pal.Theme.SurfaceHover)
	}
	b.Children(func() {
		kit.Glyph(c, kit.IcPlus, kit.IconSm, pal.Theme.TextMuted)
		ui.Text(c, outlinetext.PlanLineAdd()).FontSize(kit.FsNote).TextColor(pal.Theme.TextMuted)
	})
	if b.Clicked() {
		p.finishLine()
		l, err := p.Services.Book.Content().SaveLine(workspace.PlotLine{Name: outlinetext.PlanLineNew()})
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
	pal := p.Services.Palette.Colors()
	content := p.Services.Book.Content()
	label, hint := outlinetext.PlanChapterOutline(), outlinetext.PlanWriteChapterOutline()
	hook := "章纲"
	if n.Folder {
		label, hint = outlinetext.PlanVolumeOutline(), outlinetext.PlanWriteVolumeOutline()
		hook = "卷纲"
	}
	if p.outlineDraft.Node == n.ID {
		in := ui.TextAreaBase(c.Key("plan-outline-"+n.ID), &p.outlineDraft.Text).Label(label).Placeholder(hint).FillWidth().Height(kit.HControl*3).FontSize(kit.FsBody).LineHeight(kit.LhBody).Padding(6, 8).Border(1, pal.Theme.Border).Radius(kit.RControl).AutoFocus()
		now := time.Now()
		if p.outlineDraft.Change(now) {
			p.geometry = nil
		}
		if wait, due := p.outlineDraft.Waits(p.Services.Book.Content().Timing.SaveAfter, now); due {
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
	b := ui.ButtonBase(c).FillWidth().Padding(2, 4).Radius(kit.RXs).Label(hook + "-" + n.ID).Tooltip(outlinetext.PlanClickEdit())
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
	if err := d.Save(p.Services.Book.Content()); err != nil {
		p.Services.Notices.Notices().Warn(err)
		return
	}
	if draft := p.drafts[d.Node]; draft != nil {
		draft.Text, draft.Saved, draft.Seen = d.Text, d.Text, d.Text
	}
}

// FinishOutline saves the 章纲 being typed and closes it.
func (p *Page) FinishOutline() {
	p.Services.Notices.Notices().Warn(p.SaveChapter())
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
	e, err := p.Services.Book.Content().SaveCard(p.card, p.after)
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
	pal := p.Services.Palette.Colors()
	status := p.Services.Notices.Notices()
	content := p.Services.Book.Content()
	if !p.cardOpen {
		return
	}
	pal.Modal(c, &p.cardOpen, func() {
		ui.Column(c).Width(kit.DialogSmall).Padding(kit.DialogPad).Gap(16).Children(func() {
			title, about := outlinetext.PlanCardNew(), outlinetext.PlanCardIntro()
			if p.card.ID != "" {
				title, about = outlinetext.PlanCardEdit(), ""
			} else if p.added > 0 {
				about = outlinetext.PlanCardAdded(p.lastTitle)
			}
			pal.DialogHead(c, title, about)
			var enter bool
			kit.Labeled(c, outlinetext.PlanCardWhat(), func() {
				in := ui.TextInput(c.Key(fmt.Sprintf("card-title-%d", p.added)), &p.card.Title).Label(outlinetext.PlanCardEvent()).Placeholder(outlinetext.PlanCardWhatHint()).FillWidth().AutoFocus()
				enter = in.Submitted()
			})
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).Children(func() {
					kit.Labeled(c, outlinetext.PlanCardChapter(), func() {
						names, ids := []string{outlinetext.PlanNoChapter()}, map[string]string{}
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
					kit.Labeled(c, outlinetext.PlanLine(), func() {
						names, ids := []string{outlinetext.PlanNoLine()}, map[string]string{}
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
			kit.Labeled(c, outlinetext.PlanCardWhen(), func() {
				in := ui.TextInput(c, &p.card.When).Label(outlinetext.PlanCardWhenLabel()).Placeholder(outlinetext.PlanCardWhenHint()).FillWidth()
				enter = enter || in.Submitted()
			})
			kit.Labeled(c, outlinetext.PlanCardAbout(), func() {
				ui.TextArea(c, &p.card.About).Label(outlinetext.PlanCardNotes()).FillWidth().Height(kit.HControl * 3)
			})
			save, more, drop := enter && kit.Trim(p.card.Title) != "", false, false
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if p.card.ID != "" {
					if pal.Ghost(c, outlinetext.PlanCardDelete(), pal.Theme.Danger).Clicked() {
						drop = true
					}
					ui.Spacer(c)
				}
				if pal.Secondary(c, locales.CommonCancel()).Label("取消卡片").Clicked() {
					p.cardOpen = false
				}
				if p.card.ID == "" {
					if pal.Secondary(c, outlinetext.PlanCardAddMore()).Disabled(kit.Trim(p.card.Title) == "").Clicked() {
						more = true
					}
				}
				verb := outlinetext.PlanAddCard()
				if p.card.ID != "" {
					verb = locales.CommonSave()
				}
				if pal.Primary(c, verb).Label("保存卡片").Disabled(kit.Trim(p.card.Title) == "").Clicked() {
					save = true
				}
			})
			if drop {
				p.change(content.DeleteCard(p.card.ID))
				if status.ErrorText() == "" {
					status.Did(outlinetext.PlanCardDeleted(p.card.Title))
					p.cardOpen = false
				}
			} else if save || more {
				p.saveCard(more)
			}
		})
	})
}
