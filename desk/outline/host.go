package outline

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
)

// Host gives the outline its book, palette, navigation, and shared column.
type Host interface {
	// Colors gives the current palette and its widgets.
	Colors() *kit.Palette
	// Content gives the book content shared by the pages.
	Content() *book.Book
	// WorkPath gives the absolute folder to reveal when the outline cannot be read.
	WorkPath() string
	// Notices gives the errors, completion notices, and hints shown by toasts.
	Notices() *kit.Status
	// Chapter gives the selected editor, its ID on the writing page, and whether a chapter is open.
	Chapter() (string, *editor.Editor, bool)
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
	// Dialogs gives the optional file save and folder reveal callbacks.
	Dialogs() (save func(string, string) (string, error), reveal func(string) error)
}
