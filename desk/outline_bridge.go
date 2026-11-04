package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/desk/outline"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/locales"
)

// chapterOutlines is the temporary read contract until the context module owns it.
type chapterOutlines interface {
	ChapterOutline(string) string
}

func (a *App) initOutline() {
	h := extensionHost{a}
	a.plan.Services = outline.Services{Book: h, Palette: h, Notices: h, Dialogs: h, Desktop: h}
	a.chapterOutlines = &a.plan
}

func (a *App) registerOutline(r *ext.Registrar) {
	ext.Contribute(r, hostapi.ChapterOpens, "outline-open", func(v hostapi.ChapterOpened) { a.plan.OpenChapter(v.ID, v.Outline) })
	ext.Contribute(r, hostapi.ChapterEditStates, "outline-state", hostapi.ChapterEdits{Dirty: func(id string) bool { d := a.plan.Draft(id); return d.Text != d.Saved }, Discard: func(id string) { d := a.plan.Draft(id); d.Text, d.Seen = d.Saved, d.Saved }})
	ext.Contribute(r, hostapi.SaveEdits, "outline-flush", func() error { return a.plan.SaveChapter() })
	ext.Contribute(r, hostapi.Pages, "outline", hostapi.Page{Title: locales.PagePlan, Label: "大纲", Icon: kit.IcOutline, Order: 30,
		Available: func() bool { return a.Store != nil }, Enter: func() { a.plan.Enter() }, Leave: func() { a.plan.Leave() }, Draw: func(c *ui.Context) { a.plan.View(c) },
		HasPane: func() bool { return true }, PaneOpen: func() bool { return a.plan.Pane }, TogglePane: func() { a.plan.Pane = !a.plan.Pane }})
	ext.Contribute(r, hostapi.ChapterHeadItems, "outline-head", hostapi.DrawItem{Draw: func(c *ui.Context) { a.plan.ChapterHead(c) }})
	ext.Contribute(r, hostapi.Overlays, "outline-dialog", hostapi.Overlay{Draw: func(c *ui.Context) { a.plan.Dialog(c) }})
	ext.Contribute(r, hostapi.BeforeLeave, "outline-leave", func() { a.plan.FinishOutline() })
	ext.Contribute(r, hostapi.BeforeChapterChange, "outline-change", func() { a.plan.FinishOutline() })
	ext.Contribute(r, hostapi.WindowActivities, "outline-activity", func(v hostapi.WindowActivity) { a.plan.WindowActivity(v.Active, v.Returned) })
	ext.Contribute(r, hostapi.FrameTicks, "outline-save", hostapi.FrameTick{Tick: func(c *ui.Context) { a.plan.WatchChapter(c) }})
}
