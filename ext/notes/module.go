// Package notes lays contributed margin notes beside the prose.
package notes

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/ext/marks/marksapi"
	"writer/ext/notes/notestext"
)

// Services supplies the frame-thread book, palette and writing geometry.
type Services struct {
	Book    hostapi.Book
	Editing hostapi.Editing
	Palette hostapi.Palette
	Prose   hostapi.ProseUI
}

// Panel owns margin layout and animations, without producing prose marks.
type Panel struct {
	services Services
	reader   *ext.Ref[marksapi.Reader]
	margin   marginState
}

// Module registers margin notes.
var Module = defaultModule()

func defaultModule() ext.Module {
	m, _ := New()
	m.Setup = func(r *ext.Registrar) { instance, _ := New(); instance.Setup(r) }
	return m
}

// New creates an independent margin layout controller and module.
func New() (ext.Module, *Panel) {
	p := &Panel{}
	return ext.Module{ID: "notes", Kind: ext.Feature, Group: hostapi.GroupWrite, Requires: []string{"marks"}, Name: notestext.ModNotes, Desc: notestext.ModNotesAbout, Setup: func(r *ext.Registrar) {
		book := ext.Use(r, hostapi.BookService)
		bookEditing := ext.Use(r, hostapi.EditingService)
		palette := ext.Use(r, hostapi.PaletteService)
		prose := ext.Use(r, hostapi.ProseUIService)
		p.reader = ext.Use(r, marksapi.Service)
		r.OnStart(func(*ext.Session) error {
			p.services = Services{book.Get(), bookEditing.Get(), palette.Get(), prose.Get()}
			p.margin = marginState{}
			return nil
		})
		r.OnStop(func(*ext.Session) { p.margin = marginState{} })
		ext.Contribute(r, hostapi.PaneTabs, "margin", hostapi.PaneTab{Key: "margin", Before: "talk", Title: notestext.PaneTabNotes, Draw: p.marginPanel})
		ext.Contribute(r, hostapi.FrameTicks, "visibility", hostapi.FrameTick{Tick: func(*ui.Context) {
			if !p.services.Prose.PaneVisible("margin") {
				p.margin.visible = false
			}
		}})
	}}, p
}
func (a *Panel) pal() *kit.Palette      { return a.services.Palette.Colors() }
func (a *Panel) chapter() string        { id, _, _ := a.services.Editing.Chapter(); return id }
func (a *Panel) editor() *editor.Editor { _, ed, _ := a.services.Editing.Chapter(); return ed }

// Layout reports the current note layout for diagnostics.
type Layout struct {
	Boxes    map[string]ui.Rect
	Runs     map[string]uint64
	Lit      string
	Departed int
	Builds   int
}

// Inspect returns the latest frame's layout. Maps are borrowed and read-only.
func (a *Panel) Inspect() Layout {
	return Layout{a.margin.boxes, a.margin.runs, a.margin.lit, len(a.margin.departed), a.margin.notes.builds}
}

// Notes returns the active notes in prose order on the frame thread.
func (a *Panel) Notes() []marksapi.MarginItem {
	var out []marksapi.MarginItem
	for _, n := range a.marginNotes() {
		v := marksapi.MarginItem{Idea: n.idea, ID: n.id, Kind: n.kind, Title: n.title, Text: n.text, Start: n.start, End: n.end, Warn: n.warn, Open: n.open}
		for _, act := range n.actions {
			v.Actions = append(v.Actions, marksapi.NoteAction{Label: act.label, Run: act.run})
		}
		out = append(out, v)
	}
	return out
}
