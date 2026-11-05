// Package trash contributes the trash interface.
package trash

import (
	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/ext/trash/trashtext"
)

// Module registers this extension's presentation and command.
var Module = module(nil)

// New creates an independent frame-thread controller.
func New() (ext.Module, *View) { p := &View{}; return module(p), p }
func module(controller *View) ext.Module {
	return ext.Module{ID: "trash", Kind: ext.Feature, Group: hostapi.GroupFiles, Name: trashtext.ModTrash, Desc: trashtext.ModTrashAbout, Setup: func(r *ext.Registrar) {
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
		r.OnStart(func(*ext.Session) error {
			*p = View{Book: book.Get(), Editing: bookEditing.Get(), Palette: pal.Get(), Notices: notice.Get(), Desktop: desktop.Get(), Native: desktopNative.Get(), Paths: desktopPaths.Get()}
			return nil
		})
		r.OnStop(func(*ext.Session) { *p = View{} })
		ext.Contribute(r, hostapi.Commands, "open", hostapi.Command{Name: trashtext.CmdTrash, Can: func() bool { return book.Get().Store() != nil }, Run: p.openTrash, Menu: hostapi.MenuPosition{Menu: hostapi.MenuFile, After: "cats.open", Order: 55}})
		ext.Contribute(r, hostapi.Overlays, "dialog", hostapi.Overlay{Open: func() bool { return p.TrashOpen }, Draw: func(c *ui.Context) {
			if p.TrashOpen {
				p.trashView(c)
			}
		}})
	}}
}
