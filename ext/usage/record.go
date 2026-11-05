package usage

import (
	"maps"
	"writer/ext/usage/usagetext"

	"math"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/ext/ai/aiapi"
	"writer/ext/hostapi"
	"writer/locales"
	"writer/tokens"
	"writer/workspace"
)

// RecordView owns one work's AI usage snapshot and its request list.
// Loading invalidates row indices; only leaving the work resets the view.
type RecordView struct {
	Records  []workspace.Record
	Showing  bool
	Only     int // 0 for all, otherwise an index into recordFeatures plus one
	Expanded int // record index, or -1
	Rows     []int
	RowsFor  int
	List     ui.ListState
}

func (v *RecordView) reset() { *v = RecordView{Expanded: -1} }

func (v *RecordView) load(records []workspace.Record) {
	v.Records, v.Rows = records, nil
}

func (v *RecordView) show() { v.Showing, v.Only, v.Expanded = true, 0, -1 }

// recordFeatures are the features of the AI record as the author reads
// them, in the order the settings List them.
var recordFeatures = []struct {
	id   string
	name func() string
}{
	{workspace.FeatureTalk, locales.PageTalk},
	{workspace.FeatureDraft, locales.SettingsFeatureDraft},
	{workspace.FeatureRewrite, locales.SettingsFeatureRewrite},
	{workspace.FeatureContinue, usagetext.SettingsFeatureContinue},
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

// costOf is what the Records cost, by currency, those priced.
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
// prices built in. It is Only to look at; nothing clears it. Under it, the
// way to every request.
func (a *View) spent(c *ui.Context) {
	t := a.pal.Theme
	a.pal.Subhead(c, usagetext.SettingsUsage(), "")
	if len(a.Record.Records) == 0 {
		ui.Text(c, usagetext.SettingsNoUsage()).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody)
		return
	}
	all := 0
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		for _, f := range recordFeatures {
			var Rows []workspace.Record
			var u workspace.Usage
			for _, r := range a.Record.Records {
				if r.Feature == f.id {
					Rows = append(Rows, r)
					u = u.Add(r.Usage)
				}
			}
			n := len(Rows)
			if len(Rows) == 0 {
				continue
			}
			all += n
			cached := 0
			if u.Input > 0 {
				cached = int(math.Round(float64(u.CacheRead) * 100 / float64(u.Input)))
			}
			used := usagetext.SettingsSpentRow(n, tokens.Round(u.Input), cached, tokens.Round(u.Output))
			cost := costText(costOf(Rows...))
			if cost == "" {
				cost = usagetext.SettingsNoPrice()
			}
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, f.name()).FontSize(kit.FsControl).Width(120).Shrink(0).SingleLine()
				ui.Text(c, used).FontSize(kit.FsControl).TextColor(t.TextMuted).Grow(1).Shrink(1).SingleLine().Ellipsis("…")
				ui.Text(c, cost).FontSize(kit.FsControl).Shrink(0)
			})
		}
	})
	ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
		if a.pal.Ghost(c, usagetext.SettingsViewAll(all), ui.Color{}).Clicked() {
			a.Record.show()
		}
	})
}

// recordsView is the AI record: a line for each request, the newest first,
// those that failed or were cancelled too, filtered by feature. A line
// opens on the model, the parts sent and what the provider counted. The
// text sent is not kept, so not shown. The head stays above the lines as
// they scroll.
func (a *View) recordsView(c *ui.Context) {
	t := a.pal.Theme
	names := []string{usagetext.SettingsRecordsAll()}
	for _, f := range recordFeatures {
		names = append(names, f.name())
	}
	ui.Column(c).FillWidth().Grow(1).Shrink(1).Gap(12).Children(func() {
		ui.Row(c).FillWidth().Padding(12, 56, 0, 32).Shrink(0).AlignItems(ui.Center).Gap(12).Children(func() {
			if a.pal.Ghost(c, usagetext.SettingsUsageBack(), ui.Color{}).Clicked() {
				a.Record.Showing = false
			}
			ui.Spacer(c)
			was := a.Record.Only
			a.pal.Segments(c, &a.Record.Only, usagetext.SettingsFilterByFeature(), names)
			if a.Record.Only != was {
				a.Record.Expanded = -1
			}
		})
		Rows := a.Record.filtered()
		if len(Rows) == 0 {
			ui.Text(c, usagetext.SettingsNoRecords()).FontSize(kit.FsNote).TextColor(t.TextMuted).Padding(12, 56, 12, 32)
			return
		}
		now := time.Now()
		a.Record.List.Key = func(i int) any { return Rows[i] }
		ui.List(c, &a.Record.List, len(Rows), func(i int) {
			a.recordRow(c, Rows[i], now)
		}).Grow(1).FillWidth().Padding(0, 56, 32, 32)
	})
}

