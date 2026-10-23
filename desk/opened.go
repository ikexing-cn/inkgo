package desk

import (
	"reflect"
	"time"

	"writer/editor"
	"writer/workspace"
)

// openChapter is a node opened in this run: its editor (nil for a folder),
// what the disk had of it as last read or written, and the outline being
// typed.
type openChapter struct {
	id string
	ed *editor.Editor

	// What the disk has of it, as of the last read or write.
	saved    string
	savedVer int
	stamp    workspace.Stamp
	lastSnap time.Time

	// Left with the disk's text changed outside and not yet chosen
	// between, or with the chapter gone from the disk.
	external  *workspace.Chapter
	lost      bool
	lostTitle string

	// The outline, saved as the author pauses like the text.
	outline, outlineSaved, outlinePrev string
	outlineAt                          time.Time

	// changed says the prose was saved since it opened.
	changed bool
}

// unsettled says the chapter was left with words that are not on the disk:
// they could not be saved, or the disk has other words to choose between.
func (c *openChapter) unsettled() bool {
	return c.external != nil || c.lost || c.ed.Doc.Version() != c.savedVer
}

// same says the disk has what the chapter's editor has, so the editor goes
// on.
func (c *openChapter) same(ch workspace.Chapter) bool {
	marks := storedMarks(c.ed.Doc.Marks())
	return ch.Text == workspace.CleanProse(c.ed.Doc.String()) && (len(marks) == 0 && len(ch.Marks) == 0 || reflect.DeepEqual(marks, ch.Marks))
}
