// Package snaps contributes the snapshots interface.
package snaps

import (
	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/ext/snaps/snapstext"
)

// Module registers this extension's presentation and command.
var Module = module(nil)

// New creates an independent frame-thread controller.
func New() (ext.Module, *View) { p := &View{}; return module(p), p }
func module(controller *View) ext.Module {
	return ext.Module{ID: "snaps", Kind: ext.Feature, Group: hostapi.GroupFiles, Name: snapstext.ModSnaps, Desc: snapstext.ModSnapsAbout, Setup: func(r *ext.Registrar) {
		p := controller
		if p == nil {
			p = &View{}
		}
		book := ext.Use(r, hostapi.BookService)
		pal := ext.Use(r, hostapi.PaletteService)
		notice := ext.Use(r, hostapi.NoticesService)
		desktop := ext.Use(r, hostapi.DesktopService)
		talk := ext.Use(r, hostapi.TalkService)
		ai := ext.Use(r, hostapi.AIService)
		r.OnStart(func(*ext.Session) error {
			*p = View{Book: book.Get(), Palette: pal.Get(), Notices: notice.Get(), Desktop: desktop.Get(), AI: ai.Get(), Talk: talk.Get()}
			return nil
		})
		r.OnStop(func(*ext.Session) { *p = View{} })
		ext.Contribute(r, hostapi.Commands, "open", hostapi.Command{Name: snapstext.CmdSnapshots, Can: func() bool {
			_, ed, ok := book.Get().Chapter()
			_, lost, _ := desktop.Get().Selected()
			return ok && !ed.ReadOnly() && !lost && !desktop.Get().ChapterExternal()
		}, Run: p.openSnapshots, Menu: hostapi.MenuPosition{Menu: hostapi.MenuFile, After: "save", Order: 35}})
		ext.Contribute(r, hostapi.Overlays, "dialog", hostapi.Overlay{Open: func() bool { return p.SnapsOpen }, Draw: func(c *ui.Context) {
			if p.SnapsOpen {
				p.snapshots(c)
			}
		}})
	}}
}