// filtered is the records the filter leaves, newest first, by their
// index in the record: built when the filter changes or showPref reads
// the record anew.
func (v *RecordView) filtered() []int {
	if v.Rows != nil && v.RowsFor == v.Only {
		return v.Rows
	}
	Rows := make([]int, 0, len(v.Records))
	for i := len(v.Records) - 1; i >= 0; i-- {
		if v.Only == 0 || v.Records[i].Feature == recordFeatures[v.Only-1].id {
			Rows = append(Rows, i)
		}
	}
	v.Rows, v.RowsFor = Rows, v.Only
	return Rows
}

// recordRow is one line of the AI record, opened on what it sent and what
// the provider counted.
func (a *View) recordRow(c *ui.Context, i int, now time.Time) {
	t := a.pal.Theme
	r := a.Record.Records[i]
	a.pal.Hairline(c)
	open := a.Record.Expanded == i
	at, where, end, endColor := kit.When(r.At, now), featureName(r.Feature)+" · "+a.recordWhere(r), "", t.TextMuted
	switch r.Outcome {
	case workspace.OutcomeDone:
		end, endColor = usagetext.SettingsRecordDone(), t.Text
	case workspace.OutcomeCancelled:
		end = usagetext.SettingsRecordCancelled()
	default:
		end, endColor = usagetext.SettingsRecordFailed(r.Error), t.Danger
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
		a.Record.Expanded = i
		if open {
			a.Record.Expanded = -1
		}
	}
	if open {
		a.recordMore(c, r)
	}
}

// recordMore is a line of the AI record opened: the model and its effort,
// the parts sent with their tokens as estimated, and what the provider
// counted.
func (a *View) recordMore(c *ui.Context, r workspace.Record) {
	t := a.pal.Theme
	note := func(s string) {
		ui.Text(c, s).FontSize(kit.FsNote).TextColor(t.TextMuted).LineHeight(kit.LhBody).Selectable().FillWidth()
	}
	ui.Column(c).FillWidth().Padding(0, 6, 10, 6+92+12).Gap(2).Children(func() {
		switch {
		case r.Effort == "none":
			note(usagetext.SettingsRecordNoThinking(a.recordReplyLabel(r.Model, "")))
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
					name = usagetext.RecordsChapterText()
				case name == "":
					name = p.Name
				}
				parts = append(parts, name+" "+tokens.Round(p.Tokens))
			}
			note(usagetext.SettingsRecordSent(strings.Join(parts, " · ")))
		}
		used := usagetext.SettingsRecordActual(tokens.Round(r.Usage.Input), tokens.Round(r.Usage.Output))
		if r.Usage.CacheRead > 0 {
			used = usagetext.SettingsRecordActualCached(tokens.Round(r.Usage.Input), tokens.Round(r.Usage.CacheRead), tokens.Round(r.Usage.Output))
		}
		note(used)
		if r.Outcome == workspace.OutcomeFailed {
			ui.Text(c, r.Error).FontSize(kit.FsNote).TextColor(t.Danger).LineHeight(kit.LhBody).Selectable().FillWidth()
		}
	})
}

// recordWhere names where a request of the AI record was for: its talk
// by its first words, or its chapter by title, as the work has them now.
func (a *View) recordWhere(r workspace.Record) string {
	if r.Talk != "" {
		if title, ok := a.talkTitle(r.Talk); ok {
			return title
		}
		return locales.SnapAITalkGone()
	}
	if n, ok := a.host.ChapterTitle(r.Chapter); ok {
		return n
	}
	return usagetext.SettingsRecordChapterGone()
}

func (a *View) recordReplyLabel(name, effort string) string {
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

// View owns the usage page on the frame thread.
type View struct {
	revision atomic.Uint64
	loaded   uint64
	Record   RecordView
	pal      *kit.Palette
	ai       aiapi.Backend
	book     *book.Book
	host     hostapi.Book
	store    *workspace.Store
}

func (a *View) talkTitle(id string) (string, bool) {
	events, err := a.store.Events(id)
	title := workspace.TalkTitle(events, func(id string) string { t, _ := a.host.ChapterTitle(id); return t })
	if err != nil {
		return "", false
	}
	switch title.Kind {
	case workspace.AIDraft:
		text := title.Text
		if text == "" {
			text = locales.AppChapterDeleted()
		}
		return locales.SideDraftTitle(text), true
	case workspace.AIRewrite:
		return locales.SideRewriteTitle(title.Text), true
	case workspace.TitleWords:
		return title.Text, true
	default:
		return locales.SideNewTalk(), true
	}
}

// Snapshot returns the native usage view on the frame thread.
func (a *View) Snapshot() *RecordView { return &a.Record }
