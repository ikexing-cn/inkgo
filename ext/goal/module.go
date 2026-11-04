package goal

import (
	"github.com/egoist/mygo/ui"
	"time"
	"writer/ext"
	"writer/ext/goal/goaltext"
	"writer/ext/hostapi"
)

// Module registers the daily goal and count popover.
var Module = module(nil)

// New creates an independent goal controller.
func New() (ext.Module, *Page) { p := &Page{}; return module(p), p }
func module(controller *Page) ext.Module {
	return ext.Module{ID: "goal", Kind: ext.Feature, Group: hostapi.GroupWrite, Name: func() string { return goaltext.ModGoal("character") }, Desc: goaltext.ModGoalAbout, Setup: func(r *ext.Registrar) {
		p := controller
		if p == nil {
			p = &Page{}
		}
		book := ext.Use(r, hostapi.BookService)
		pal := ext.Use(r, hostapi.PaletteService)
		notices := ext.Use(r, hostapi.NoticesService)
		r.OnStart(func(*ext.Session) error {
			*p = Page{Book: book.Get(), Palette: pal.Get(), Notices: notices.Get()}
			if p.Book.Store() != nil {
				return p.load()
			}
			return nil
		})
		roll := func() {
			if p.Book.Store() != nil && p.Counter.Day != "" && p.RollDay(time.Now()) {
				p.persist()
			}
		}
		r.OnStop(func(*ext.Session) { roll(); *p = Page{} })
		ext.Contribute(r, hostapi.FrameTicks, "counter", hostapi.FrameTick{Tick: func(c *ui.Context) { roll(); c.After(time.Minute) }})
		ext.Contribute(r, hostapi.SaveEdits, "save", func() error { roll(); return nil })
		ext.Contribute(r, hostapi.WordSuffix, "goal", hostapi.CountSuffix{Text: func() string { s, _ := p.suffix(""); return s }, Tooltip: func(book string) string { _, s := p.suffix(book); return s }})
		ext.Contribute(r, hostapi.WordPopovers, "popover", hostapi.WordPopover{Draw: p.popover})
	}}
}
