package desk

import (
	"slices"

	"writer/desk/book"
	"writer/editor"
	"writer/workspace"
)

// The names of the setting complete as the prose is typed: those of the
// settled entries of workspace.NameCategories, each other name showing
// whose it is.

// entities are the names that complete.
func (a *App) entities() []editor.Entity {
	var out []editor.Entity
	for _, e := range a.book.Setting().Entries {
		if e.Place != workspace.PlaceBody || len(e.Names) == 0 || !slices.Contains(workspace.NameCategories, e.Category) {
			continue
		}
		for i, n := range e.Names {
			en := editor.Entity{Name: n, Kind: e.Category}
			if i > 0 {
				en.Of = e.Names[0]
			}
			out = append(out, en)
		}
	}
	return out
}

// namesKey is everything the name links read: the setting and whether
// names are shown at all.
type namesKey struct {
	book    *book.Book
	version int
	noNames bool
}

// feedNames gives the chapter's editor the names, again once the setting
// changed.
func (a *App) feedNames() {
	w := &a.write
	if w.cur().ed == nil {
		return
	}
	if w.named != w.cur().ed || w.namedRevs != len(a.book.Setting().Revisions) {
		w.named, w.namedRevs = w.cur().ed, len(a.book.Setting().Revisions)
		w.cur().ed.SetEntities(a.entities())
	}
	key := namesKey{a.book, a.book.Version(), a.shelf.NoNames}
	links := w.nameLinks.get(key, func() []editor.NameLink {
		var links []editor.NameLink
		if a.shelf.NoNames {
			return links
		}
		for _, e := range a.book.Setting().Entries {
			if e.Place == workspace.PlaceBody {
				for _, n := range e.Names {
					links = append(links, editor.NameLink{Name: n, Entry: e.ID})
				}
			}
		}
		return links
	})
	w.cur().ed.SetNameLinks(links, a.seeName)
}

// replaceName opens whole-book replacement using the writing page.
func (a *App) replaceName(old, new string) {
	a.page = pageWrite
	w := &a.write
	if w.cur().ed == nil {
		id := w.node
		if _, ok := a.book.Tree().Node(id); !ok {
			id = a.firstChapter()
		}
		if id != "" {
			a.openNode(id)
		}
	}
	if w.cur().ed == nil {
		return // no chapter to find in
	}
	w.finding, w.query, w.with, w.wide, w.sure = true, old, new, true, false
	w.texts, w.hitsFor = nil, [3]any{}
}
