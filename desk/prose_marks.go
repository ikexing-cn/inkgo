package desk

import (
	"writer/desk/book"
	"writer/editor"
	"writer/ext"
	"writer/ext/hostapi"
)

type proseKey struct {
	book     *book.Book
	ed       *editor.Editor
	doc      *editor.Doc
	version  int
	chapter  string
	changes  uint64
	revision uint64
}

type proseFeed struct {
	marks     cached[proseKey, []editor.ProseMark]
	changes   uint64
	revisions []uint64
	serial    uint64
}

func (a *App) feedMarks() {
	ed := a.write.cur().ed
	if ed == nil {
		return
	}
	a.syncExtensions()
	c := &a.write.prose
	if c.serial == 0 || c.changes != a.extViews.changes {
		c.changes = a.extViews.changes
		c.revisions = make([]uint64, len(a.extViews.prose))
		c.serial++
	}
	for i, item := range a.extViews.prose {
		if item.Value.Revision != nil {
			_ = ext.Guard(a.extensions, item.Owner, item.ID, func() {
				revision := item.Value.Revision()
				if c.revisions[i] != revision {
					c.revisions[i] = revision
					c.serial++
				}
			})
		}
	}
	key := proseKey{a.book, ed, ed.Doc, ed.Doc.Version(), a.write.node, c.changes, c.serial}
	c.marks.get(key, func() []editor.ProseMark {
		var out []editor.ProseMark
		text := ed.Doc.String()
		for _, item := range a.extViews.prose {
			v, err := ext.Call(a.extensions, ext.HookOf(ext.CallPoint[hostapi.ProseMarkProvider, hostapi.ProseRequest, []editor.ProseMark]{Point: hostapi.ProseMarks}, item.ID), hostapi.ProseRequest{Chapter: a.write.node, Text: text}, proseMarksCall(item.Value))
			if err == nil {
				out = append(out, v...)
			}
		}
		ed.SetProseMarks(out)
		return out
	})
}

func proseMarksCall(p hostapi.ProseMarkProvider) func(hostapi.ProseRequest) []editor.ProseMark {
	return func(in hostapi.ProseRequest) []editor.ProseMark { return p.Marks(in.Chapter, in.Text) }
}

func (a *App) marksOn() bool { return a.extensions.Ledger("marks").State == ext.On }
