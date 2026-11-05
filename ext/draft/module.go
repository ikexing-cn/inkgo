// Package Draft owns outline drafting commands and their dialog.
package draft

import (
	"github.com/egoist/mygo/ui"
	"writer/editor"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/draft/drafttext"
	"writer/ext/hostapi"
	"writer/locales"
	"writer/tokens"
	"writer/workspace"
)

// View keeps the drafting dialog on the frame thread.
type View struct {
	starterRevision uint64
	starterCache    []ext.Item[aiapi.TalkStarter]

	// Preview overrides context preview in UI fixtures; nil uses the active starter.
	Preview func() string

	book     *ext.Ref[hostapi.Book]
	surface  *ext.Ref[hostapi.ProseUI]
	palette  *ext.Ref[hostapi.Palette]
	ai       *ext.Ref[aiapi.Backend]
	starters *ext.PointReader[aiapi.TalkStarter]
	// Open reports whether the drafting dialog is open.
	Open bool
	// Words and Mode are the author's draft request; Note is its frozen context preview.
	Words, Mode, Note string
	// Cards is the chapter card count when the dialog opened.
	Cards int
}

// Module registers outline drafting.
var Module = func() ext.Module {
	m := module(&View{})
	m.Setup = func(r *ext.Registrar) { module(&View{}).Setup(r) }
	return m
}()

// New returns an isolated drafting controller.
func New() (ext.Module, *View) { v := &View{}; return module(v), v }
func module(v *View) ext.Module {
	return ext.Module{ID: "draft", Kind: ext.Feature, Group: hostapi.GroupAI, Requires: []string{"ai", "outline"}, Name: drafttext.ModDraft, Desc: drafttext.ModDraftAbout, Setup: func(r *ext.Registrar) {
		v.book = ext.Use(r, hostapi.BookService)
		v.surface = ext.Use(r, hostapi.ProseUIService)
		v.palette = ext.Use(r, hostapi.PaletteService)
		v.ai = ext.Use(r, aiapi.Service)
		v.starters = ext.ReadPoint(r, aiapi.TalkStarters)
		ext.Contribute(r, hostapi.Commands, "draft", hostapi.Command{ID: "draft", Name: drafttext.CmdDraft, Present: v.talkOn, Hint: func() string {
			if v.ed() != nil && v.chapterOutline(v.node()) == "" {
				return drafttext.CmdDraftNeedsOutline()
			}
			return ""
		}, Can: func() bool {
			_, ed, ok := v.book.Get().Chapter()
			return ok && ed != nil && !ed.ReadOnly() && v.chapterOutline(v.node()) != ""
		}, Run: v.OpenDraft, Menu: hostapi.MenuPosition{Menu: hostapi.MenuEdit, Order: 60}})
		ext.Contribute(r, hostapi.Overlays, "dialog", hostapi.Overlay{Open: func() bool { return v.Open && v.talkOn() }, Draw: func(c *ui.Context) {
			if !v.talkOn() {
				v.Open = false
			}
			if v.Open && v.talkOn() {
				v.draftView(c)
			}
		}})
		r.OnStop(func(*ext.Session) { v.Open = false; v.Words, v.Mode, v.Note, v.Cards = "", "", "", 0 })
	}}
}
func (v *View) node() string       { id, _, _ := v.book.Get().Chapter(); return id }
func (v *View) ed() *editor.Editor { _, ed, _ := v.book.Get().Chapter(); return ed }
func (v *View) talkOn() bool       { return len(v.starterItems()) > 0 }
func (v *View) outlineValue(id string) (string, bool) {
	b := v.book.Get().Content()
	if b == nil {
		return "", false
	}
	return b.Outlines()[id], true
}
func (v *View) chapterOutline(id string) string { s, _ := v.outlineValue(id); return s }
func (v *View) countUnit() string               { return v.book.Get().Store().Work().Language.Unit() }
func (v *View) flush()                          { v.book.Get().Flush() }

// DraftNote describes the fixed chapter context on the frame thread.
func (v *View) DraftNote() string {
	if v.Preview != nil {
		return v.Preview()
	}
	var p aiapi.DraftContext
	var err error
	for _, i := range v.starterItems() {
		_ = v.starters.Guard(i.Owner, i.ID, func() { p, err = i.Value.DraftContext() })
	}
	if err != nil {
		return ""
	}
	what := drafttext.DraftItemBooks()
	if p.VolumeChapters {
		what = drafttext.DraftItemVolume()
	}
	if p.Previous {
		what = drafttext.DraftItemPrev() + locales.DraftListSep() + what
	}
	if p.Cards > 0 {
		what = drafttext.DraftItemCards(p.Cards) + locales.DraftListSep() + what
	}
	return drafttext.DraftSends(what, tokens.Round(p.Tokens))
}
func (v *View) beginTalk(c *ui.Context, e workspace.Event) bool {
	ok := false
	for _, i := range v.starterItems() {
		_ = v.starters.Guard(i.Owner, i.ID, func() { ok = i.Value.Start(c, e) })
	}
	return ok
}

func (v *View) starterItems() []ext.Item[aiapi.TalkStarter] {
	revision := v.starters.Revision()
	if v.starterRevision != revision || v.starterRevision == 0 {
		v.starterCache = v.starters.Items()
		v.starterRevision = revision
	}
	return v.starterCache
}
