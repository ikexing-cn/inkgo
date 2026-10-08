package desk

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

func (a *App) ideaText(id string) string {
	for _, i := range a.ideas {
		if i.ID == id {
			return clip(i.Text, 18)
		}
	}
	return id
}

func (a *App) selectIdea(i workspace.Idea) {
	a.selected = i.ID
	a.editText = i.Text
	a.editCategory = i.Category
	a.fact = i.Fact
	a.verified = i.Verified
	a.factSource = i.FactSource
}

func (a *App) startReject(i workspace.Idea) {
	a.selectIdea(i)
	a.rejectID = i.ID
	a.reason = ""
}

func (a *App) card(c *ui.Context, i workspace.Idea) {
	t := a.pal.theme
	tint := a.pal.statusColor(i.Status)
	chosen := a.selected == i.ID
	card := ui.Column(c).Key(i.ID).FillWidth().Radius(12).Background(a.pal.raised).Border(1, t.Border).Focusable().Label("思路-" + i.ID)
	hover := card.Hovered()
	if chosen {
		card.BorderColor(a.pal.ink.Alpha(0.55)).Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, 0.07))
	} else if hover {
		card.BorderColor(t.TextMuted.Alpha(0.35)).Shadow(0, 2, 10, 0, ui.RGBA(0, 0, 0, 0.04))
	}
	if i.Status == workspace.Rejected || i.Status == workspace.Superseded {
		card.Opacity(0.72)
	}
	if a.focusCard == i.ID {
		card.Focus().ScrollIntoView()
		a.selectIdea(i)
		a.focusCard = ""
	}
	if card.Clicked() && !chosen {
		a.selectIdea(i)
	}
	card.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputKeyDown || ev.Mods != 0 || a.busy != "" {
			return false
		}
		// HandleInput only runs while the card itself has the focus, never
		// while one of its fields is being edited.
		switch ev.Key {
		case ui.Key1:
			if i.Status != workspace.Rejected && i.Status != workspace.Adopted {
				a.mark(c, i, workspace.Kept, "")
			}
		case ui.Key2:
			if i.Status != workspace.Rejected && i.Status != workspace.Adopted {
				a.startReject(i)
			}
		case ui.Key3:
			if i.Status != workspace.Adopted {
				a.mark(c, i, workspace.Deferred, "")
			}
		case ui.KeyTab:
			if next := a.nextCard(i.ID); next != "" {
				a.focusCard = next
			}
		default:
			return false
		}
		return true
	})
	card.Children(func() {
		ui.Column(c).FillWidth().Padding(12, 14, 10, 14).Gap(9).Children(func() {
			a.cardBody(c, i, tint)
			if a.rejectID == i.ID {
				a.rejection(c, i)
			} else {
				a.cardActions(c, i)
			}
			if a.expanded == i.ID {
				a.ideaDetails(c, i)
			}
		})
	})
}

// decide is a compact decision button: an icon and a word, tinted when it is
// the card's current state.
func (a *App) decide(c *ui.Context, s *ui.SVG, label string, color ui.Color, current bool) *ui.Element {
	t := a.pal.theme
	b := ui.ButtonBase(c).Row().Gap(5).AlignItems(ui.Center).Padding(4, 10, 4, 8).Radius(7).Label(label).Shrink(0)
	fg, bg := t.TextMuted, ui.Color{}
	switch {
	case current:
		fg, bg = color, color.Alpha(0.12)
	case b.Hovered():
		fg, bg = color, color.Alpha(0.08)
	}
	if bg.A != 0 {
		b.Background(bg)
	}
	if b.IsDisabled() && !current {
		b.Opacity(0.4)
	}
	b.Children(func() {
		glyph(c, s, 14, fg)
		ui.Text(c, label).FontSize(12.5).TextColor(fg).FontWeight(map[bool]int{true: 600, false: 500}[current])
	})
	return b
}

