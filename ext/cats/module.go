package cats

import (
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/cats/catstext"
	"writer/ext/hostapi"
	"writer/locales"
)

// Module registers the category editor and its File menu command.
var Module = module(nil)

// New creates an independent category editor controller.
func New() (ext.Module, *Page) { p := &Page{}; return module(p), p }
func module(controller *Page) ext.Module {
	return ext.Module{ID: "cats", Kind: ext.Feature, Group: hostapi.GroupFiles, Name: catstext.ModCats, Desc: catstext.ModCatsAbout, Setup: func(r *ext.Registrar) {
		p := controller
		if p == nil {
			p = &Page{}
		}
		book := ext.Use(r, hostapi.BookService)
		bookEditing := ext.Use(r, hostapi.EditingService)
		pal := ext.Use(r, hostapi.PaletteService)
		notices := ext.Use(r, hostapi.NoticesService)
		desktop := ext.Use(r, hostapi.NavigationService)
		r.OnStart(func(*ext.Session) error {
			*p = Page{Book: book.Get(), Editing: bookEditing.Get(), Palette: pal.Get(), Notices: notices.Get()}
			return nil
		})
		r.OnStop(func(*ext.Session) { *p = Page{} })
		ext.Contribute(r, hostapi.Pages, "categories", hostapi.Page{MenuOnly: true, Title: locales.CmdCategories, Label: "作品分类", Icon: kit.IcTag, Order: 50, Available: func() bool { return book.Get().Store() != nil }, Draw: p.View})
		ext.Contribute(r, hostapi.Commands, "open", hostapi.Command{Name: locales.CmdCategories, Can: func() bool { return book.Get().Store() != nil }, Run: func() { desktop.Get().ShowPage("cats.categories") }, Menu: hostapi.MenuPosition{Menu: hostapi.MenuFile, After: "file-organize-separator"}})
	}}
}
