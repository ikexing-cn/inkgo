// Package hooks owns foreshadowing and its writing interface.
package hooks

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	"writer/ext/context/contextapi"
	"writer/ext/hooks/hookstext"
	"writer/ext/hostapi"
	"writer/ext/marks/marksapi"
	"writer/locales"
	"writer/workspace"
)

// Services contains the frame-thread capabilities used by foreshadowing.
type Services struct {
	Book    hostapi.Book
	Palette hostapi.Palette
	Notices hostapi.Notices
	Desktop hostapi.Desktop
	Pane    hostapi.Pane
	Prose   hostapi.ProseUI
}

// Panel owns one book's foreshadowing routes and drafts.
type Panel struct {
	services Services
	State    State
	fores    cached[foreshadowKey, []workspace.Foreshadow]
	late     *ext.Pref[int]
}

// Module registers the foreshadowing extension.
var Module = defaultModule()

func defaultModule() ext.Module {
	m, _ := New()
	m.Setup = func(r *ext.Registrar) { instance, _ := New(); instance.Setup(r) }
	return m
}

// New creates an isolated foreshadowing extension and its controller.
func New() (ext.Module, *Panel) {
	p := &Panel{}
	return ext.Module{ID: "hooks", Kind: ext.Feature, Group: hostapi.GroupLore, Requires: []string{"context", "marks"}, Name: hookstext.ModHooks, Desc: hookstext.ModHooksAbout, Setup: func(r *ext.Registrar) {
		book := ext.Use(r, hostapi.BookService)
		palette := ext.Use(r, hostapi.PaletteService)
		notices := ext.Use(r, hostapi.NoticesService)
		desktop := ext.Use(r, hostapi.DesktopService)
		pane := ext.Use(r, hostapi.PaneService)
		prose := ext.Use(r, hostapi.ProseUIService)
		p.late = ext.Setting(r, "late-after", ext.Schema[int]{Title: hookstext.SettingsLateAfter, Default: 30, Scope: ext.BookScope})
		r.OnStart(func(*ext.Session) error {
			p.services = Services{book.Get(), palette.Get(), notices.Get(), desktop.Get(), pane.Get(), prose.Get()}
			p.State = State{}
			p.fores = cached[foreshadowKey, []workspace.Foreshadow]{}
			return nil
		})
		r.OnStop(func(*ext.Session) { p.State = State{} })
		ext.Contribute(r, contextapi.Hooks, "references", contextapi.HookProvider(func(struct{}) []workspace.Foreshadow { return p.foreshadows() }))
		ext.Contribute(r, hostapi.PaneTabs, "foreshadowing", hostapi.PaneTab{Key: "fore", Title: hookstext.PaneTabHooks, Draw: p.forePanel, Count: p.waiting})
		ext.Contribute(r, hostapi.ToolHints, "chapter-hints", hostapi.DrawItem{Draw: p.toolHints})
		ext.Contribute(r, hostapi.ProseOverlays, "popovers", hostapi.DrawItem{Draw: p.forePopover})
		ext.Contribute(r, hostapi.SelectionMenus, "hooks", hostapi.SelectionMenu{Group: "hooks", Build: p.menu})
		ext.Contribute(r, marksapi.MarginSources, "prose", marksapi.MarginSource{Key: func() any { return p.foreshadowKey() }, Notes: p.marginNotes})
		ext.Contribute(r, hostapi.PendingGroups, "hooks-pending", hostapi.PendingSource{Key: func() string { return fmt.Sprint(p.foreshadowKey(), p.late.Get()) }, Group: p.pending})
		ext.Contribute(r, hostapi.SettingsPages, "reminders", hostapi.SettingsPage{Key: "reminders", Title: locales.SettingsTabRemind, Label: "设置-提醒", Order: 20})
		ext.Contribute(r, hostapi.SettingsSections, "reminders-section", hostapi.SettingsSection{Work: true, Draw: p.remindersView})
		ext.Contribute(r, hostapi.WritingKeys, "selection", hostapi.WritingKey{HideSelection: func() bool { return p.State.Noting || p.State.Paying }, Escape: func() bool {
			f := &p.State
			open := f.Noting || f.Paying
			f.Noting, f.Paying = false, false
			return open
		}, Close: func() { p.State.Noting, p.State.Paying = false, false }})
	}}, p
}
func (a *Panel) book() *book.Book  { return a.services.Book.Content() }
func (a *Panel) pal() *kit.Palette { return a.services.Palette.Colors() }
func (a *Panel) node() string      { id, _, _ := a.services.Book.Chapter(); return id }
func (a *Panel) lost() bool        { _, lost, _ := a.services.Desktop.Selected(); return lost }

type currentChapter struct{ ed *editor.Editor }

