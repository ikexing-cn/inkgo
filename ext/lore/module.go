package lore

import (
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/context/contextapi"
	"writer/ext/hostapi"
	"writer/ext/lore/loreapi"
	"writer/ext/lore/loretext"
	"writer/workspace"
)

// Module registers the setting interface and its live reference entries.
var Module = ext.Module{ID: "lore", Kind: ext.Feature, Group: hostapi.GroupLore, Requires: []string{"context"}, Optional: []string{"ai"}, Name: loretext.ModLore, Desc: loretext.ModLoreAbout, Setup: setup}

func setup(r *ext.Registrar) {
	for _, tool := range TalkTools() {
		ext.Contribute(r, aiapi.Tools, tool.Name, tool)
	}

	p := &Panel{}
	book := ext.Use(r, hostapi.BookService)
	palette := ext.Use(r, hostapi.PaletteService)
	notices := ext.Use(r, hostapi.NoticesService)
	desktop := ext.Use(r, hostapi.DesktopService)
	talk := ext.Use(r, hostapi.TalkService)
	search := ext.Use(r, hostapi.SearchService)
	ai := ext.Use(r, hostapi.AIService)
	edits := ext.Use(r, hostapi.RevisionsService)
	pane := ext.Use(r, hostapi.PaneService)
	r.OnStart(func(*ext.Session) error {
		*p = Panel{services: Services{Book: book.Get(), Palette: palette.Get(), Notices: notices.Get(), Desktop: desktop.Get(), Talk: talk.Get(), Search: search.Get(), AI: ai.Get(), Edits: edits.Get(), Pane: pane.Get()}}
		return nil
	})
	r.OnStop(func(*ext.Session) {
		if p.services.Book.Content() != nil {
			p.services.Notices.Notices().SetError(p.SaveEdits())
		}
		*p = Panel{}
	})
	ext.Provide(r, loreapi.Active, func(*ext.Session) struct{} { return struct{}{} })
	ext.Contribute(r, contextapi.Entries, "entries", contextapi.EntryProvider(func(struct{}) []workspace.Entry {
		if p.services.Book.Content() == nil {
			return []workspace.Entry{}
		}
		return p.services.Book.Content().Setting().Entries
	}))
	register(r, p)
}
