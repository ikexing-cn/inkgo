package desk

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/tokens"
	"writer/workspace"
)

// recordView owns one work's AI usage snapshot and its request list.
// Loading invalidates row indices; only leaving the work resets the view.
type recordView struct {
	records  []workspace.Record
	showing  bool
	only     int // 0 for all, otherwise an index into recordFeatures plus one
	expanded int // record index, or -1
	rows     []int
	rowsFor  int
	list     ui.ListState
}

func (v *recordView) reset() { *v = recordView{expanded: -1} }

func (v *recordView) load(records []workspace.Record) {
	v.records, v.rows = records, nil
}

func (v *recordView) show() { v.showing, v.only, v.expanded = true, 0, -1 }

// recordFeatures are the features of the AI record as the author reads
// them, in the order the settings list them.
var recordFeatures = []struct{ id, name string }{
	{workspace.FeatureTalk, "讨论"},
	{workspace.FeatureDraft, "起草"},
	{workspace.FeatureRewrite, "改写"},
	{workspace.FeatureContinue, "续写"},
	{workspace.FeatureCheck, "检查"},
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
	t := a.pal.Theme
	a.pal.Subhead(c, "本作品用量", "")
	if len(a.record.records) == 0 {
		ui.Text(c, "尚未使用 AI。").FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody)
		return
	}
	all := 0
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		for _, f := range recordFeatures {
			var rows []workspace.Record
			var u workspace.Usage
			for _, r := range a.record.records {
				if r.Feature == f.id {
					rows = append(rows, r)
					u = u.Add(r.Usage)
				}
			}
			n := len(rows)
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
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, f.name).FontSize(kit.FsControl).Width(120).Shrink(0).SingleLine()
				ui.Text(c, used).FontSize(kit.FsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
				ui.Text(c, cost).FontSize(kit.FsControl).Shrink(0)
			})
		}
	})
	ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
		if a.pal.Ghost(c, fmt.Sprintf("查看全部记录（%d 次请求）", all), ui.Color{}).Clicked() {
			a.record.show()
		}
	})
}

// recordsView is the AI record: a line for each request, the newest first,
// those that failed or were cancelled too, filtered by feature. A line
// opens on the model, the parts sent and what the provider counted. The
// text sent is not kept, so not shown. The head stays above the lines as
// they scroll.
func (a *App) recordsView(c *ui.Context) {
	t := a.pal.Theme
	names := []string{"全部"}
	for _, f := range recordFeatures {
		names = append(names, f.name)
	}
	ui.Column(c).FillWidth().Grow(1).Shrink(1).Gap(12).Children(func() {
		ui.Row(c).FillWidth().Padding(12, 56, 0, 32).Shrink(0).AlignItems(ui.Center).Gap(12).Children(func() {
			if a.pal.Ghost(c, "‹ 本作品用量", ui.Color{}).Clicked() {
				a.record.showing = false
			}
			ui.Spacer(c)
			was := a.record.only
			a.pal.Segments(c, &a.record.only, "按功能筛选", names)
			if a.record.only != was {
				a.record.expanded = -1
			}
		})
		rows := a.record.filtered()
		if len(rows) == 0 {
			ui.Text(c, "没有这类请求。").FontSize(kit.FsNote).TextColor(t.TextMuted).Padding(12, 56, 12, 32)
			return
		}
		now := time.Now()
		a.record.list.Key = func(i int) any { return rows[i] }
		ui.List(c, &a.record.list, len(rows), func(i int) {
			a.recordRow(c, rows[i], now)
		}).Grow(1).FillWidth().Padding(0, 56, 32, 32)
	})
}

// filtered is the records the filter leaves, newest first, by their
// index in the record: built when the filter changes or showPref reads
// the record anew.
func (v *recordView) filtered() []int {
	if v.rows != nil && v.rowsFor == v.only {
		return v.rows
	}
	rows := make([]int, 0, len(v.records))
	for i := len(v.records) - 1; i >= 0; i-- {
		if v.only == 0 || v.records[i].Feature == recordFeatures[v.only-1].id {
			rows = append(rows, i)
		}
	}
	v.rows, v.rowsFor = rows, v.only
	return rows
}

// recordRow is one line of the AI record, opened on what it sent and what
// the provider counted.
func (a *App) recordRow(c *ui.Context, i int, now time.Time) {
	t := a.pal.Theme
	r := a.record.records[i]
	a.pal.Hairline(c)
	open := a.record.expanded == i
	at, where, end, endColor := kit.When(r.At, now), featureName(r.Feature)+" · "+a.recordWhere(r), "", t.TextMuted
	switch r.Outcome {
	case workspace.OutcomeDone:
		end, endColor = "完成", t.Text
	case workspace.OutcomeCancelled:
		end = "已取消"
	default:
		end, endColor = "失败："+r.Error, t.Danger
	}
	cost := costText(costOf(r))
	if cost == "" {
		cost = "—"
	}
	row := ui.ButtonBase(c).Row().FillWidth().MinHeight(kit.HRow+3).Gap(12).AlignItems(ui.Center).Padding(0, 6).Radius(kit.RXs).Label("记录-" + at + " " + where)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Text(c, at).FontSize(kit.FsControl).TextColor(t.TextMuted).Width(92).Shrink(0).SingleLine()
		ui.Text(c, where).FontSize(kit.FsControl).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
		ui.Text(c, tokens.Round(r.Usage.Input+r.Usage.Output)).FontSize(kit.FsControl).TextColor(t.TextMuted).Width(48).Shrink(0).SingleLine()
		ui.Text(c, cost).FontSize(kit.FsControl).TextColor(t.TextMuted).Width(76).Shrink(0).SingleLine()
		ui.Text(c, end).FontSize(kit.FsControl).TextColor(endColor).Width(120).Shrink(0).SingleLine().Ellipsis("…")
		chevron := kit.IcChevron
		if open {
			chevron = kit.IcChevronDown
		}
		kit.Glyph(c, chevron, kit.IconXs, a.pal.Faint)
	})
	if row.Clicked() {
		a.record.expanded = i
		if open {
			a.record.expanded = -1
		}
	}
	if open {
		a.recordMore(c, r)
	}
}

// recordMore is a line of the AI record opened: the model and its effort,
// the parts sent with their tokens as estimated, and what the provider
// counted.
func (a *App) recordMore(c *ui.Context, r workspace.Record) {
	t := a.pal.Theme
	note := func(s string) {
		ui.Text(c, s).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody).Selectable().FillWidth()
	}
	ui.Column(c).FillWidth().Padding(0, 6, 10, 6+92+12).Gap(2).Children(func() {
		switch {
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
			ui.Text(c, r.Error).FontSize(kit.FsNote).TextColor(t.Danger).LineHeight(kit.LhBody).Selectable().FillWidth()
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
	if n, ok := a.book.Tree().Node(r.Chapter); ok {
		return n.Title
	}
	return "已删除的章节"
}
