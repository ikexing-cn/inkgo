// Package lore owns the setting panel and its authoring interactions.
package lore

import (
	"strings"
	"time"
	"writer/ext"
	"writer/ext/find/findapi"
	"writer/ext/hostapi"
	"writer/ext/lore/loreapi"
	"writer/ext/lore/loretext"
	"writer/ext/talk/talkapi"
	"writer/workspace"
)

const flashFor = 3 * time.Second

type renamed struct {
	old, new string
	count    int
}
type noteAction struct {
	label string
	run   func()
}

// Services contains the frame-thread host capabilities used by the panel.
type Services struct {
	Book    hostapi.Book
	Palette hostapi.Palette
	Notices hostapi.Notices
	Desktop hostapi.Desktop
	Talk    func() (talkapi.ServiceAPI, bool)
	Search  hostapi.Search
	AI      hostapi.AI
	Edits   hostapi.Revisions
	Pane    hostapi.Pane
}

// Panel owns one work's entry routes and form drafts.
type Panel struct {
	replacement *ext.OptRef[findapi.Replacement]
	services    Services
	Doc         Form
	renamed     renamed
}

// Form holds the entry panel draft and route.
type Form struct {
	Query                             string
	Route                             Route
	From, Text, Category, Name, Names string
	ideas                             cached[ideasKey, []workspace.Idea]
}

// register contributes the setting interface under the registrar's owner.
func register(r *ext.Registrar, p *Panel) {
	ext.Contribute(r, hostapi.PaneTabs, "setting", hostapi.PaneTab{Key: "setting", Title: loretext.PaneTabLore, Draw: p.body, Before: "talk", BeforeFallback: "fore", Fallback: true, Count: func() int { return len(p.ideas()) }})
	ext.Contribute(r, hostapi.BesideItems, "setting-column", hostapi.DrawItem{Draw: p.settingPanel})
	ext.Contribute(r, hostapi.SelectionMenus, "setting-name", hostapi.SelectionMenu{Build: p.nameMenu})
	ext.Contribute(r, hostapi.MarginSources, "setting-changes", hostapi.MarginSource{Notes: p.marginNotes})
	ext.Provide(r, loreapi.EntryNavigationService, func(*ext.Session) loreapi.EntryNavigator {
		return loreapi.EntryNavigator{Owner: "lore", Navigation: p}
	})
	ext.Contribute(r, hostapi.SaveEdits, "setting-save", p.SaveEdits)
	ext.Contribute(r, hostapi.BeforeLeave, "setting-leave", func() { p.services.Notices.Notices().SetError(p.SaveEdits()) })
}

func (a *Panel) selected() string { id, _, _ := a.services.Desktop.Selected(); return id }

func (a *Panel) firstCategory() string {
	cats := a.services.Book.Store().Work().Categories
	if len(cats) > 0 {
		return cats[0].Name
	}
	return ""
}

func (a *Panel) categoryNames() []string {
	var out []string
	for _, c := range a.services.Book.Store().Work().Categories {
		out = append(out, c.Name)
	}
	return out
}

// Reset drops routes and drafts when the work is closed.
func (a *Panel) Reset() { a.Doc = Form{}; a.renamed = renamed{} }

// Open shows an entry, or the pending list for an unsettled entry.
func (a *Panel) Open(e workspace.Entry) {
	a.services.Pane.RevealPane("setting")
	if e.Place == workspace.PlaceOpen {
		a.Doc.Route.JumpToWaiting()
	} else {
		a.Doc.Route.JumpToEntry(e.ID)
	}
}

// Jump opens an entry directly.
func (a *Panel) Jump(id string) { a.services.Pane.Show("setting"); a.Doc.Route.JumpToEntry(id) }

// Edit opens the entry's form.
func (a *Panel) Edit(e workspace.Entry) {
	a.services.Pane.RevealPane("setting")
	a.Doc.Route.JumpToEntry(e.ID)
	a.editSetting(e)
}

// Waiting opens the pending list.
func (a *Panel) Waiting() { a.services.Pane.Show("setting"); a.Doc.Route.JumpToWaiting() }

// Answer opens a question's answer form.
func (a *Panel) Answer(e workspace.Entry) {
	a.Open(e)
	a.Doc.Route.Answer(e.ID)
	a.Doc.Text, a.Doc.Category, a.Doc.Name, a.Doc.Names = "", e.Category, "", ""
	if len(e.Names) > 0 {
		a.Doc.Name, a.Doc.Names = e.Names[0], strings.Join(e.Names[1:], "、")
	}
}

// CloseWaiting closes pending entries when AI is turned off.
func (a *Panel) CloseWaiting() { a.Doc.Route.CloseWaiting() }

// RenameCount reports remaining prose matches of the old name.
func (a *Panel) RenameCount() int { return a.renamed.count }

// Reveal shows an entry over the current route.
func (a *Panel) Reveal(id string) {
	a.services.Pane.RevealPane("setting")
	a.Doc.Route.OpenEntry(id)
}

// CacheBuilds reports derived-value rebuilds for frame performance diagnostics.
func (a *Panel) CacheBuilds() int { return a.Doc.ideas.builds }
