package desk

import (
	"writer/desk/book"
	"writer/editor"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/ext/marks/marksapi"
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

func registerLegacyMarks(r *ext.Registrar, a *App) {
	ext.Contribute(r, marksapi.MarginSources, "ai-written", marksapi.MarginSource{Key: func() any {
		ed := a.write.cur().ed
		version := 0
		if ed != nil {
			version = ed.Doc.Version()
		}
		return struct {
			ed      *editor.Editor
			version int
			ai      bool
		}{ed, version, a.aiOn()}
	}, Notes: func(chapter, text string) []marksapi.MarginItem {
		if !a.aiOn() || a.write.cur().ed == nil {
			return nil
		}
		var out []marksapi.MarginItem
		for _, m := range a.write.cur().ed.Doc.Marks() {
			if m.Kind == editor.MarkAI && !m.Orphaned {
				out = append(out, marksapi.MarginItem{Start: m.Start, End: m.End, Look: editor.MarkAI})
			}
		}
		return out
	}})
}
func proseMarksCall(p hostapi.ProseMarkProvider) func(hostapi.ProseRequest) []editor.ProseMark {
	return func(in hostapi.ProseRequest) []editor.ProseMark { return p.Marks(in.Chapter, in.Text) }
}

func (a *App) marksOn() bool { return a.extensions.Ledger("marks").State == ext.On }
