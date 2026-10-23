package exportdialog

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/shelf"
	"writer/workspace"
)

// Host gives the export dialog its book, local preferences, and file actions.
type Host interface {
	// Colors gives the current palette and its widgets.
	Colors() *kit.Palette
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
	// PickInto starts a folder choice and reports whether a picker is available.
	PickInto(*ui.Context, string, func(string)) bool
	// StartJob runs work in the background and returns its result to the view thread.
	StartJob(ui.Services, func() func())
	// Dialogs gives the optional file save and folder reveal callbacks.
	Dialogs() (save func(string, string) (string, error), reveal func(string) error)
}
