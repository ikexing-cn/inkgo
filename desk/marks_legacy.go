package desk

import (
	"strings"
	"unicode/utf8"
	"writer/desk/book"
	"writer/editor"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/ext/marks/marksapi"
	"writer/locales"
	"writer/workspace"
)

// marksOn is the temporary check prerequisite bridge. Remove it when check moves.
func (a *App) marksOn() bool { _, ok := ext.Lookup(a.extensions, marksapi.Service); return ok }

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
	ext.Contribute(r, marksapi.MarginSources, "check", marksapi.MarginSource{Key: func() any { return a.checkKey() }, Notes: a.checkMarginSource})
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
func (a *App) checkMarginSource(chapter, text string) []marksapi.MarginItem {
	if !a.checksOn() {
		return nil
	}
	var out []marksapi.MarginItem
	for _, it := range a.checkItems(false) {
		if it.node != chapter || it.f.Quote == "" {
			continue
		}
		at := strings.Index(text, it.f.Quote)
		if at < 0 {
			continue
		}
		start := utf8.RuneCountInString(text[:at])
		n := marksapi.MarginItem{ID: "check-" + it.f.ID, Kind: kindLabel(it.f.Kind), Title: it.f.Title, Start: start, End: start + utf8.RuneCountInString(it.f.Quote), Warn: true, Look: editor.MarkCheck, Open: func() { a.ck.tab = 0; a.openCheck(it.f.ID) }}
		switch it.f.Kind {
		case workspace.FindConflict:
			n.Kind, n.Text = locales.NotesKindInconsistent(), locales.NotesSays(it.f.Says)
		case workspace.FindNew:
			n.Text = it.f.Text
		}
		for _, act := range a.checkActions(it) {
			n.Actions = append(n.Actions, marksapi.NoteAction{Label: act.label, Run: act.run})
		}
		out = append(out, n)
	}
	return out
}

func proseMarksCall(p hostapi.ProseMarkProvider) func(hostapi.ProseRequest) []editor.ProseMark {
	return func(in hostapi.ProseRequest) []editor.ProseMark { return p.Marks(in.Chapter, in.Text) }
}
