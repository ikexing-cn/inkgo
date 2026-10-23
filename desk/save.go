package desk

import (
	"errors"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// The writing page saves as the author pauses, looks at the disk for
// changes made outside, and keeps a snapshot every so often while they
// write. Tests shorten these.
var (
	saveAfter  = 1500 * time.Millisecond
	checkEvery = 2 * time.Second
	snapEvery  = 10 * time.Minute
)

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
	a.status.did("已保存。停笔片刻也会自动保存")
}

// saveText writes the text shown when it changed since the disk had it,
// with a snapshot when writing has gone on long enough since the last.
func (a *App) saveText() error {
	w := &a.write
	if w.cur().ed == nil || w.cur().external != nil || w.cur().lost || w.cur().ed.Doc.Version() == w.cur().savedVer {
		return nil
	}
	text := w.cur().ed.Doc.String()
	marks := storedMarks(w.cur().ed.Doc.Marks())
	stamp, err := a.book.SaveChapterOver(w.node, text, marks, w.cur().stamp)
	if errors.Is(err, workspace.ErrChangedOutside) {
		// Changed outside since the disk was last looked at. Other words
		// there wait for the author to choose, as when the look finds
		// them; the same words under a new date are written over.
		if a.lookAtText(); w.cur().external != nil {
			return nil
		}
		stamp, err = a.book.SaveChapterOver(w.node, text, marks, w.cur().stamp)
	}
	if err != nil {
		return err
	}
	w.cur().stamp, w.cur().saved, w.cur().savedVer = stamp, text, w.cur().ed.Doc.Version()
	w.cur().changed = true
	if time.Since(w.cur().lastSnap) >= snapEvery {
		if _, err = a.Store.TakeSnapshot(w.node, workspace.SnapInterval, text, marks); err != nil {
			return err
		}
		w.cur().lastSnap = time.Now()
	}
	return nil
}

func (a *App) saveOutline() error {
	w := &a.write
	if w.node == "" || w.cur().outline == w.cur().outlineSaved || w.cur().lost {
		return nil
	}
	if err := a.book.SaveOutline(w.node, w.cur().outline); err != nil {
		return err
	}
	w.cur().outlineSaved = w.cur().outline
	return nil
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
	if w.cur().ed != nil && w.cur().ed.Doc.Version() != w.cur().savedVer && w.cur().external == nil && !w.cur().lost {
		if wait := saveAfter - time.Since(w.cur().ed.LastEdit()); wait > 0 {
			c.After(wait)
		} else {
			a.warn(a.saveText())
		}
	}
	if w.cur().outline != w.cur().outlinePrev {
		w.cur().outlinePrev, w.cur().outlineAt = w.cur().outline, time.Now()
	}
	if w.cur().outline != w.cur().outlineSaved {
		if wait := saveAfter - time.Since(w.cur().outlineAt); wait > 0 {
			c.After(wait)
		} else {
			a.warn(a.saveOutline())
		}
	}
	if wait := checkEvery - time.Since(w.checked); wait > 0 {
		c.After(wait)
		return
	}
	w.checked = time.Now()
	c.After(checkEvery)
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
	if w.node == "" || w.cur().lost {
		return
	}
	n, ok := a.book.Tree().Node(w.node)
	if !ok {
		w.cur().lost, w.cur().lostTitle = true, "找回的章节"
		if old != nil {
			w.cur().lostTitle = old.Title
		}
		return
	}
	if n.Folder || w.cur().external != nil {
		return
	}
	a.lookAtText()
}

// lookAtText reads the text of the chapter shown when the disk's is not
// the version last seen. Other words than those last saved wait in
// w.cur().external for the author to choose between them and the editor's.
func (a *App) lookAtText() {
	w := &a.write
	st, err := a.Store.ChapterStamp(w.node)
	if err != nil {
		a.warn(err)
		return
	}
	if st.Size == w.cur().stamp.Size && st.Mod.Equal(w.cur().stamp.Mod) {
		return
	}
	ch, err := a.Store.ReadChapter(w.node)
	if err != nil {
		a.warn(err)
		return
	}
	if ch.Text == workspace.CleanProse(w.cur().saved) {
		w.cur().stamp = ch.Stamp
		return
	}
	w.cur().external = &ch
}

// takeDisk replaces the text shown with the disk's, keeping the text
// shown as a snapshot.
func (a *App) takeDisk() {
	w := &a.write
	ch := w.cur().external
	if ch == nil {
		return
	}
	_, err := a.Store.TakeSnapshot(w.node, workspace.SnapExternal, w.cur().ed.Doc.String(), storedMarks(w.cur().ed.Doc.Marks()))
	if err != nil {
		a.warn(err)
		return
	}
	w.cur().ed = a.chapterEditor(ch.Text, ch.Marks)
	w.cur().saved, w.cur().savedVer, w.cur().stamp, w.cur().external = ch.Text, w.cur().ed.Doc.Version(), ch.Stamp, nil
	a.status.did("已采用磁盘版本，编辑器原内容已存为快照")
}

// keepMine writes the text shown over the disk's, keeping the disk's as a
// snapshot.
func (a *App) keepMine() {
	w := &a.write
	ch := w.cur().external
	if ch == nil {
		return
	}
	if _, err := a.Store.TakeSnapshot(w.node, workspace.SnapExternal, ch.Text, ch.Marks); err != nil {
		a.warn(err)
		return
	}
	w.cur().external, w.cur().savedVer, w.cur().stamp = nil, -1, ch.Stamp
	a.warn(a.saveText())
	a.status.did("已保留编辑器版本，磁盘内容已存为快照")
}

// saveLost saves the text of a node gone from the disk as a new document.
func (a *App) saveLost() {
	w := &a.write
	id, err := a.book.NewChapter("", w.cur().lostTitle, false)
	if err == nil {
		_, err = a.book.SaveChapter(id, w.cur().ed.Doc.String(), storedMarks(w.cur().ed.Doc.Marks()))
	}
	if err != nil {
		a.warn(err)
		return
	}
	// The new chapter takes the tab of the one that was lost.
	if i := slices.Index(w.tabs, w.node); i >= 0 {
		w.tabs[i] = id
	}
	w.cur().ed, w.cur().lost = nil, false
	a.openNode(id)
	a.status.did("已存为新章节，位于最上层末尾")
}
