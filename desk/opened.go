package desk

import (
	"errors"
	"reflect"
	"time"

	"writer/desk/book"
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
	outline book.OutlineDraft

	// changed says the prose was saved since it opened.
	changed bool
}

// unsettled says the chapter was left with words that are not on the disk:
// they could not be saved, or the disk has other words to choose between.
func (c *openChapter) unsettled() bool {
	if c.ed == nil {
		return false
	}
	return c.external != nil || c.lost || c.ed.Doc.Version() != c.savedVer
}

// same says the disk has what the chapter's editor has, so the editor goes
// on.
func (c *openChapter) same(ch workspace.Chapter) bool {
	marks := c.marks()
	return ch.Text == workspace.CleanProse(c.ed.Doc.String()) && (len(marks) == 0 && len(ch.Marks) == 0 || reflect.DeepEqual(marks, ch.Marks))
}

// marks is the editor's marks in the stored form.
func (c *openChapter) marks() []workspace.ChapterMark {
	if c.ed == nil {
		return nil
	}
	return storedMarks(c.ed.Doc.Marks())
}

// look reads the chapter's stamp and text from the disk. Other words than
// those last saved wait in external for the author to choose.
func (c *openChapter) look(store *workspace.Store) error {
	st, err := store.ChapterStamp(c.id)
	if err != nil {
		return err
	}
	if st.Size == c.stamp.Size && st.Mod.Equal(c.stamp.Mod) {
		return nil
	}
	ch, err := store.ReadChapter(c.id)
	if err != nil {
		return err
	}
	if ch.Text == workspace.CleanProse(c.saved) {
		c.stamp = ch.Stamp
		return nil
	}
	c.external = &ch
	return nil
}

// save writes the chapter's text when it changed since the disk had it,
// with a snapshot when writing has gone on long enough since the last.
func (c *openChapter) save(store *workspace.Store, b *book.Book, now time.Time) error {
	if c.ed == nil || c.external != nil || c.lost || c.ed.Doc.Version() == c.savedVer {
		return nil
	}
	text := c.ed.Doc.String()
	marks := c.marks()
	stamp, err := b.SaveChapterOver(c.id, text, marks, c.stamp)
	if errors.Is(err, workspace.ErrChangedOutside) {
		// Changed outside since the disk was last looked at. Other words
		// there wait for the author to choose, as when the look finds
		// them; the same words under a new date are written over.
		if err = c.look(store); err != nil {
			return err
		}
		if c.external != nil {
			return nil
		}
		stamp, err = b.SaveChapterOver(c.id, text, marks, c.stamp)
	}
	if err != nil {
		return err
	}
	c.stamp, c.saved, c.savedVer = stamp, text, c.ed.Doc.Version()
	c.changed = true
	if now.Sub(c.lastSnap) >= snapEvery {
		if _, err = store.TakeSnapshot(c.id, workspace.SnapInterval, text, marks); err != nil {
			return err
		}
		c.lastSnap = now
	}
	return nil
}

// takeDisk replaces the chapter's text with the disk's, keeping the text
// shown as a snapshot. The caller builds the editor of the returned text.
func (c *openChapter) takeDisk(store *workspace.Store) (workspace.Chapter, error) {
	ch := c.external
	if ch == nil {
		return workspace.Chapter{}, nil
	}
	if _, err := store.TakeSnapshot(c.id, workspace.SnapExternal, c.ed.Doc.String(), c.marks()); err != nil {
		return workspace.Chapter{}, err
	}
	c.saved, c.savedVer, c.stamp, c.external = ch.Text, -1, ch.Stamp, nil
	return *ch, nil
}

// keepMine writes the chapter's text over the disk's, keeping the disk's
// as a snapshot.
func (c *openChapter) keepMine(store *workspace.Store, b *book.Book, now time.Time) error {
	ch := c.external
	if ch == nil {
		return nil
	}
	if _, err := store.TakeSnapshot(c.id, workspace.SnapExternal, ch.Text, ch.Marks); err != nil {
		return err
	}
	c.external, c.savedVer, c.stamp = nil, -1, ch.Stamp
	return c.save(store, b, now)
}

// saveLost saves the chapter's text as a new document and returns its ID.
func (c *openChapter) saveLost(b *book.Book) (string, error) {
	id, err := b.NewChapter("", c.lostTitle, false)
	if err != nil {
		return "", err
	}
	if _, err = b.SaveChapter(id, c.ed.Doc.String(), c.marks()); err != nil {
		return "", err
	}
	return id, nil
}
