package lore

import (
	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/context/contextapi"
	"writer/ext/find/findapi"
	"writer/ext/hostapi"
	"writer/ext/lore/loreapi"
	"writer/ext/lore/loretext"
	"writer/ext/talk/talkapi"
	"writer/workspace"
)

// Module registers the setting interface and its live reference entries.
var Module = ext.Module{ID: "lore", Kind: ext.Feature, Group: hostapi.GroupLore, Requires: []string{"context"}, Optional: []string{"ai", "talk", "find", "marks"}, Name: loretext.ModLore, Desc: loretext.ModLoreAbout, Setup: setup}

func setup(r *ext.Registrar) {
	for _, tool := range TalkTools() {
		ext.Contribute(r, aiapi.Tools, tool.Name, tool)
	}

	replacement := ext.UseOptional(r, findapi.ReplacementService)
	p := &Panel{}
	book := ext.Use(r, hostapi.BookService)
	bookEditing := ext.Use(r, hostapi.EditingService)
	palette := ext.Use(r, hostapi.PaletteService)
	notices := ext.Use(r, hostapi.NoticesService)
	desktop := ext.Use(r, hostapi.NavigationService)
	desktopNative := ext.Use(r, hostapi.NativeUIService)
	desktopPaths := ext.Use(r, hostapi.MachinePathsService)
	talk := ext.UseOptional(r, talkapi.Service)
	search := ext.Use(r, hostapi.SearchService)
	ai := ext.Use(r, hostapi.ExtensionsService)
	pane := ext.Use(r, hostapi.PaneService)
	r.OnStart(func(*ext.Session) error {
		*p = Panel{replacement: replacement, services: Services{Book: book.Get(), Editing: bookEditing.Get(), Palette: palette.Get(), Notices: notices.Get(), Desktop: desktop.Get(), Native: desktopNative.Get(), Paths: desktopPaths.Get(), Talk: talk.Get, Search: search.Get(), AI: ai.Get(), Pane: pane.Get()}}
		if p.services.Book.Content() != nil {
			p.seen = len(p.services.Book.Content().Setting().Revisions)
		}
		return nil
	})
	r.OnStop(func(*ext.Session) {
		if p.services.Book.Content() != nil {
			p.services.Notices.Notices().SetError(p.SaveEdits())
		}
		*p = Panel{}
	})
	ext.Contribute(r, hostapi.State, "refresh", hostapi.ViewState{Refresh: p.flashNew})
	ext.Provide(r, loreapi.RevisionsService, func(*ext.Session) loreapi.Revisions { return p })
	ext.Provide(r, loreapi.SelectionService, func(*ext.Session) loreapi.Selection { return p })
	ext.Contribute(r, hostapi.Overlays, "take", hostapi.Overlay{Open: p.SelectionOpen, Draw: func(c *ui.Context) {
		if p.Take.Open {
			p.take(c)
		}
	}})
	ext.Provide(r, loreapi.Active, func(*ext.Session) struct{} { return struct{}{} })
	ext.Contribute(r, contextapi.Entries, "entries", contextapi.EntryProvider(func(struct{}) []workspace.Entry {
		if p.services.Book.Content() == nil {
			return []workspace.Entry{}
		}
		return p.services.Book.Content().Setting().Entries
	}))
	register(r, p)
}