func (a *App) cardBody(c *ui.Context, i workspace.Idea, tint ui.Color) {
	t := a.pal.theme
	ui.Row(c).FillWidth().Gap(7).AlignItems(ui.Center).Children(func() {
		dot(c, tint, 7)
		ui.Text(c, statusLabel(i.Status)).FontSize(12).FontWeight(600).TextColor(tint)
		ui.Text(c, "·").FontSize(12).TextColor(a.pal.faint)
		ui.Text(c, i.Category).FontSize(12).TextColor(t.TextMuted)
		ui.Text(c, "·").FontSize(12).TextColor(a.pal.faint)
		who := "AI 提出"
		if i.Source == "human" {
			who = "你提出"
		}
		ui.Text(c, who).FontSize(12).TextColor(a.pal.faint)
		ui.Spacer(c)
		if i.Fact {
			if i.Verified {
				pill(c, "事实 · 已核实", t.Success)
			} else {
				pill(c, "事实 · 未核实", t.Warning)
			}
		}
	})
	ui.Text(c, i.Text).FillWidth().FontSize(14).LineHeight(1.65)
	note := func(s *ui.SVG, color ui.Color, text string) {
		ui.Row(c).FillWidth().Gap(7).Padding(6, 9).Radius(7).Background(color.Alpha(0.07)).Children(func() {
			glyph(c, s, 13, color)
			ui.Text(c, text).FontSize(12).TextColor(color.Mix(t.Text, 0.3)).LineHeight(1.5).Grow(1).Shrink(1)
		})
	}
	if i.Status == workspace.Kept {
		for _, id := range i.Conflicts {
			note(icAlert, t.Danger, "和「"+a.ideaText(id)+"」矛盾")
		}
	}
	for _, sim := range i.Similar {
		line := "和「" + a.ideaText(sim.ID) + "」相近"
		if sim.Difference != "" {
			line += "：" + sim.Difference
		}
		note(icSparkle, t.Warning, line)
	}
	if i.Status == workspace.Rejected {
		if n := len(i.History); n > 0 && i.History[n-1].Reason != "" {
			ui.Text(c, "放弃理由："+i.History[n-1].Reason).FontSize(12).TextColor(t.TextMuted)
		}
	}
}

func (a *App) cardActions(c *ui.Context, i workspace.Idea) {
	t := a.pal.theme
	busy := a.busy != ""
	ui.Row(c).FillWidth().Gap(2).AlignItems(ui.Center).Margin(0, -6).Children(func() {
		switch i.Status {
		case workspace.Rejected:
			if a.decide(c, icRetry, "捞回待定", t.Warning, false).Disabled(busy).Clicked() {
				a.mark(c, i, workspace.Deferred, "")
			}
		case workspace.Adopted:
			ui.Text(c, "已写进设定，受保护").FontSize(12).TextColor(t.TextMuted).Padding(0, 6)
		default:
			if a.decide(c, icCheck, "留下", t.Success, i.Status == workspace.Kept).Label("留下-" + i.ID).Disabled(busy || i.Status == workspace.Kept).Clicked() {
				a.mark(c, i, workspace.Kept, "")
			}
			if a.decide(c, icX, "放弃", t.Danger, false).Disabled(busy).Clicked() {
				a.startReject(i)
			}
			if a.decide(c, icClock, "待定", t.Warning, i.Status == workspace.Deferred).Disabled(busy || i.Status == workspace.Deferred).Clicked() {
				a.mark(c, i, workspace.Deferred, "")
			}
		}
		ui.Spacer(c)
		label := "详情"
		if a.expanded == i.ID {
			label = "收起"
		}
		if ghost(c, label, ui.Color{}).Label("详情-" + i.ID).Clicked() {
			if a.expanded == i.ID {
				a.expanded = ""
			} else {
				a.selectIdea(i)
				a.expanded = i.ID
			}
		}
	})
}

// rejection asks for an optional reason in place, where the author is looking.
func (a *App) rejection(c *ui.Context, i workspace.Idea) {
	t := a.pal.theme
	ui.Column(c).Key("rejection").FillWidth().Gap(8).Padding(10, 12).Radius(9).Background(a.pal.side).Border(1, a.pal.line).Children(func() {
		ui.Text(c, "为什么放弃？写一句能帮 AI 以后避开，留空也行").FontSize(12.5).TextColor(t.TextMuted)
		done := ui.TextInput(c, &a.reason).Label("放弃理由").Placeholder("例如：和故事基调不符").AutoFocus().FillWidth().Submitted()
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			ui.Spacer(c)
			if ghost(c, "取消", ui.Color{}).Clicked() || c.Shortcut(0, ui.KeyEscape) {
				a.rejectID = ""
			}
			done = action(c, "确认放弃", t.Danger, true).Disabled(a.busy != "").Clicked() || done
		})
		if done && a.busy == "" {
			a.rejectID = ""
			a.mark(c, i, workspace.Rejected, trim(a.reason))
		}
	})
}

