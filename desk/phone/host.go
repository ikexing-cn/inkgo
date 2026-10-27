package phone

import (
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/shelf"
)

// Host gives the phone preview its current chapter, palette, and local preferences.
type Host interface {
	// Colors gives the current palette and its widgets.
	Colors() *kit.Palette
	// PhoneColors gives the light and dark palettes for the phone screen.
	PhoneColors() (*kit.Palette, *kit.Palette)
	// Content gives the book content shared by the pages.
	Content() *book.Book
	// WorkPath identifies the work by its absolute folder.
	WorkPath() string
	// ShelfPrefs gives the editable preferences shared by works on this machine.
	ShelfPrefs() *shelf.Shelf
	// SaveShelf saves the machine preferences.
	SaveShelf()
	// Chapter gives the selected editor, its ID on the writing page, and whether a chapter is open.
	Chapter() (string, *editor.Editor, bool)
}
