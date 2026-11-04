package outline

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/context/contextapi"
	"writer/ext/hostapi"
	"writer/ext/outline/outlineapi"
	"writer/ext/outline/outlinetext"
	"writer/locales"
)

// Module registers the outline and its live reference material.
var Module = module(nil)

// New creates an independent outline module and its session controller.
// The controller is initialized on Start and cleared on Stop.
func New() (ext.Module, *Page) {
	p := &Page{}
	return module(p), p
}

func module(controller *Page) ext.Module {
	return ext.Module{ID: "outline", Kind: ext.Feature, Group: hostapi.GroupLore, Requires: []string{"context"}, Name: outlinetext.ModOutline, Desc: outlinetext.ModOutlineAbout, Setup: func(r *ext.Registrar) {
		p := controller
		if p == nil {
			p = &Page{}
		}
		ext.Provide(r, outlineapi.Active, func(*ext.Session) struct{} { return struct{}{} })
		book := ext.Use(r, hostapi.BookService)
		palette := ext.Use(r, hostapi.PaletteService)
		notices := ext.Use(r, hostapi.NoticesService)
		dialogs := ext.Use(r, hostapi.DialogsService)
		desktop := ext.Use(r, hostapi.DesktopService)
		r.OnStart(func(*ext.Session) error {
			*p = Page{Services: Services{Book: book.Get(), Palette: palette.Get(), Notices: notices.Get(), Dialogs: dialogs.Get(), Desktop: desktop.Get()}}
			return nil
		})
		r.OnStop(func(*ext.Session) {
			if p.Services.Book.Content() != nil {
				p.Leave()
			}
			*p = Page{}
		})
		setupPage(r, p)
		ext.Contribute(r, contextapi.Outlines, "outlines", contextapi.OutlineProvider(func(id string) string {
			if p.outlineDraft.Node == id {
				return p.outlineDraft.Text
			}
			return p.ChapterOutline(id)
		}))
	}}
}

func setupPage(r *ext.Registrar, p *Page) {
	ext.Contribute(r, hostapi.ChapterOpens, "outline-open", func(v hostapi.ChapterOpened) { p.OpenChapter(v.ID) })
	ext.Contribute(r, hostapi.ChapterEditStates, "outline-state", hostapi.ChapterEdits{Dirty: func(id string) bool { d := p.Draft(id); return d.Text != d.Saved }, Discard: func(id string) { d := p.Draft(id); d.Text, d.Seen = d.Saved, d.Saved }})
	ext.Contribute(r, hostapi.SaveEdits, "outline-flush", func() error { return p.SaveChapter() })
	ext.Contribute(r, hostapi.Pages, "outline", hostapi.Page{Title: locales.PagePlan, Label: "大纲", Icon: kit.IcOutline, Order: 30,
		Available: func() bool { return p.Services.Book.Store() != nil }, Enter: func() { p.Enter() }, Leave: func() { p.Leave() }, Draw: func(c *ui.Context) { p.View(c) },
		HasPane: func() bool { return true }, PaneOpen: func() bool { return p.Pane }, TogglePane: func() { p.Pane = !p.Pane }})
	ext.Contribute(r, hostapi.ChapterHeadItems, "outline-head", hostapi.DrawItem{Draw: func(c *ui.Context) { p.ChapterHead(c) }})
	ext.Contribute(r, hostapi.Overlays, "outline-dialog", hostapi.Overlay{Draw: func(c *ui.Context) { p.Dialog(c) }})
	ext.Contribute(r, hostapi.BeforeLeave, "outline-leave", func() { p.FinishOutline() })
	ext.Contribute(r, hostapi.BeforeChapterChange, "outline-change", func() { p.FinishOutline() })
	ext.Contribute(r, hostapi.WindowActivities, "outline-activity", func(v hostapi.WindowActivity) { p.WindowActivity(v.Active, v.Returned) })
	ext.Contribute(r, hostapi.FrameTicks, "outline-save", hostapi.FrameTick{Tick: func(c *ui.Context) { p.WatchChapter(c) }})
}
