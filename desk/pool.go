package desk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/discussion"
	"writer/workspace"
)

// filters are the pool's tabs; the last one shows every card.
var filters = []struct {
	name   string
	status workspace.Status
}{{"待处理", workspace.Unprocessed}, {"留下", workspace.Kept}, {"待定", workspace.Deferred}, {"放弃", workspace.Rejected}, {"全部", ""}}

var extras = []string{"不限", "有冲突", "现实事实", "已采用", "被取代"}

const undoFor = 8 * time.Second

func statusLabel(s workspace.Status) string {
	switch s {
	case workspace.Unprocessed:
		return "待处理"
	case workspace.Kept:
		return "留下"
	case workspace.Deferred:
		return "待定"
	case workspace.Rejected:
		return "放弃"
	case workspace.Adopted:
		return "已采用"
	case workspace.Superseded:
		return "被取代"
	}
	return string(s)
}

func (a *App) pool(c *ui.Context) {
	t := a.pal.theme
	counts := map[workspace.Status]int{}
	for _, i := range a.ideas {
		counts[i.Status]++
	}
	ui.Row(c).FillWidth().Height(34).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "思路池").FontSize(15).FontWeight(600)
		ui.Textf(c, "%d", len(a.ideas)).FontSize(12.5).TextColor(a.pal.faint)
		ui.Spacer(c)
		if n := discussion.ConflictCount(a.ideas); n > 0 {
			b := ui.ButtonBase(c).Row().Gap(5).AlignItems(ui.Center).Padding(3, 9).Radius(999).Background(t.Danger.Alpha(0.1)).Label(fmt.Sprintf("%d 处冲突", n))
			b.Children(func() {
				glyph(c, icAlert, 12, t.Danger)
				ui.Textf(c, "%d 处冲突", n).FontSize(12).TextColor(t.Danger)
			})
			if b.Clicked() {
				a.filter, a.extra, a.turn = len(filters)-1, "有冲突", ""
			}
		}
	})
	// Tabs: text with a count, an ink underline under the one chosen.
	ui.Column(c).FillWidth().Gap(0).Children(func() {
		ui.Row(c).FillWidth().Gap(14).Children(func() {
			for n, f := range filters {
				on := a.filter == n
				tab := ui.ButtonBase(c).Column().Gap(7).Padding(4, 0, 0, 0).Label("筛选-" + f.name).AlignItems(ui.Center)
				tab.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						fg := t.TextMuted
						if on || tab.Hovered() {
							fg = t.Text
						}
						weight := 400
						if on {
							weight = 600
						}
						ui.Text(c, f.name).FontSize(13).FontWeight(weight).TextColor(fg)
						if f.status != "" && counts[f.status] > 0 {
							ui.Textf(c, "%d", counts[f.status]).FontSize(11.5).TextColor(a.pal.faint)
						}
					})
					bar := ui.Box(c).FillWidth().Height(2).Radius(1)
					if on {
						bar.Background(a.pal.ink)
					}
				})
				if tab.Clicked() {
					a.filter, a.turn = n, ""
				}
			}
		})
		a.pal.hairline(c)
	})
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		s := ui.Row(c).Grow(1).Shrink(1).Gap(7).AlignItems(ui.Center).Padding(0, 10).Height(32).Radius(8).Background(a.pal.raised).Border(1, t.Border)
		s.Children(func() {
			glyph(c, icSearch, 14, a.pal.faint)
			ui.TextInputBase(c, &a.search).Label("搜索思路").Placeholder("搜索思路").FontSize(13).Grow(1)
		})
		if s.FocusWithin() {
			s.BorderColor(t.TextMuted.Alpha(0.55))
		}
		if ui.Select(c, &a.extra, extras).Label("更多筛选").Width(104).Changed() && a.extra != "不限" {
			a.filter = len(filters) - 1
		}
	})
	if a.turn != "" {
		ui.Row(c).FillWidth().Padding(5, 6, 5, 10).Radius(8).Gap(8).AlignItems(ui.Center).Background(a.pal.brand.Alpha(0.08)).Children(func() {
			glyph(c, icSparkle, 13, a.pal.brand)
			ui.Text(c, "只看这一轮整理出的思路").FontSize(12.5).TextColor(a.pal.brand).Grow(1)
			if ghost(c, "显示全部", a.pal.brand).Clicked() {
				a.turn = ""
			}
		})
	}
	a.undoBar(c)
	ui.Scroll(c).Key("pool").Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(8).Padding(2, 4, 8, 0).Children(func() {
			visible := a.visibleIdeas()
			if len(visible) == 0 {
				a.emptyPool(c)
			}
			for _, idea := range visible {
				a.card(c, idea)
			}
		})
	})
	ui.Row(c).FillWidth().Gap(10).Justify(ui.Center).Children(func() {
		for _, k := range [][2]string{{"1", "留下"}, {"2", "放弃"}, {"3", "待定"}, {"Tab", "下一张"}} {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Padding(0, 5).Radius(4).Border(1, t.Border).Background(a.pal.raised).Children(func() {
					ui.Text(c, k[0]).FontSize(10.5).TextColor(t.TextMuted)
				})
				ui.Text(c, k[1]).FontSize(11.5).TextColor(a.pal.faint)
			})
		}
	})
}

