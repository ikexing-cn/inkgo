package desk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/editor"
	"writer/shelf"
	"writer/workspace"
)

type host interface {
	colors() *palette
	phoneColors() (*palette, *palette)
	Book() *book.Book
	workspace() (*workspace.Store, string)
	notices() *status
	shelfPrefs() *shelf.Shelf
	saveShelf()
	chapter() (string, *editor.Editor, bool)
	flush()
	syncPlanOutline(string, string)
	openWriting(string)
	showOutline()
	dropMenu(*ui.Context, ui.Element, string, func(ui.Element), func(*menu))
	settingColumn(*ui.Context)
	startJob(ui.Services, func() func())
	dialogs() (save func(string, string) (string, error), pick func(string) (string, error), reveal func(string) error)
}

func (a *App) colors() *palette { return a.pal }

func (a *App) Book() *book.Book { return a.book }

func (a *App) workspace() (*workspace.Store, string) { return a.Store, a.path }

func (a *App) notices() *status { return &a.status }

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

func (a *App) dialogs() (func(string, string) (string, error), func(string) (string, error), func(string) error) {
	return a.SaveFile, a.PickFolder, a.Reveal
}

func (a *App) phoneColors() (*palette, *palette) { return a.light, a.dark }

func (a *App) syncPlanOutline(id, text string) {
	if a.write.node == id {
		d := &a.write.cur().outline
		d.text, d.saved, d.seen = text, text, text
	}
}

func (a *App) openWriting(id string) {
	a.page = pageWrite
	a.openNode(id)
}

func (a *App) showOutline() { a.page = pagePlan }
