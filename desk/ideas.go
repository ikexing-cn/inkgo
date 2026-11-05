package desk

import (
	"writer/desk/book"
	"writer/workspace"
)

type ideasKey struct {
	book    *book.Book
	version int
	ai      bool
}

func (a *App) ideas() []workspace.Idea {
	return a.doc.ideas.get(ideasKey{a.book, a.book.Version(), (a.aiOn() && a.hasEntries() && a.marksOn())}, func() []workspace.Idea {
		if !(a.aiOn() && a.hasEntries() && a.marksOn()) {
			return nil
		}
		return (workspace.Setting{Entries: a.entries()}).Ideas(a.book.Tree().Order())
	})
}
