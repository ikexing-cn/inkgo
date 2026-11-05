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
		bookEditing := ext.Use(r, hostapi.EditingService)
		pal := ext.Use(r, hostapi.PaletteService)
		notice := ext.Use(r, hostapi.NoticesService)
		desktop := ext.Use(r, hostapi.NavigationService)
		desktopNative := ext.Use(r, hostapi.NativeUIService)
		desktopPaths := ext.Use(r, hostapi.MachinePathsService)
		ai := ext.Use(r, hostapi.ExtensionsService)
		r.OnStart(func(*ext.Session) error {
			*p = View{Book: book.Get(), Editing: bookEditing.Get(), Palette: pal.Get(), Notices: notice.Get(), Desktop: desktop.Get(), Native: desktopNative.Get(), Paths: desktopPaths.Get(), AI: ai.Get()}
			return nil
		})
		r.OnStop(func(*ext.Session) { *p = View{} })
		ext.Contribute(r, hostapi.Commands, "open", hostapi.Command{Name: snapstext.CmdSnapshots, Can: func() bool {
			_, ed, ok := bookEditing.Get().Chapter()
			_, lost, _ := desktop.Get().Selected()
			return ok && !ed.ReadOnly() && !lost && !bookEditing.Get().ChapterExternal()
		}, Run: p.openSnapshots, Menu: hostapi.MenuPosition{Menu: hostapi.MenuFile, After: "save", Order: 35}})
		ext.Contribute(r, hostapi.Overlays, "dialog", hostapi.Overlay{Open: func() bool { return p.SnapsOpen }, Draw: func(c *ui.Context) {
			if p.SnapsOpen {
				p.snapshots(c)
			}
		}})
	}}
}
