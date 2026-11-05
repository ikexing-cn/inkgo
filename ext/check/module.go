// Package check owns consistency checks and their writing interface.
package check

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/check/checktext"
	"writer/ext/hostapi"
	"writer/ext/lore/loreapi"
	"writer/ext/marks/marksapi"
	"writer/locales"
	"writer/workspace"
	"writer/writing"
)

// Panel owns the cached findings, route and cancellable runner.
type Panel struct {
	// State exposes cached findings and runner state for frame-thread diagnostics.
	State      checks
	bookHost   hostapi.Book
	palette    hostapi.Palette
	notices    hostapi.Notices
	desktop    hostapi.Desktop
	search     hostapi.Search
	backend    aiapi.Backend
	context    loreapi.References
	navigation loreapi.EntryNavigator
	revisions  loreapi.Revisions
}

// Module registers consistency checks with their required prerequisites.
var Module = module(nil)

// New creates an isolated check controller for a host.
func New() (ext.Module, *Panel) { p := &Panel{}; return module(p), p }
func module(controller *Panel) ext.Module {
	return ext.Module{ID: "check", Kind: ext.Feature, Group: hostapi.GroupAI, Requires: []string{"ai", "lore", "marks"}, Name: checktext.ModCheck, Desc: checktext.ModCheckAbout, Setup: func(r *ext.Registrar) {
		p := controller
		if p == nil {
			p = &Panel{}
		}
		b := ext.Use(r, hostapi.BookService)
		pal := ext.Use(r, hostapi.PaletteService)
		n := ext.Use(r, hostapi.NoticesService)
		d := ext.Use(r, hostapi.DesktopService)
		search := ext.Use(r, hostapi.SearchService)
		ai := ext.Use(r, aiapi.Service)
		material := ext.Use(r, loreapi.ReferencesService)
		nav := ext.Use(r, loreapi.EntryNavigationService)
		rev := ext.Use(r, loreapi.RevisionsService)
		r.OnStart(func(*ext.Session) error {
			p.bookHost = b.Get()
			p.palette = pal.Get()
			p.notices = n.Get()
			p.desktop = d.Get()
			p.search = search.Get()
			p.backend = ai.Get()
			p.context = material.Get()
			p.navigation = nav.Get()
			p.revisions = rev.Get()
			if p.State.Of != b.Get().Store() {
				p.State = checks{}
			}
			p.State.Runner = checkRunner{}
			return nil
		})
		r.OnStop(func(*ext.Session) { p.stopChecks() })
		ext.Contribute(r, hostapi.Commands, "open", hostapi.Command{Name: locales.PageCheck, Can: func() bool { return p.store() != nil }, Run: func() { p.openCheck("") }})
		ext.Contribute(r, hostapi.Pages, "check", hostapi.Page{Title: locales.PageCheck, Available: func() bool { return p.store() != nil }, Count: func() int { return len(p.listedChecks()) }, Icon: kit.IcDone, Order: 40, Enter: func() { p.loadChecks(); p.pruneStoredChecks(); p.State.ProseOf = "" }, Draw: p.checkPage})
		ext.Contribute(r, hostapi.FrameTicks, "run", hostapi.FrameTick{Tick: func(c *ui.Context) {
			p.pollChecks(c)
			p.checkWhilePausing()
			if _, ed, _ := p.bookHost.Chapter(); ed != nil {
				ed.SetDoubtColor(p.pal().Amber)
			}
		}})
		ext.Contribute(r, hostapi.BeforeChapterChange, "leave", p.leaveCheck)
		ext.Contribute(r, hostapi.BeforeLeave, "leave-work", p.leaveCheck)
		ext.Contribute(r, hostapi.FooterItems, "status", hostapi.DrawItem{Placement: "prose", Draw: func(c *ui.Context) {
			if _, ed, ok := p.bookHost.Chapter(); ok && ed != nil {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() { p.footCheck(c) })
			}
		}})
		ext.Contribute(r, marksapi.MarginSources, "prose", marksapi.MarginSource{Key: func() any { return p.checkKey() }, Notes: p.checkMarginSource})
		ext.Contribute(r, hostapi.PendingGroups, "findings", hostapi.PendingSource{Key: func() string { return pendingKey(p.checkKey()) }, Group: p.checkPending})
		ext.Contribute(r, hostapi.SettingsPages, "reminders", hostapi.SettingsPage{Key: "reminders", Title: locales.SettingsTabRemind, Label: "设置-提醒", Order: 20})
		ext.Contribute(r, hostapi.SettingsSections, "timing", hostapi.SettingsSection{Work: true, Draw: func(c *ui.Context) {
			p.pal().Subhead(c, checktext.SettingsCheckText(), "")
			ui.Column(c).FillWidth().Gap(2).Children(func() { p.whenRows(c) })
		}})
		ext.Contribute(r, hostapi.State, "view", hostapi.ViewState{Load: p.loadState, Save: p.saveState})
	}}
}
func (a *Panel) book() *book.Book                                 { return a.bookHost.Content() }
func (a *Panel) pal() *kit.Palette                                { return a.palette.Colors() }
func (a *Panel) store() *workspace.Store                          { return a.bookHost.Store() }
func (a *Panel) status() *kit.Status                              { return a.notices.Notices() }
func (a *Panel) ai() aiapi.Backend                                { return a.backend }
func (a *Panel) node() string                                     { id, _, _ := a.desktop.Selected(); return id }
func (a *Panel) warn(err error)                                   { a.status().Warn(err) }
func (a *Panel) checksOn() bool                                   { return a.store() != nil }
func (a *Panel) entries() []workspace.Entry                       { es, _ := a.context.Entries(); return es }
func (a *Panel) countUnit() string                                { return a.store().Work().Language.Unit() }
func (a *Panel) entryNavigationAvailable() bool                   { return a.navigation.Navigation != nil }
func (a *Panel) entryNavigation(fn func(loreapi.EntryNavigation)) { fn(a.navigation.Navigation) }
func (a *Panel) pen() (chapterChecker, error) {
	m, err := a.ai().Model(a.store(), aiapi.RoleWriting)
	if err != nil {
		return nil, err
	}
	_, setting := a.context.Entries()
	return &writing.Writer{Store: a.store(), Model: m, Material: aiapi.Material{Setting: setting, Outline: a.context.HasOutlines()}}, nil
}

type currentChapter struct {
	ed            *editor.Editor
	saved         string
	savedVer      int
	changed, lost bool
}
type writingState struct {
	Node    string
	Chapter currentChapter
}

func (w writingState) cur() *currentChapter { return &w.Chapter }
func (a *Panel) writing() writingState {
	_, ed, _ := a.bookHost.Chapter()
	state := a.bookHost.ChapterState()
	return writingState{a.node(), currentChapter{ed, state.Saved, state.Version, state.Changed, state.Lost}}
}