func (a *App) ideaDetails(c *ui.Context, i workspace.Idea) {
	t := a.pal.theme
	busy := a.busy != ""
	locked := busy || i.Status == workspace.Adopted
	ui.Column(c).FillWidth().Gap(10).Padding(4, 0, 2, 0).Children(func() {
		a.pal.hairline(c)
		ui.TextArea(c, &a.editText).Label("思路内容").FillWidth().Height(84).FontSize(13.5).LineHeight(1.6).Disabled(locked)
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "分类").FontSize(12.5).TextColor(t.TextMuted)
			ui.Select(c, &a.editCategory, a.categoryNames()).Label("思路分类").Width(140).Disabled(locked)
			ui.Checkbox(c, &a.fact, "涉及现实事实").Disabled(locked)
		})
		if a.fact {
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Checkbox(c, &a.verified, "我已核实").Disabled(locked)
				ui.TextInput(c, &a.factSource).Label("核实来源").Placeholder("来源或链接").Grow(1).Disabled(locked)
			})
		}
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			if a.pal.primary(c, "保存修改").Disabled(locked).Clicked() {
				a.saveDetails(c, i)
			}
			if i.Status == workspace.Kept && a.pal.secondary(c, "检查冲突").Disabled(busy).Clicked() {
				if err := a.modelsReady(); err != nil {
					a.setError(err)
				} else {
					a.run(c, "正在检查和其他留下的思路是否矛盾…", func(ctx context.Context) (func(), error) {
						return nil, a.Engine.CheckConflicts(ctx, i.ID)
					})
				}
			}
			ui.Spacer(c)
			if ghost(c, "查看原文", ui.Color{}).Disabled(busy || i.Origin.Message == "").Clicked() {
				a.showOrigin(i)
			}
		})
		for n := len(i.History) - 1; n >= 0; n-- {
			h := i.History[n]
			line := fmt.Sprintf("%s  %s → %s", h.At.Local().Format("01-02 15:04"), statusLabel(h.From), statusLabel(h.To))
			if h.Reason != "" {
				line += "  " + h.Reason
			}
			ui.Text(c, line).FontSize(11.5).TextColor(a.pal.faint)
		}
	})
}

func (a *App) saveDetails(c *ui.Context, i workspace.Idea) {
	i.Text = a.editText
	i.Category = a.editCategory
	i.Fact = a.fact
	i.Verified = a.fact && a.verified
	i.FactSource = a.factSource
	if i.Status != workspace.Kept {
		err := a.Store.SaveIdea(i)
		if err == nil {
			a.notice = "修改已保存"
			err = a.refresh()
		}
		a.setError(err)
		return
	}
	if err := a.modelsReady(); err != nil {
		a.Engine.Book = nil
	}
	a.run(c, "正在保存，并检查是否与其他思路矛盾…", func(ctx context.Context) (func(), error) {
		if err := a.Store.SaveIdea(i); err != nil {
			return nil, err
		}
		if err := a.Engine.CheckConflicts(ctx, i.ID); err != nil {
			return nil, fmt.Errorf("修改已保存，冲突检查失败：%w", err)
		}
		return func() { a.notice = "修改已保存" }, nil
	})
}

// showOrigin opens the message a card came from with its words selected.
func (a *App) showOrigin(i workspace.Idea) {
	events, err := a.Store.Events(i.Origin.Session)
	if err != nil {
		a.setError(err)
		return
	}
	for _, m := range events {
		if m.ID == i.Origin.Message {
			if i.Origin.Session != a.session {
				a.switchSession(i.Origin.Session)
			}
			a.openSource(m, i.Origin.Span.Start, i.Origin.Span.End)
			return
		}
	}
	a.setError(fmt.Errorf("找不到这条思路的原话"))
}
