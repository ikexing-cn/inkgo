// Package continuation owns ghost text and the writing model selection.
package continuation

import (
	"context"
	"github.com/egoist/mygo/ui"
	"sync/atomic"
	"writer/editor"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/context/contextapi"
	"writer/ext/continue/continuetext"
	"writer/ext/hostapi"
	"writer/locales"
	"writer/workspace"
)

// Module registers continuation and its settings.
var Module = ext.Module{ID: "continue", Kind: ext.Feature, Group: hostapi.GroupAI, Requires: []string{"ai", "context"}, Name: continuetext.ModContinue, Desc: continuetext.ModContinueAbout, Setup: setup}

type View struct {
	backend  aiapi.Backend
	ai       *ext.Ref[aiapi.Backend]
	book     *ext.Ref[hostapi.Book]
	editing  *ext.Ref[hostapi.Editing]
	notices  *ext.Ref[hostapi.Notices]
	palette  *ext.Ref[hostapi.Palette]
	material *ext.Ref[contextapi.Context]
	Writer   atomic.Pointer[aiapi.Pen]
	key      penKey
	Valid    bool
	Err      error
	auto     *ext.Pref[bool]
	choices  ModelChoices
}

// ModelChoices holds the editable writing model selection.
type ModelChoices struct{ PenProvider, PenModel, PenEffort string }
type penKey struct {
	writing, discussion workspace.Selection
	providers           int
	material            aiapi.Material
}
type pen struct {
	Writer  *atomic.Pointer[aiapi.Pen]
	chapter string
}

func (p pen) Predict(ctx context.Context, before string) (string, error) {
	if w := p.Writer.Load(); w != nil {
		return (*w).Predict(ctx, aiapi.Scene{Chapter: p.chapter, Before: before})
	}
	return "", nil
}

// New returns the module and native frame-thread view.
func New() (ext.Module, *View) {
	s := &View{}
	m := Module
	m.Setup = func(r *ext.Registrar) { setupView(r, s) }
	return m, s
}
func setup(r *ext.Registrar) { setupView(r, &View{}) }
func setupView(r *ext.Registrar, s *View) {
	*s = View{ai: ext.Use(r, aiapi.Service), book: ext.Use(r, hostapi.BookService), editing: ext.Use(r, hostapi.EditingService), notices: ext.Use(r, hostapi.NoticesService), palette: ext.Use(r, hostapi.PaletteService), material: ext.Use(r, contextapi.Service)}
	ext.Contribute(r, aiapi.ModelChanges, "models", s.Invalidate)
	s.auto = ext.Setting(r, "automatic", ext.Schema[bool]{Title: continuetext.SettingsAutoContinue, Scope: ext.BookScope})
	r.OnStart(func(*ext.Session) error {
		s.Valid = false
		if st := s.book.Get().Store(); st != nil {
			w := st.Work().Writing
			s.choices = ModelChoices{w.Provider, w.Model, w.Effort}
		}
		return nil
	})
	r.OnStop(func(*ext.Session) { s.Writer.Store(nil); s.Valid = false; s.Err = nil })
	ext.Contribute(r, hostapi.FrameTicks, "pen", hostapi.FrameTick{Tick: func(*ui.Context) { s.keepPen() }})
	ext.Contribute(r, hostapi.Predictions, "ghost", hostapi.Prediction{Predictor: func(id string) editor.Predictor { return pen{&s.Writer, id} }, Note: func() string {
		if w := s.Writer.Load(); w != nil {
			return (*w).Name()
		}
		return ""
	}, OnDemand: func() bool { return !s.auto.Get() }})
	can := func() bool { _, ed, ok := s.editing.Get().Chapter(); return ok && ed != nil && !ed.ReadOnly() }
	run := func() {
		s.keepPen()
		_, ed, ok := s.editing.Get().Chapter()
		if !ok || ed == nil {
			return
		}
		if s.Writer.Load() == nil {
			s.notices.Get().Notices().Warn(s.Err)
			return
		}
		ed.RequestPrediction()
		ed.Focus()
	}
	ext.Contribute(r, hostapi.Commands, "continue", hostapi.Command{ID: "continue", Menu: hostapi.MenuPosition{Menu: hostapi.MenuEdit, Order: 50}, Name: continuetext.CmdContinue, Key: ui.KeyJ, Mods: ui.Cmd, Can: can, Run: run, HelpGroup: locales.HelpGroupWrite, HelpText: continuetext.HelpContinue, HelpBefore: "accept-drop"})
	ext.Contribute(r, hostapi.SelectionMenus, "menu", hostapi.SelectionMenu{Order: -30, Build: func(string) []hostapi.SelectionAction {
		return []hostapi.SelectionAction{{Label: continuetext.CmdContinue(), Command: "continue", Disabled: !can(), Run: run}}
	}})
	ext.Contribute(r, hostapi.SettingsSections, "model", hostapi.SettingsSection{Work: true, Placement: "ai.models", Draw: s.modelSettings})
	ext.Contribute(r, hostapi.SettingsSections, "writing", hostapi.SettingsSection{Work: true, Placement: "ai.writing", Draw: func(c *ui.Context) {
		pal := s.palette.Get().Colors()
		pal.Subhead(c, continuetext.SettingsWriting(), "")
		v := s.auto.Get()
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			if ui.Checkbox(c, &v, continuetext.SettingsAutoContinue()).Disabled(s.book.Get().Held()).Changed() {
				s.auto.Set(v)
			}
			pal.TickNote(c, continuetext.SettingsAutoNote())
		})
	}})
}
func (s *View) keepPen() {
	st := s.book.Get().Store()
	if st == nil {
		return
	}
	w := st.Work()
	m := s.material.Get()
	_, setting := m.Entries()
	key := penKey{w.Writing, w.Discussion, len(s.currentAI().Providers()), aiapi.Material{Setting: setting, Outline: m.HasOutlines()}}
	if s.Valid && key == s.key {
		return
	}
	s.key = key
	s.Valid = true
	p, err := s.currentAI().Pen(st, key.material)
	s.Err = err
	if err != nil {
		s.Writer.Store(nil)
	} else {
		s.Writer.Store(&p)
	}
}

// Choices exposes the editable writing model selection on the frame thread.
func (s *View) Choices() *ModelChoices { return &s.choices }

// Invalidate rebuilds the selected pen next frame.
func (s *View) Invalidate() { s.Valid = false }

// Predict requests continuation on a worker.
func (s *View) Predict(ctx context.Context, chapter, before string) (string, error) {
	return (pen{&s.Writer, chapter}).Predict(ctx, before)
}

// SetBackend overrides the pen backend in integration fixtures on the frame thread.
func (s *View) SetBackend(b aiapi.Backend) { s.backend = b }
func (s *View) currentAI() aiapi.Backend {
	if s.backend != nil {
		return s.backend
	}
	return s.ai.Get()
}
