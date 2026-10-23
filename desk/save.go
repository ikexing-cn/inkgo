package desk

import (
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
)

// snapEvery is how often the writing page keeps a snapshot while the author
// writes. Tests shorten it.
var snapEvery = 10 * time.Minute

// flush saves the text and the outline shown, as leaving them does. What
// an AI proposes is not in the text, and stays.
func (a *App) flush() {
	if a.Store == nil {
		return
	}
	a.keepCaret()
	a.warn(a.saveText())
	a.warn(a.saveOutline())
}

// saveNow is Ctrl+S: what the page shows goes to the disk now, though it
// would anyway as the author pauses. What the AI writes is not in the text
// until taken, so nothing waits for it.
func (a *App) saveNow() {
	for _, err := range []error{a.saveText(), a.saveOutline()} {
		if err != nil {
			a.warn(err)
			return
		}
	}
	a.keepCaret()
	a.status.Did("已保存。停笔片刻也会自动保存")
}

// saveText writes the text shown when it changed since the disk had it,
// with a snapshot when writing has gone on long enough since the last.
func (a *App) saveText() error {
	w := &a.write
	return w.cur().save(a.Store, a.book, time.Now())
}

func (a *App) saveOutline() error {
	w := &a.write
	if w.node == "" || w.cur().lost {
		return nil
	}
	return w.cur().outline.Save(a.book)
}

// watch saves what paused long enough and looks at the disk, each frame
// of the writing page.
func (a *App) watch(c *ui.Context) {
	if a.rollDay(time.Now()) {
		a.savePrefs()
	}
	w := &a.write
	if len(w.closing) > 0 {
		return // asked whether to save: nothing is saved until the answer
	}
	o := w.cur()
	if o.ed != nil && o.ed.Doc.Version() != o.savedVer && o.external == nil && !o.lost {
		if wait := book.SaveAfter - time.Since(o.ed.LastEdit()); wait > 0 {
			c.After(wait)
		} else {
			a.warn(a.saveText())
		}
	}
	d := &o.outline
	d.Change(time.Now())
	if wait, due := d.Waits(book.SaveAfter, time.Now()); due {
		if wait > 0 {
			c.After(wait)
		} else {
			a.warn(a.saveOutline())
		}
	}
	if wait := book.CheckEvery - time.Since(w.checked); wait > 0 {
		c.After(wait)
		return
	}
	w.checked = time.Now()
	c.After(book.CheckEvery)
	a.checkDisk()
}

// checkDisk rereads the tree, which may have changed outside, and the
// text of the node shown.
func (a *App) checkDisk() {
	w := &a.write
	old, _ := a.book.Tree().Node(w.node)
	if err := a.loadTree(); err != nil {
		a.warn(err)
		return
	}
	defer a.keepTabs()
	o := w.cur()
	if w.node == "" || o.lost {
		return
	}
	n, ok := a.book.Tree().Node(w.node)
	if !ok {
		o.lost, o.lostTitle = true, "找回的章节"
		if old != nil {
			o.lostTitle = old.Title
		}
		return
	}
	if n.Folder || o.external != nil {
		return
	}
	a.lookAtText()
}

// lookAtText reads the text of the chapter shown when the disk's is not
// the version last seen. Other words than those last saved wait in the
// chapter's external for the author to choose between them and the
// editor's.
func (a *App) lookAtText() {
	w := &a.write
	if err := w.cur().look(a.Store); err != nil {
		a.warn(err)
	}
}

// takeDisk replaces the text shown with the disk's, keeping the text
// shown as a snapshot.
func (a *App) takeDisk() {
	w := &a.write
	c := w.cur()
	if c.external == nil {
		return
	}
	ch, err := c.takeDisk(a.Store)
	if err != nil {
		a.warn(err)
		return
	}
	c.ed = a.chapterEditor(ch.Text, ch.Marks)
	c.savedVer = c.ed.Doc.Version()
	a.status.Did("已采用磁盘版本，编辑器原内容已存为快照")
}

// keepMine writes the text shown over the disk's, keeping the disk's as a
// snapshot.
func (a *App) keepMine() {
	w := &a.write
	if w.cur().external == nil {
		return
	}
	if err := w.cur().keepMine(a.Store, a.book, time.Now()); err != nil {
		a.warn(err)
		return
	}
	a.status.Did("已保留编辑器版本，磁盘内容已存为快照")
}

// saveLost saves the text of a node gone from the disk as a new document.
func (a *App) saveLost() {
	w := &a.write
	c := w.cur()
	id, err := c.saveLost(a.book)
	if err != nil {
		a.warn(err)
		return
	}
	// The new chapter takes the tab of the one that was lost.
	old := w.node
	if i := slices.Index(w.tabs, old); i >= 0 {
		w.tabs[i] = id
	}
	delete(w.open, old)
	c.ed, c.lost = nil, false
	a.openNode(id)
	a.status.Did("已存为新章节，位于最上层末尾")
}