func (a *App) emptyPool(c *ui.Context) {
	t := a.pal.theme
	title, hint := "这里还空着", "讨论之后，你和 AI 提到的思路会出现在这里。\n还没决定的思路不会被 AI 当作依据。"
	switch {
	case len(a.ideas) == 0:
	case a.search != "" || a.turn != "" || (a.extra != "" && a.extra != "不限"):
		title, hint = "没有符合条件的思路", "换个关键词，或清除筛选。"
	case filters[a.filter].status == workspace.Unprocessed:
		title, hint = "都处理完了", "新的讨论会带来新的思路。留下的思路会在之后的讨论里交给 AI 参考。"
	default:
		title, hint = "这一栏还没有思路", ""
	}
	ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(56, 16).Gap(8).Children(func() {
		ui.Box(c).Size(36, 36).Radius(10).Background(t.SurfacePressed).Center().Children(func() {
			glyph(c, icSparkle, 18, a.pal.faint)
		})
		ui.Text(c, title).FontSize(14).FontWeight(600)
		if hint != "" {
			ui.Text(c, hint).FontSize(12.5).TextColor(t.TextMuted).TextAlign(ui.Center).LineHeight(1.6)
		}
	})
}

func (a *App) visibleIdeas() []workspace.Idea {
	var turnIDs map[string]bool
	if a.turn != "" {
		turnIDs = map[string]bool{}
		for _, e := range a.events {
			if e.ID == a.turn {
				for _, i := range a.turnIdeas(e) {
					turnIDs[i.ID] = true
				}
			}
		}
	}
	want := filters[min(a.filter, len(filters)-1)].status
	q := strings.ToLower(trim(a.search))
	var found []workspace.Idea
	for _, idea := range a.ideas {
		switch {
		case turnIDs != nil && !turnIDs[idea.ID]:
		case q != "" && !strings.Contains(strings.ToLower(idea.Text), q):
		case want != "" && idea.Status != want:
		case a.extra == "有冲突" && !(idea.Status == workspace.Kept && len(idea.Conflicts) > 0):
		case a.extra == "现实事实" && !idea.Fact:
		case a.extra == "已采用" && idea.Status != workspace.Adopted:
		case a.extra == "被取代" && idea.Status != workspace.Superseded:
		default:
			found = append(found, idea)
		}
	}
	return found
}

// mark records the author's decision, then moves the focus to the next card
// so a queue of cards can be worked through from the keyboard.
func (a *App) mark(c *ui.Context, i workspace.Idea, status workspace.Status, reason string) {
	a.markWith(c, i, status, reason, true)
}

func (a *App) markWith(c *ui.Context, i workspace.Idea, status workspace.Status, reason string, remember bool) {
	if a.busy != "" || i.Status == status {
		return
	}
	// The decision needs no model; only the conflict check after a keep does.
	eng := &discussion.Engine{Store: a.Store}
	if status == workspace.Kept {
		eng, _ = a.engine(c)
	}
	next := a.nextCard(i.ID)
	from, id := i.Status, i.ID
	a.run(c, "正在保存你的决定…", func(ctx context.Context) (func(), error) {
		err := eng.Mark(ctx, id, status, reason)
		saved := err == nil
		if !saved {
			if now, e := a.Store.Idea(id); e == nil && now.Status == status {
				saved = true
			}
		}
		return func() {
			if !saved {
				return
			}
			a.focusCard = next
			if remember {
				a.last = &undo{id: id, text: i.Text, from: from, to: status, at: time.Now()}
			} else {
				a.last = nil
			}
		}, err
	})
}

// nextCard is the card after id among those shown, or the one before it.
func (a *App) nextCard(id string) string {
	visible := a.visibleIdeas()
	for n, v := range visible {
		if v.ID != id {
			continue
		}
		if n+1 < len(visible) {
			return visible[n+1].ID
		}
		if n > 0 {
			return visible[n-1].ID
		}
	}
	return ""
}

// undoBar offers to take back the last decision for a few seconds. A rejected
// card can only come back as deferred, which the button says.
func (a *App) undoBar(c *ui.Context) {
	u := a.last
	if u == nil {
		return
	}
	left := undoFor - c.Now().Sub(u.at)
	if left <= 0 {
		a.last = nil
		return
	}
	c.After(left)
	back, label := u.from, "撤销"
	if u.to == workspace.Rejected && u.from != workspace.Deferred {
		back, label = workspace.Deferred, "改回待定"
	}
	ui.Row(c).FillWidth().Padding(5, 6, 5, 12).Radius(9).Gap(6).AlignItems(ui.Center).Background(a.pal.ink).Shadow(0, 4, 14, 0, ui.RGBA(0, 0, 0, 0.12)).Children(func() {
		glyph(c, icCheck, 13, a.pal.onInk)
		ui.Textf(c, "已%s「%s」", statusLabel(u.to), clip(u.text, 12)).FontSize(12.5).TextColor(a.pal.onInk).SingleLine().Ellipsis("…").Grow(1).Shrink(1)
		if ghost(c, label, a.pal.onInk).Disabled(a.busy != "").Clicked() {
			for _, i := range a.ideas {
				if i.ID == u.id {
					a.markWith(c, i, back, "撤销", false)
				}
			}
		}
		if iconButton(c, icX, "关闭撤销", a.pal.onInk.Alpha(0.7)).Clicked() {
			a.last = nil
		}
	})
}
