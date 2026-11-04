package desk

import (
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/locales"
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
var recordFeatures = []struct {
	id   string
	name func() string
}{
	{workspace.FeatureTalk, locales.PageTalk},
	{workspace.FeatureDraft, locales.SettingsFeatureDraft},
	{workspace.FeatureRewrite, locales.SettingsFeatureRewrite},
	{workspace.FeatureContinue, locales.SettingsFeatureContinue},
	{workspace.FeatureCheck, locales.PageCheck},
}

func featureName(id string) string {
	for _, f := range recordFeatures {
		if f.id == id {
			return f.name()
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
			out = append(out, locales.AiCostUnder(cur+"0.01"))
		} else {
			if cur == "$" {
				out = append(out, locales.SettingsSpentCost(v))
			} else {
				out = append(out, locales.SettingsCostCurrency(cur, v))
			}
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
	a.pal.Subhead(c, locales.SettingsUsage(), "")
	if len(a.record.records) == 0 {
		ui.Text(c, locales.SettingsNoUsage()).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody)
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
			cached := 0
			if u.Input > 0 {
				cached = int(math.Round(float64(u.CacheRead) * 100 / float64(u.Input)))
			}
			used := locales.SettingsSpentRow(n, tokens.Round(u.Input), cached, tokens.Round(u.Output))
			cost := costText(costOf(rows...))
			if cost == "" {
				cost = locales.SettingsNoPrice()
			}
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, f.name()).FontSize(kit.FsControl).Width(120).Shrink(0).SingleLine()
				ui.Text(c, used).FontSize(kit.FsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
				ui.Text(c, cost).FontSize(kit.FsControl).Shrink(0)
			})
		}
	})
	ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
		if a.pal.Ghost(c, locales.SettingsViewAll(all), ui.Color{}).Clicked() {
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
	names := []string{locales.SettingsRecordsAll()}
	for _, f := range recordFeatures {
		names = append(names, f.name())
	}
	ui.Column(c).FillWidth().Grow(1).Shrink(1).Gap(12).Children(func() {
		ui.Row(c).FillWidth().Padding(12, 56, 0, 32).Shrink(0).AlignItems(ui.Center).Gap(12).Children(func() {
			if a.pal.Ghost(c, locales.SettingsUsageBack(), ui.Color{}).Clicked() {
				a.record.showing = false
			}
			ui.Spacer(c)
			was := a.record.only
			a.pal.Segments(c, &a.record.only, locales.SettingsFilterByFeature(), names)
			if a.record.only != was {
				a.record.expanded = -1
			}
		})
		rows := a.record.filtered()
		if len(rows) == 0 {
			ui.Text(c, locales.SettingsNoRecords()).FontSize(kit.FsNote).TextColor(t.TextMuted).Padding(12, 56, 12, 32)
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
		end, endColor = locales.SettingsRecordDone(), t.Text
	case workspace.OutcomeCancelled:
		end = locales.SettingsRecordCancelled()
	default:
		end, endColor = locales.SettingsRecordFailed(r.Error), t.Danger
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
			note(locales.SettingsRecordNoThinking(a.recordReplyLabel(r.Model, "")))
		default:
			note(a.recordReplyLabel(r.Model, "") + " · " + a.ai.EffortLabel(r.Effort))
		}
		if len(r.Parts) > 0 {
			var parts []string
			for _, p := range r.Parts {
				name := ""
				if label := partNames[p.Name]; label != nil {
					name = label()
				}
				switch {
				case p.Name == workspace.PartSetting:
					name = locales.RecordsLore()
				case p.Name == workspace.PartProse && r.Feature != workspace.FeatureTalk:
					name = locales.RecordsChapterText()
				case name == "":
					name = p.Name
				}
				parts = append(parts, name+" "+tokens.Round(p.Tokens))
			}
			note(locales.SettingsRecordSent(strings.Join(parts, " · ")))
		}
		used := locales.SettingsRecordActual(tokens.Round(r.Usage.Input), tokens.Round(r.Usage.Output))
		if r.Usage.CacheRead > 0 {
			used = locales.SettingsRecordActualCached(tokens.Round(r.Usage.Input), tokens.Round(r.Usage.CacheRead), tokens.Round(r.Usage.Output))
		}
		note(used)
		if r.Outcome == workspace.OutcomeFailed {
			ui.Text(c, r.Error).FontSize(kit.FsNote).TextColor(t.Danger).LineHeight(kit.LhBody).Selectable().FillWidth()
		}
	})
}

// recordWhere names where a request of the AI record was for: its talk
// by its first words, or its chapter by title, as the work has them now.
func (a *App) recordWhere(r workspace.Record) string {
	if r.Talk != "" {
		if title, ok := a.conversationTitle(r.Talk); ok {
			return title
		}
		return locales.SnapAITalkGone()
	}
	if n, ok := a.book.Tree().Node(r.Chapter); ok {
		return n.Title
	}
	return locales.SettingsRecordChapterGone()
}

func (a *App) recordReplyLabel(name, effort string) string {
	ref, _ := workspace.ParseModelRef(name)
	name = ref.Model
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if effort != "" && name != "" {
		name += " · " + a.ai.EffortLabel(effort)
	}
	return name
}

// partNames name the parts of a talk request as the author reads them.
var partNames = map[string]func() string{
	workspace.PartRules:    locales.ChatPartRules,
	workspace.PartIndex:    locales.ChatPartCatalog,
	workspace.PartDigest:   locales.ChatPartDigest,
	workspace.PartOutline:  locales.ChatPartOutline,
	workspace.PartCards:    locales.ChatPartCards,
	workspace.PartOutlines: locales.ChatPartPrevOutlines,
	workspace.PartPrevious: locales.ChatPartPrevChapter,
	workspace.PartChapter:  locales.ChatPartThisChapter,
	workspace.PartRecap:    locales.ChatPartRecap,
	workspace.PartLooked:   locales.ChatPartLook,
	workspace.PartTalk:     locales.ChatPartBefore,
	workspace.PartProse:    locales.ChatPartProse,
	workspace.PartInput:    locales.ChatPartSaid,
}
