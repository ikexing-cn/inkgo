package desk

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/tokens"
	"writer/workspace"
)

// recordFeatures are the features of the AI record as the author reads
// them, in the order the settings list them. 此前 is what versions before
// the record counted, by model only.
var recordFeatures = []struct{ id, name string }{
	{workspace.FeatureTalk, "讨论"},
	{workspace.FeatureDraft, "起草"},
	{workspace.FeatureRewrite, "改写"},
	{workspace.FeatureContinue, "续写"},
	{workspace.FeatureCheck, "检查"},
	{workspace.FeatureBefore, "此前"},
}

func featureName(id string) string {
	for _, f := range recordFeatures {
		if f.id == id {
			return f.name
		}
	}
	return id
}

// costText says what costs cost in their currencies, estimated: under a
// cent as such. With none priced it says nothing.
func costText(costs map[string]float64) string {
	var out []string
	for _, cur := range slices.Sorted(maps.Keys(costs)) {
		if v := costs[cur]; v < 0.01 {
			out = append(out, "不到 "+cur+"0.01")
		} else {
			out = append(out, fmt.Sprintf("约 %s%.2f", cur, v))
		}
	}
	return strings.Join(out, " + ")
}

// costOf is what the records cost, by currency, those priced.
func costOf(rs ...workspace.Record) map[string]float64 {
	costs := map[string]float64{}
	for _, r := range rs {
		if r.Currency != "" {
			costs[r.Currency] += r.Cost
		}
	}
	return costs
}

// spent is what the work has spent on the AI, a line for each feature: the
// requests, their tokens as the providers counted them and the cost by the
// prices built in. It is only to look at; nothing clears it. Under it, the
// way to every request.
func (a *App) spent(c *ui.Context) {
	t := a.pal.theme
	a.subhead(c, "本作品用量", "按内置价格估算，以服务商账单为准。")
	if len(a.bk.records) == 0 {
		ui.Text(c, "尚未使用 AI。").FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody)
		return
	}
	all := 0
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		for _, f := range recordFeatures {
			var rows []workspace.Record
			n := 0
			var u workspace.Usage
			for _, r := range a.bk.records {
				if r.Feature == f.id {
					rows = append(rows, r)
					n += r.Count()
					u = u.Add(r.Usage)
				}
			}
			if len(rows) == 0 {
				continue
			}
			all += n
			used := fmt.Sprintf("%d 次请求 · 输入 %s", n, tokens.Round(u.Input))
			if u.Input > 0 {
				used += fmt.Sprintf("，缓存命中 %d%%", int(math.Round(float64(u.CacheRead)*100/float64(u.Input))))
			}
			used += " · 输出 " + tokens.Round(u.Output)
			cost := costText(costOf(rows...))
			if cost == "" {
				cost = "无价格"
			}
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, f.name).FontSize(fsControl).Width(150).Shrink(0).SingleLine()
				ui.Text(c, used).FontSize(fsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
				ui.Text(c, cost).FontSize(fsControl).Shrink(0)
			})
		}
	})
	ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
		if a.pal.ghost(c, fmt.Sprintf("查看全部记录（%d 次请求）", all), ui.Color{}).Clicked() {
			a.bk.log, a.bk.logOnly, a.bk.logOpen = true, 0, -1
		}
	})
}

