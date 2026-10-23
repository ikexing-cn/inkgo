package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/shelf"
	"writer/workspace"
)

// appHost is what the pages of the packages under desk reach of App: the
// methods their Host interfaces name, which App does not export.
type appHost struct{ a *App }

type host interface {
	// Colors gives the current palette and its widgets.
	Colors() *kit.Palette
	// PhoneColors gives the light and dark palettes for the phone screen.
	PhoneColors() (*kit.Palette, *kit.Palette)
	// Content gives the book content shared by the pages.
	Content() *book.Book
	// Workspace gives the current store and work path.
	Workspace() (*workspace.Store, string)
	// Notices gives the errors, completion notices, and hints shown by toasts.
	Notices() *kit.Status
	// ShelfPrefs gives the editable preferences shared by works on this machine.
	ShelfPrefs() *shelf.Shelf
	// SaveShelf saves the machine preferences.
	SaveShelf()
	// Chapter gives the selected editor, its ID on the writing page, and whether a chapter is open.
	Chapter() (string, *editor.Editor, bool)
	// Flush saves the shown chapter before export reads it.
	Flush()
	// SyncPlanOutline updates the shown chapter draft after its outline is saved.
	SyncPlanOutline(string, string)
	// OpenWriting switches to the writing page and opens the chapter.
	OpenWriting(string)
	// ShowOutline switches to the outline page.
	ShowOutline()
	// DropMenu draws a menu using the host menu ownership.
	DropMenu(*ui.Context, ui.Element, string, func(ui.Element), func(*kit.Menu))
	// SettingColumn draws the shared right column beside the outline page.
	SettingColumn(*ui.Context)
	// PickInto starts a folder choice and reports whether a picker is available.
	PickInto(*ui.Context, string, func(string)) bool
	// StartJob runs work in the background and returns its result to the view thread.
	StartJob(ui.Services, func() func())
	// Dialogs gives the optional file save and folder reveal callbacks.
	Dialogs() (save func(string, string) (string, error), reveal func(string) error)
}

func (h appHost) Colors() *kit.Palette { return h.a.pal }

func (h appHost) Content() *book.Book { return h.a.book }

func (h appHost) Workspace() (*workspace.Store, string) { return h.a.Store, h.a.path }

func (h appHost) Notices() *kit.Status { return &h.a.status }

func (h appHost) ShelfPrefs() *shelf.Shelf { return &h.a.shelf }

// Chapter gives an empty ID off the writing page; the editor is the selected tab's.
// open also checks that the chapter exists and is not a folder.
func (h appHost) Chapter() (id string, ed *editor.Editor, open bool) {
	if h.a.page == pageWrite {
		id = h.a.write.node
	}
	open, _ = h.a.chapterOpen()
	return id, h.a.write.cur().ed, open
}

func (h appHost) StartJob(sv ui.Services, fn func() func()) { h.a.jobs.start(sv, fn) }

func (h appHost) Dialogs() (func(string, string) (string, error), func(string) error) {
	return h.a.SaveFile, h.a.Reveal
}

func (h appHost) PhoneColors() (*kit.Palette, *kit.Palette) { return h.a.light, h.a.dark }

func (h appHost) SyncPlanOutline(id, text string) {
	if h.a.write.node == id {
		d := &h.a.write.cur().outline
		d.Text, d.Saved, d.Seen = text, text, text
	}
}

func (h appHost) OpenWriting(id string) {
	h.a.page = pageWrite
	h.a.openNode(id)
}

func (h appHost) ShowOutline() { h.a.page = pagePlan }

func (h appHost) DropMenu(c *ui.Context, b ui.Element, key string, place func(ui.Element), build func(*kit.Menu)) {
	h.a.dropMenu(c, b, key, place, build)
}

func (a *App) dropMenu(c *ui.Context, b ui.Element, key string, place func(ui.Element), build func(*kit.Menu)) {
	a.menu.DropMenu(c, a.pal, b, key, place, build)
}

func (h appHost) SaveShelf() { h.a.saveShelf() }

func (h appHost) Flush() { h.a.flush() }

func (h appHost) SettingColumn(c *ui.Context) { h.a.settingColumn(c) }

func (h appHost) PickInto(c *ui.Context, title string, picked func(string)) bool {
	return h.a.pickInto(c, title, picked)
}
