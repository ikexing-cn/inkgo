// Package names supplies setting name completion and highlights.
package names

import (
	"slices"
	"writer/desk/book"
	"writer/editor"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/ext/lore/loreapi"
	"writer/ext/names/namestext"
	"writer/workspace"
)

// Module registers the name provider and its local display preference.
var Module = ext.Module{ID: "names", Kind: ext.Feature, Group: hostapi.GroupWrite, Requires: []string{"lore"}, Name: namestext.ModNames, Desc: namestext.ModNamesAbout, Setup: func(r *ext.Registrar) {
	book := ext.Use(r, hostapi.BookService)
	nav := ext.Use(r, loreapi.EntryNavigationService)
	show := ext.Setting(r, "highlights", ext.Schema[bool]{Title: namestext.SettingsMarkNames, Default: true, Scope: ext.Local})
	cache := nameCache{open: func(id string) { nav.Get().Navigation.Jump(id) }}
	ext.Contribute(r, hostapi.NameHighlights, "names", hostapi.NameProvider(func(struct{}) hostapi.Names {
		return cache.get(book.Get().Content(), show.Get())
	}))
}}

// namesKey covers the borrowed setting, including reloads without a new revision.
type namesKey struct {
	book                *book.Book
	version, revisions  int
	highlights, entries bool
}

type nameCache struct {
	key    namesKey
	value  hostapi.Names
	open   func(string)
	builds uint64
}

func (c *nameCache) get(b *book.Book, highlights bool) hostapi.Names {
	key := namesKey{book: b, highlights: highlights}
	if b != nil {
		key.version, key.revisions = b.Version(), len(b.Setting().Revisions)
		key.entries = len(b.Setting().Entries) != 0
	}
	if c.builds != 0 && c.key == key {
		return c.value
	}
	c.key, c.builds = key, c.builds+1
	out := hostapi.Names{Revision: c.builds, Open: c.open}
	if b != nil {
		for _, e := range b.Setting().Entries {
			if e.Place != workspace.PlaceBody {
				continue
			}
			for i, n := range e.Names {
				if slices.Contains(workspace.NameCategories, e.Category) {
					en := editor.Entity{Name: n, Kind: e.Category}
					if i > 0 {
						en.Of = e.Names[0]
					}
					out.Entities = append(out.Entities, en)
				}
				if highlights {
					out.Links = append(out.Links, editor.NameLink{Name: n, Entry: e.ID})
				}
			}
		}
	}
	c.value = out
	return out
}
