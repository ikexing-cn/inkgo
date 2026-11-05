// Package usage owns AI usage records and their settings page.
package usage

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"sync"
	"sync/atomic"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/hostapi"
	"writer/ext/usage/usagetext"
	"writer/workspace"
)

// Module registers usage accounting and its presentation.
var Module = ext.Module{ID: "usage", Kind: ext.Feature, Group: hostapi.GroupAI, Requires: []string{"ai"}, Name: usagetext.ModUsage, Desc: usagetext.ModUsageAbout, Setup: setup}

// New returns the module and its frame-thread view.
func New() (ext.Module, *View) {
	a := &View{}
	m := Module
	m.Setup = func(r *ext.Registrar) { setupView(r, a) }
	return m, a
}
func setup(r *ext.Registrar) { setupView(r, &View{}) }
func setupView(r *ext.Registrar, a *View) {
	book := ext.Use(r, hostapi.BookService)
	editing := ext.Use(r, hostapi.EditingService)
	palette := ext.Use(r, hostapi.PaletteService)
	backend := ext.Use(r, aiapi.Service)
	notices := ext.Use(r, hostapi.NoticesService)
	var current atomic.Pointer[logRun]
	r.OnStart(func(s *ext.Session) error {
		current.Store(&logRun{session: s, store: book.Get().Store()})
		a.host = book.Get()
		a.editing = editing.Get()
		a.store = a.host.Store()
		a.book = a.host.Content()
		a.ai = backend.Get()
		a.pal = palette.Get().Colors()
		a.Record.reset()
		a.revision.Add(1)
		return nil
	})
	r.OnStop(func(*ext.Session) { current.Store(nil); a.Record.reset() })
	ext.Contribute(r, aiapi.Calls, "record", func(call aiapi.Call) {
		run := current.Load()
		if run == nil || !run.session.Active() || (call.Activation != nil && call.Activation != run.session) || call.Store == nil || call.Store != run.store {
			return
		}
		run.mu.Lock()
		err := call.Store.Log(call.Record)
		if err == nil {
			run.failed = false
		} else if !run.failed {
			run.errors = append(run.errors, err)
			run.failed = true
		}
		run.mu.Unlock()
		a.revision.Add(1)
	})
	ext.Contribute(r, hostapi.FrameTicks, "log-errors", hostapi.FrameTick{Tick: func(*ui.Context) {
		run := current.Load()
		if run == nil || !run.session.Active() {
			return
		}
		run.mu.Lock()
		errors := run.errors
		run.errors = nil
		run.mu.Unlock()
		for _, err := range errors {
			notices.Get().Notices().Warn(fmt.Errorf("%s", usagetext.UsageLogFailed(err.Error())))
		}
	}})
	ext.Contribute(r, aiapi.Usage, "usage", func(c *ui.Context) {
		if a.host.Store() == nil {
			return
		}
		if revision := a.revision.Load(); revision != a.loaded {
			records, err := a.store.Records()
			notices.Get().Notices().Warn(err)
			a.Record.load(records)
			a.loaded = revision
		}
		a.pal = palette.Get().Colors()
		a.spent(c)
	})
	ext.Contribute(r, hostapi.SettingsDetails, "records", hostapi.SettingsDetail{
		Enter:   func(string) { a.revision.Add(1) },
		Matches: func(tab string) bool { return tab == "AI" && a.Record.Showing && a.store != nil },
		Title:   usagetext.SettingsRecordsTitle, Note: usagetext.SettingsRecordsNote,
		Draw: a.recordsView, Close: func() { a.Record.Showing = false },
	})
}

type logRun struct {
	session *ext.Session
	store   *workspace.Store
	mu      sync.Mutex
	failed  bool
	errors  []error
}
