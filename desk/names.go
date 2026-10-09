package desk

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"writer/editor"
	"writer/workspace"
)

// The names of the setting complete as the prose is typed: those of the
// settled entries of workspace.NameCategories, each other name showing
// whose it is.

// entities are the names that complete.
func (a *App) entities() []editor.Entity {
	var out []editor.Entity
	for _, e := range a.set.Entries {
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

// feedNames gives the chapter's editor the names, again once the setting
// changed.
func (a *App) feedNames() {
	w := &a.write
	if w.ed == nil {
		return
	}
	if w.named != w.ed || w.namedRevs != len(a.set.Revisions) {
		w.named, w.namedRevs = w.ed, len(a.set.Revisions)
		w.ed.SetEntities(a.entities())
	}
}

// named reports whether some entry already has name.
func (a *App) named(name string) bool {
	return slices.ContainsFunc(a.set.Entries, func(e workspace.Entry) bool {
		return e.Place != workspace.PlaceDropped && slices.Contains(e.Names, name)
	})
}

// nameMenu offers to make the selection, a short name, an entry of its
// own in a category of names.
func (a *App) nameMenu(m *ui.Menu, sel string) {
	name := strings.TrimSpace(sel)
	n := utf8.RuneCountInString(name)
	if n == 0 || n > 12 || strings.ContainsAny(name, "\n\r\t") {
		m.Item("加入名字表").Disabled(true)
		return
	}
	if a.named(name) {
		m.Item("已在名字表里").Disabled(true)
		return
	}
	m.Submenu("加入名字表", func(m *ui.Menu) {
		for _, c := range a.Store.Work().Categories {
			if slices.Contains(workspace.NameCategories, c.Name) && m.Item(c.Name).Chosen() {
				a.revise(workspace.Op{Op: workspace.OpAdd, Place: workspace.PlaceBody, Category: c.Name, Text: name, Names: []string{name}})
				if a.Error == "" {
					a.notice = "「" + name + "」已加入设定的" + c.Name + "，打字时可以补全"
				}
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
	t := a.pal.theme
	ui.Row(c).FillWidth().Gap(6).Padding(6, 8, 6, 10).Radius(8).Background(a.pal.raised).Border(1, a.pal.line).AlignItems(ui.Center).Children(func() {
		ui.Textf(c, "正文里还有 %d 处「%s」", r.count, r.old).FontSize(12.5).Grow(1).Shrink(1)
		if ghost(c, "全书替换", t.Text).Label("把旧名换成新名").Clicked() {
			a.renamed = renamed{}
			a.replaceName(r.old, r.new)
		}
		if iconButton(c, icX, "不换旧名", ui.Color{}).Tooltip("不换").Clicked() {
			a.renamed = renamed{}
		}
	})
}

// replaceName opens the find bar on the writing page, finding old in the
// whole book with new to replace it.
func (a *App) replaceName(old, new string) {
	if !a.write.entered {
		a.write.entered = true
		a.enterWriting()
	}
	a.page = 3
	w := &a.write
	if w.ed == nil {
		return // no chapter to find in
	}
	w.finding, w.query, w.with, w.wide, w.sure = true, old, new, true, false
	w.texts, w.hitsFor = nil, [3]any{}
}
