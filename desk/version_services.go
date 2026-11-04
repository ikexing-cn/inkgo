package desk

import (
	"writer/editor"
	"writer/workspace"
)

// SaveText saves the selected editor on the frame thread.
func (h extensionHost) SaveText() error { return h.a.saveText() }

// RestoreSnapshot replaces and saves the selected editor on the frame thread.
func (h extensionHost) RestoreSnapshot(sn workspace.Snapshot, text string) error {
	a := h.a
	w := &a.write
	if err := a.saveText(); err != nil {
		return err
	}
	if _, err := a.Store.TakeSnapshot(w.node, workspace.SnapRestore, w.cur().ed.Doc.String(), storedMarks(w.cur().ed.Doc.Marks())); err != nil {
		return err
	}
	w.cur().ed = a.chapterEditor(text, sn.Marks)
	w.cur().ed.Select(0, 0)
	w.cur().savedVer = -1
	return a.saveText()
}

// RestoreTrash restores a node and refreshes book state on the frame thread.
func (h extensionHost) RestoreTrash(id string) error { return h.a.book.RestoreChapter(id) }

// PurgeTrash purges a node and refreshes book state on the frame thread.
func (h extensionHost) PurgeTrash(id string) error { return h.a.book.PurgeChapter(id) }

// ModelLabel describes an AI model on the frame thread.
func (h extensionHost) ModelLabel(r workspace.AIRecord) string { return h.a.modelLabel(r) }

// ChapterExternal reports a pending external edit on the frame thread.
func (h extensionHost) ChapterExternal() bool { return h.a.write.cur().external != nil }

// ReplaceBookText snapshots and replaces book prose on the frame thread.
func (h extensionHost) ReplaceBookText(q, to string) (chapters, total int, here bool, err error) {
	a := h.a
	if q == "" {
		return
	}
	if err = a.saveText(); err != nil {
		return
	}
	var ids []string
	a.book.Tree().Walk(func(n *workspace.Node, _ int) {
		if !n.Folder {
			ids = append(ids, n.ID)
		}
	})
	for _, id := range ids {
		ch, e := a.Store.ReadChapter(id)
		if e != nil {
			err = e
			return
		}
		ed := a.write.cur().ed
		text := ch.Text
		current := id == a.write.node && ed != nil
		if current {
			text = ed.Doc.String()
		}
		doc := editor.NewDoc(text)
		doc.LoadMarks(editorMarks(ch.Marks))
		if len(doc.Find(q)) == 0 {
			continue
		}
		if _, err = a.Store.TakeSnapshot(id, workspace.SnapReplace, text, ch.Marks); err != nil {
			return
		}
		n := 0
		if current {
			n = ed.ReplaceAll(q, to)
			here = true
			err = a.saveText()
		} else {
			n = doc.ReplaceAll(q, to)
			_, err = a.book.SaveChapterOver(id, doc.String(), storedMarks(doc.Marks()), ch.Stamp)
		}
		if err != nil {
			return
		}
		chapters++
		total += n
	}
	return
}
