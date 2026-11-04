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
	return a.doc.ideas.get(ideasKey{a.book, a.book.Version(), a.aiOn()}, func() []workspace.Idea {
		if !a.aiOn() {
			return nil
		}
		return a.book.Setting().Ideas(a.book.Tree().Order())
	})
}
