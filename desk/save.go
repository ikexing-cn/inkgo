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
	if w.ed == nil || w.external != nil || w.lost || w.ed.Doc.Version() == w.savedVer {
		return nil
	}
	text := w.ed.Doc.String()
	marks := storedMarks(w.ed.Doc.Marks())
	stamp, err := a.Store.WriteChapterOver(w.node, text, marks, w.stamp)
	if errors.Is(err, workspace.ErrChangedOutside) {
		// Changed outside since the disk was last looked at. Other words
		// there wait for the author to choose, as when the look finds
		// them; the same words under a new date are written over.
		if a.lookAtText(); w.external != nil {
			return nil
		}
		stamp, err = a.Store.WriteChapterOver(w.node, text, marks, w.stamp)
	}
	if err != nil {
		return err
	}
	w.stamp, w.saved, w.savedVer = stamp, text, w.ed.Doc.Version()
	w.changed = true
	if w.counts != nil {
		w.counts[w.node] = workspace.Count(text)
	}
	if time.Since(w.lastSnap) >= snapEvery {
		if _, err = a.Store.TakeSnapshot(w.node, workspace.SnapInterval, text, marks); err != nil {
			return err
		}
		w.lastSnap = time.Now()
	}
	return nil
}

func (a *App) saveOutline() error {
	w := &a.write
	if w.node == "" || w.outline == w.outlineSaved || w.lost {
		return nil
	}
	if err := a.Store.WriteOutline(w.node, w.outline); err != nil {
		return err
	}
	w.outlineSaved = w.outline
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
	if w.ed != nil && w.ed.Doc.Version() != w.savedVer && w.external == nil && !w.lost {
		if wait := saveAfter - time.Since(w.ed.LastEdit()); wait > 0 {
			c.After(wait)
		} else {
			a.warn(a.saveText())
		}
	}
	if w.outline != w.outlinePrev {
		w.outlinePrev, w.outlineAt = w.outline, time.Now()
	}
	if w.outline != w.outlineSaved {
		if wait := saveAfter - time.Since(w.outlineAt); wait > 0 {
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
	old, _ := w.tree.Node(w.node)
	if err := a.loadTree(); err != nil {
		a.warn(err)
		return
	}
	defer a.keepTabs()
	if w.node == "" || w.lost {
		return
	}
	n, ok := w.tree.Node(w.node)
	if !ok {
		w.lost, w.lostTitle = true, "找回的章节"
		if old != nil {
			w.lostTitle = old.Title
		}
		return
	}
	if n.Folder || w.external != nil {
		return
	}
	a.lookAtText()
}

// lookAtText reads the text of the chapter shown when the disk's is not
// the version last seen. Other words than those last saved wait in
// w.external for the author to choose between them and the editor's.
func (a *App) lookAtText() {
	w := &a.write
	st, err := a.Store.ChapterStamp(w.node)
	if err != nil {
		a.warn(err)
		return
	}
	if st.Size == w.stamp.Size && st.Mod.Equal(w.stamp.Mod) {
		return
	}
	ch, err := a.Store.ReadChapter(w.node)
	if err != nil {
		a.warn(err)
		return
	}
	if ch.Text == workspace.CleanProse(w.saved) {
		w.stamp = ch.Stamp
		return
	}
	w.external = &ch
}

// takeDisk replaces the text shown with the disk's, keeping the text
// shown as a snapshot.
func (a *App) takeDisk() {
	w := &a.write
	ch := w.external
	if ch == nil {
		return
	}
	_, err := a.Store.TakeSnapshot(w.node, workspace.SnapExternal, w.ed.Doc.String(), storedMarks(w.ed.Doc.Marks()))
	if err != nil {
		a.warn(err)
		return
	}
	w.ed = a.chapterEditor(ch.Text, ch.Marks)
	w.saved, w.savedVer, w.stamp, w.external = ch.Text, w.ed.Doc.Version(), ch.Stamp, nil
	if w.counts != nil {
		w.counts[w.node] = workspace.Count(ch.Text)
	}
	a.status.did("已采用磁盘版本，编辑器原内容已存为快照")
}

// keepMine writes the text shown over the disk's, keeping the disk's as a
// snapshot.
func (a *App) keepMine() {
	w := &a.write
	ch := w.external
	if ch == nil {
		return
	}
	if _, err := a.Store.TakeSnapshot(w.node, workspace.SnapExternal, ch.Text, ch.Marks); err != nil {
		a.warn(err)
		return
	}
	w.external, w.savedVer, w.stamp = nil, -1, ch.Stamp
	a.warn(a.saveText())
	a.status.did("已保留编辑器版本，磁盘内容已存为快照")
}

// saveLost saves the text of a node gone from the disk as a new document.
func (a *App) saveLost() {
	w := &a.write
	id, err := a.Store.NewChapter("", w.lostTitle, false)
	if err == nil {
		_, err = a.Store.WriteChapter(id, w.ed.Doc.String(), storedMarks(w.ed.Doc.Marks()))
	}
	if err != nil {
		a.warn(err)
		return
	}
	// The new chapter takes the tab of the one that was lost.
	if i := slices.Index(w.tabs, w.node); i >= 0 {
		w.tabs[i] = id
	}
	w.ed, w.lost = nil, false
	a.warn(a.loadTree())
	a.openNode(id)
	a.status.did("已存为新章节，位于最上层末尾")
}
