package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/shelf"
	"writer/workspace"
)

type host interface {
	// colors gives the current palette and its widgets.
	colors() *kit.Palette
	// phoneColors gives the light and dark palettes for the phone screen.
	phoneColors() (*kit.Palette, *kit.Palette)
	// content gives the book content shared by the pages.
	content() *book.Book
	// workspace gives the current store and work path.
	workspace() (*workspace.Store, string)
	// notices gives the errors, completion notices, and hints shown by toasts.
	notices() *kit.Status
	// shelfPrefs gives the editable preferences shared by works on this machine.
	shelfPrefs() *shelf.Shelf
	// saveShelf saves the machine preferences.
	saveShelf()
	// chapter gives the selected editor, its ID on the writing page, and whether a chapter is open.
	chapter() (string, *editor.Editor, bool)
	// flush saves the shown chapter before export reads it.
	flush()
	// syncPlanOutline updates the shown chapter draft after its outline is saved.
	syncPlanOutline(string, string)
	// openWriting switches to the writing page and opens the chapter.
	openWriting(string)
	// showOutline switches to the outline page.
	showOutline()
	// dropMenu draws a menu using the host menu ownership.
	dropMenu(*ui.Context, ui.Element, string, func(ui.Element), func(*kit.Menu))
	// settingColumn draws the shared right column beside the outline page.
	settingColumn(*ui.Context)
	// pickInto starts a folder choice and reports whether a picker is available.
	pickInto(*ui.Context, string, func(string)) bool
	// startJob runs work in the background and returns its result to the view thread.
	startJob(ui.Services, func() func())
	// dialogs gives the optional file save and folder reveal callbacks.
	dialogs() (save func(string, string) (string, error), reveal func(string) error)
}

func (a *App) colors() *kit.Palette { return a.pal }

func (a *App) content() *book.Book { return a.book }

func (a *App) workspace() (*workspace.Store, string) { return a.Store, a.path }

func (a *App) notices() *kit.Status { return &a.status }

func (a *App) shelfPrefs() *shelf.Shelf { return &a.shelf }

// The ID is empty off the writing page; the editor is the selected tab's.
// open also checks that the chapter exists and is not a folder.
func (a *App) chapter() (id string, ed *editor.Editor, open bool) {
	if a.page == pageWrite {
		id = a.write.node
	}
	open, _ = a.chapterOpen()
	return id, a.write.cur().ed, open
}

func (a *App) startJob(sv ui.Services, fn func() func()) { a.jobs.start(sv, fn) }

func (a *App) dialogs() (func(string, string) (string, error), func(string) error) {
	return a.SaveFile, a.Reveal
}

func (a *App) phoneColors() (*kit.Palette, *kit.Palette) { return a.light, a.dark }

func (a *App) syncPlanOutline(id, text string) {
	if a.write.node == id {
		d := &a.write.cur().outline
		d.Text, d.Saved, d.Seen = text, text, text
	}
}

func (a *App) openWriting(id string) {
	a.page = pageWrite
	a.openNode(id)
}

func (a *App) showOutline() { a.page = pagePlan }

func (a *App) dropMenu(c *ui.Context, b ui.Element, key string, place func(ui.Element), build func(*kit.Menu)) {
	a.menu.DropMenu(c, a.pal, b, key, place, build)
}