// recordsView is the AI record: a line for each request, the newest first,
// those that failed or were cancelled too, filtered by feature. A line
// opens on the model, the parts sent and what the provider counted. The
// text sent is not kept, so not shown.
func (a *App) recordsView(c *ui.Context) {
	t := a.pal.theme
	names := []string{"全部"}
	var ids []string
	for _, f := range recordFeatures {
		if f.id != workspace.FeatureBefore || slices.ContainsFunc(a.bk.records, func(r workspace.Record) bool { return r.Feature == f.id }) {
			names, ids = append(names, f.name), append(ids, f.id)
		}
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
		if a.pal.ghost(c, "‹ 本作品用量", ui.Color{}).Clicked() {
			a.bk.log = false
		}
		ui.Spacer(c)
		was := a.bk.logOnly
		a.pal.segments(c, &a.bk.logOnly, "按功能筛选", names)
		if a.bk.logOnly != was {
			a.bk.logOpen = -1
		}
	})
	now := time.Now()
	ui.Column(c).FillWidth().Children(func() {
		shown := 0
		for i := len(a.bk.records) - 1; i >= 0; i-- {
			r := a.bk.records[i]
			if a.bk.logOnly > 0 && r.Feature != ids[a.bk.logOnly-1] {
				continue
			}
			shown++
			a.pal.hairline(c)
			open := a.bk.logOpen == i
			at, where, end, endColor := "此前", r.Model[strings.LastIndex(r.Model, "/")+1:], "", t.TextMuted
			if r.Feature != workspace.FeatureBefore {
				at, where = when(r.At, now), featureName(r.Feature)+" · "+a.recordWhere(r)
				switch r.Outcome {
				case workspace.OutcomeDone:
					end, endColor = "完成", t.Text
				case workspace.OutcomeCancelled:
					end = "已取消"
				default:
					end, endColor = "失败："+r.Error, t.Danger
				}
			} else {
				where = fmt.Sprintf("共 %d 次请求 · %s", r.Requests, where)
			}
			cost := costText(costOf(r))
			if cost == "" {
				cost = "—"
			}
			row := ui.ButtonBase(c).Row().FillWidth().MinHeight(hRow+3).Gap(12).AlignItems(ui.Center).Padding(0, 6).Radius(rXs).Label("记录-" + at + " " + where)
			if row.Hovered() {
				row.Background(t.SurfaceHover)
			}
			row.Children(func() {
				ui.Text(c, at).FontSize(fsControl).TextColor(t.TextMuted).Width(92).Shrink(0).SingleLine()
				ui.Text(c, where).FontSize(fsControl).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
				ui.Text(c, tokens.Round(r.Usage.Input+r.Usage.Output)).FontSize(fsControl).TextColor(t.TextMuted).Width(48).Shrink(0).SingleLine()
				ui.Text(c, cost).FontSize(fsControl).TextColor(t.TextMuted).Width(76).Shrink(0).SingleLine()
				ui.Text(c, end).FontSize(fsControl).TextColor(endColor).Width(120).Shrink(0).SingleLine().Ellipsis("…")
				chevron := icChevron
				if open {
					chevron = icChevronDown
				}
				glyph(c, chevron, iconXs, a.pal.faint)
			})
			if row.Clicked() {
				a.bk.logOpen = i
				if open {
					a.bk.logOpen = -1
				}
			}
			if open {
				a.recordMore(c, r)
			}
		}
		if shown == 0 {
			ui.Text(c, "没有这类请求。").FontSize(fsNote).TextColor(t.TextMuted).Padding(12, 0)
		}
	})
}

// recordMore is a line of the AI record opened: the model and its effort,
// the parts sent with their tokens as estimated, and what the provider
// counted.
func (a *App) recordMore(c *ui.Context, r workspace.Record) {
	t := a.pal.theme
	note := func(s string) {
		ui.Text(c, s).FontSize(fsNote).TextColor(t.TextMuted).LineHeight(lhBody).Selectable().FillWidth()
	}
	ui.Column(c).FillWidth().Padding(0, 6, 10, 6+92+12).Gap(2).Children(func() {
		switch {
		case r.Feature == workspace.FeatureBefore:
			note(a.replyLabel(r.Model, ""))
			note("此版本以前记下的用量，不分功能和章节。")
		case r.Effort == "none":
			note(a.replyLabel(r.Model, "") + " · 不思考")
		default:
			note(a.replyLabel(r.Model, "") + " · " + a.ai.EffortLabel(r.Effort))
		}
		if len(r.Parts) > 0 {
			var parts []string
			for _, p := range r.Parts {
				name := partNames[p.Name]
				switch {
				case p.Name == workspace.PartSetting:
					name = "设定"
				case p.Name == workspace.PartProse && r.Feature != workspace.FeatureTalk:
					name = "本章正文"
				case name == "":
					name = p.Name
				}
				parts = append(parts, name+" "+tokens.Round(p.Tokens))
			}
			note("发送：" + strings.Join(parts, " · "))
		}
		used := "实际用量：输入 " + tokens.Round(r.Usage.Input)
		if r.Usage.CacheRead > 0 {
			used += "，其中缓存命中 " + tokens.Round(r.Usage.CacheRead)
		}
		note(used + "；输出 " + tokens.Round(r.Usage.Output))
		if r.Outcome == workspace.OutcomeFailed {
			ui.Text(c, r.Error).FontSize(fsNote).TextColor(t.Danger).LineHeight(lhBody).Selectable().FillWidth()
		}
	})
}

// recordWhere names where a request of the AI record was for: its talk
// by its first words, or its chapter by title, as the work has them now.
func (a *App) recordWhere(r workspace.Record) string {
	if r.Talk != "" {
		if title, ok := a.titles[r.Talk]; ok {
			return title
		}
		return "已删除的讨论"
	}
	if n, ok := a.write.tree.Node(r.Chapter); ok {
		return n.Title
	}
	return "已删除的章节"
}