func (a *Panel) current() currentChapter {
	_, ed, _ := a.services.Book.Chapter()
	return currentChapter{ed}
}
func (a *Panel) warn(err error) { a.services.Notices.Notices().Warn(err) }
func (a *Panel) waiting() int {
	n := 0
	for _, f := range a.foreshadows() {
		if f.Status() == workspace.ForeLost || f.Late(a.late.Get()) {
			n++
		}
	}
	return n
}
func (a *Panel) menu(string) []hostapi.SelectionAction {
	ed := a.current().ed
	disabled := ed == nil
	if ed != nil {
		s, e := ed.Selection()
		disabled = s == e || ed.ReadOnly()
	}
	out := []hostapi.SelectionAction{{Label: locales.EditorMarkForeshadow(), Disabled: disabled, Run: a.openBury}, {Label: hookstext.EditorMarkPay(), Disabled: disabled, Run: func() { a.openPay("") }}}
	if ed != nil {
		if id := a.foreAt(); id != "" {
			out = append(out, hostapi.SelectionAction{Label: hookstext.EditorSeeMark(), Run: func() { a.showFore(id) }})
		}
	}
	return out
}
func (a *Panel) marginNotes(chapter, text string) []marksapi.MarginItem {
	var out []marksapi.MarginItem
	for _, f := range a.foreshadows() {
		if f.Status() != workspace.ForeOpen && f.Status() != workspace.ForeDone {
			continue
		}
		open := func() { a.showFore(f.ID) }
		if f.Node == chapter && f.End > f.Start {
			kind := hookstext.NotesKindHook()
			if f.Status() == workspace.ForeDone {
				kind = hookstext.NotesKindHookPaid()
			}
			out = append(out, marksapi.MarginItem{ID: "fore-" + f.ID, Kind: kind, Title: foreName(f), Start: f.Start, End: f.End, Look: editor.MarkForeshadow, Open: open})
		}
		for _, p := range f.Payoffs {
			if p.Node == chapter && p.End > p.Start {
				out = append(out, marksapi.MarginItem{ID: "pay-" + p.ID, Kind: hookstext.NotesKindPayoff(), Title: foreName(f), Text: hookstext.NotesPlanted(f.Title), Start: p.Start, End: p.End, Look: editor.MarkPayoff, Open: open})
			}
		}
	}
	return out
}
func (a *Panel) pending() (hostapi.PendingGroup, bool) {
	group := hostapi.PendingGroup{ID: "marks", Name: hookstext.StatusGroupMarks(), Color: a.pal().Faint}
	for _, f := range a.foreshadows() {
		it := hostapi.PendingItem{Here: f.Here, Sub: f.Title}
		switch {
		case f.Status() == workspace.ForeLost:
			it.Key = "伏笔/失去锚点/" + f.Node + "/" + f.Quote
			it.Text = hookstext.StatusQuoteGone(kit.Clip(foreName(f), 16))
		case f.Late(a.late.Get()):
			it.Key = "伏笔/没回收/" + f.Node + "/" + f.Quote
			it.Text = hookstext.StatusUnpaid(kit.Clip(foreName(f), 16), f.Gap)
			it.Sub = hookstext.StatusPlantedIn(it.Sub)
		default:
			continue
		}
		if f.Here {
			it.Sub += locales.StatusHereTag()
		}
		it.Open = func(ui.Services) {
			a.showMark(f.Node, f.ID)
			if a.services.Prose.PaneVisible("margin") || a.services.Prose.PaneVisible("fore") || a.services.Prose.PaneVisible("setting") || a.services.Prose.PaneVisible("talk") {
				a.showFore(f.ID)
			}
		}
		group.Items = append(group.Items, it)
	}
	return group, len(group.Items) > 0
}

// OpenBury starts a foreshadowing draft for the current selection.
func (a *Panel) OpenBury() { a.openBury() }

// OpenPay starts a payoff draft, optionally selecting a foreshadowing.
func (a *Panel) OpenPay(id string) { a.openPay(id) }

// Pay records the payoff draft.
func (a *Panel) Pay(id string) { a.pay(id) }

// Renote changes a foreshadowing's note.
func (a *Panel) Renote(f workspace.Foreshadow, note string) { a.renoteFore(f, note) }

// Drop changes whether a foreshadowing has been given up.
func (a *Panel) Drop(f workspace.Foreshadow, dropped bool) { a.dropFore(f, dropped) }

// Rehang anchors an orphaned foreshadowing to the current selection.
func (a *Panel) Rehang(f workspace.Foreshadow) { a.rehang(f) }

// Show selects a foreshadowing in its pane.
func (a *Panel) Show(id string) { a.showFore(id) }

// SetLateAfter saves the book-scoped reminder threshold.
func (a *Panel) SetLateAfter(n int) { a.late.Set(n) }

// CacheBuilds reports reference-cache rebuilds for diagnostics.
func (a *Panel) CacheBuilds() int { return a.fores.builds }

// Waiting returns the count of orphaned or overdue foreshadowing.
func (a *Panel) Waiting() int { return a.waiting() }

// Bury records the current foreshadowing draft.
func (a *Panel) Bury() { a.bury() }
