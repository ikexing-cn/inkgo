package desk

import (
	"slices"
	"strings"
	"unicode/utf8"
	"writer/locales"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
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

func (a *App) seeName(id string) {
	a.doc.hidden, a.write.focus, a.doc.tab = false, false, paneSetting
	a.doc.route.jumpToEntry(id)
}

// named reports whether some entry already has name.
func (a *App) named(name string) bool {
	return slices.ContainsFunc(a.book.Setting().Entries, func(e workspace.Entry) bool {
		return e.Place != workspace.PlaceDropped && slices.Contains(e.Names, name)
	})
}

// nameMenu offers to make the selection, a short name, an entry of its
// own in a category of names.
func (a *App) nameMenu(m *kit.Menu, sel string) {
	name := strings.TrimSpace(sel)
	n := utf8.RuneCountInString(name)
	if n == 0 || n > 12 || strings.ContainsAny(name, "\n\r\t") {
		m.Item(locales.EditorAddName()).Disabled(true)
		return
	}
	if a.named(name) {
		m.Item(locales.EditorAlreadyNamed()).Disabled(true)
		return
	}
	m.Submenu(locales.EditorAddName(), func(m *kit.Menu) {
		for _, c := range a.Store.Work().Categories {
			if slices.Contains(workspace.NameCategories, c.Name) {
				m.Item(c.Name).Do(func() {
					a.revise(workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: c.Name, Text: name, Names: []string{name}})
					if a.errorText() == "" {
						a.status.Did(locales.EditorAddedToLore(name, c.Name))
					}
				})
			}
		}
	})
}

// splitNames reads names as the author writes them, apart by spaces or
// the marks that list things.
func splitNames(s string) []string {
	return workspace.CleanNames(strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '　' || strings.ContainsRune("、,，;；/|", r)
	}))
}

// renamed is the name an entry was called before the author renamed it,
// while the prose still says it.
type renamed struct {
	old, new string
	count    int
}

// noteRename notes that the author renamed old to new, if the prose
// still says old somewhere.
func (a *App) noteRename(old, new string) {
	a.renamed = renamed{}
	if old == "" || old == new || a.Store == nil {
		return
	}
	w := &a.write
	if a.loadTree() != nil {
		return
	}
	keep := w.texts
	w.texts = nil
	n := 0
	for _, h := range a.bookHits(old) {
		n += h.Count
	}
	w.texts = keep
	if n > 0 {
		a.renamed = renamed{old: old, new: new, count: n}
	}
}

// renameNotice says the prose still has the old name, and replaces it
// through the book on a click, after the usual confirmation.
func (a *App) renameNotice(c *ui.Context) {
	r := a.renamed
	if r.count == 0 {
		return
	}
	ui.Row(c).FillWidth().Gap(6).Padding(6, 8, 6, 10).Radius(kit.RCard).Background(a.pal.Raised).Border(1, a.pal.Line).AlignItems(ui.Center).Children(func() {
		ui.Text(c, locales.LoreRenamedLeft(r.count, r.old)).FontSize(kit.FsNote).Grow(1).Shrink(1)
		if a.pal.Ghost(c, locales.LoreReplaceAll(), ui.Color{}).Label("将旧名替换为新名").Clicked() {
			a.renamed = renamed{}
			a.replaceName(r.old, r.new)
		}
		if kit.IconButton(c, kit.IcX, locales.LoreKeepOld(), ui.Color{}).Tooltip(locales.LoreKeepOldTip()).Clicked() {
			a.renamed = renamed{}
		}
	})
}

// replaceName opens the find bar on the writing page, finding old in the
// whole book with new to replace it.
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
