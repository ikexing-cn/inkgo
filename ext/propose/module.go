// Package propose owns AI edit suggestions and Rewriting on the prose.
package propose

import (
	"context"
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"sync"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/hostapi"
	"writer/ext/marks/marksapi"
	"writer/ext/propose/proposetext"
	"writer/workspace"
)

// View keeps proposal state and placement caches on the frame thread.
type View struct {
	starterRevision uint64
	starterCache    []ext.Item[aiapi.TalkStarter]

	completedMu sync.Mutex
	completed   []completedProposal
	activation  *ext.Session
	of          *workspace.Store

	aiHost *ext.Ref[hostapi.AI]

	book     *ext.Ref[hostapi.Book]
	surface  *ext.Ref[hostapi.ProseUI]
	desktop  *ext.Ref[hostapi.Desktop]
	palette  *ext.Ref[hostapi.Palette]
	notices  *ext.Ref[hostapi.Notices]
	starters *ext.PointReader[aiapi.TalkStarter]
	// Frame is the latest copied talk snapshot on the frame thread.
	Frame aiapi.ProposalFrame
	// Props holds undecided proposals, retained while the extension is disabled.
	Props   []*Proposal
	propVer int
	seen    map[string]bool
	placed  placedKey
	// Rewriting reports whether the rewrite request bar is open.
	Rewriting bool
	// Instruction is the author's rewrite request.
	Instruction string
	// Leaving resumes the pending navigation after the author discards proposals.
	Leaving func()
}

// Module registers edit suggestions.
var Module = func() ext.Module {
	m := module(&View{})
	m.Setup = func(r *ext.Registrar) { module(&View{}).Setup(r) }
	return m
}()

// New returns an isolated edit suggestion controller.
func New() (ext.Module, *View) { v := &View{}; return module(v), v }
func module(v *View) ext.Module {
	return ext.Module{ID: "propose", Kind: ext.Feature, Group: hostapi.GroupAI, Requires: []string{"ai"}, Optional: []string{"marks"}, Name: proposetext.ModPropose, Desc: proposetext.ModProposeAbout, Setup: func(r *ext.Registrar) {
		v.aiHost = ext.Use(r, hostapi.AIService)
		v.book = ext.Use(r, hostapi.BookService)
		v.surface = ext.Use(r, hostapi.ProseUIService)
		v.desktop = ext.Use(r, hostapi.DesktopService)
		v.palette = ext.Use(r, hostapi.PaletteService)
		v.notices = ext.Use(r, hostapi.NoticesService)
		v.starters = ext.ReadPoint(r, aiapi.TalkStarters)
		ext.Contribute(r, aiapi.Proposals, "proposals", aiapi.Suggestions(v))
		ext.Contribute(r, aiapi.Tools, "propose", v.tool())
		ext.Contribute(r, hostapi.SelectionButtons, "rewrite-button", hostapi.SelectionButton{Icon: kit.IcPen, Label: proposetext.EditorAIRewrite, Tip: proposetext.EditorRewriteSel, Available: v.canRewrite, Run: v.OpenRewrite})
		ext.Contribute(r, hostapi.Commands, "rewrite", hostapi.Command{Present: v.talkOn, Name: proposetext.EditorAIRewriteMore, Can: v.canRewrite, Run: v.OpenRewrite})
		ext.Contribute(r, hostapi.SelectionMenus, "rewrite-menu", hostapi.SelectionMenu{Order: -20, Build: func(string) []hostapi.SelectionAction {
			if !v.talkOn() {
				return nil
			}
			return []hostapi.SelectionAction{{Label: proposetext.EditorAIRewriteMore(), Disabled: !v.canRewrite(), Run: v.OpenRewrite}}
		}})
		ext.Contribute(r, hostapi.BeforeEditor, "bar", hostapi.DrawItem{Draw: func(c *ui.Context) {
			if !v.talkOn() {
				v.Rewriting = false
			}
			if v.ed() != nil {
				v.placeProps()
				v.aiBar(c)
			}
		}})
		ext.Contribute(r, hostapi.ProseOverlays, "actions", hostapi.DrawItem{Draw: func(c *ui.Context) {
			if v.ed() != nil {
				v.propActs(c)
			}
		}})
		ext.Contribute(r, hostapi.Overlays, "leave", hostapi.Overlay{Open: func() bool { return v.Leaving != nil }, Draw: v.leaveView})
		ext.Contribute(r, hostapi.LeaveQuestions, "leave-question", v.Leave)
		ext.Contribute(r, hostapi.WritingKeys, "rewrite-keys", hostapi.WritingKey{HideSelection: func() bool { return v.Rewriting }, Close: func() { v.Rewriting = false }})
		r.OnStart(func(s *ext.Session) error {
			v.completedMu.Lock()
			v.activation = s
			v.completed = nil
			v.completedMu.Unlock()
			if st := v.book.Get().Store(); st != v.of {
				v.of = st
				v.Props = nil
				v.seen = nil
				v.propVer++
				v.Instruction = ""
			}
			return nil
		})
		ext.Contribute(r, hostapi.State, "state", hostapi.ViewState{Refresh: func() error { v.pruneProps(); return nil }})
		ext.Contribute(r, marksapi.MarginSources, "ai-written", marksapi.MarginSource{Key: func() any {
			ed := v.ed()
			ver := 0
			if ed != nil {
				ver = ed.Doc.Version()
			}
			return struct {
				ed  *editor.Editor
				ver int
			}{ed, ver}
		}, Notes: func(chapter, text string) []marksapi.MarginItem {
			if v.ed() == nil {
				return nil
			}
			var out []marksapi.MarginItem
			for _, m := range v.ed().Doc.Marks() {
				if m.Kind == editor.MarkAI && !m.Orphaned {
					out = append(out, marksapi.MarginItem{Start: m.Start, End: m.End, Look: editor.MarkAI})
				}
			}
			return out
		}})
		r.OnStop(func(*ext.Session) {
			v.completedMu.Lock()
			v.activation = nil
			v.completed = nil
			v.completedMu.Unlock()
			v.Frame = aiapi.ProposalFrame{}
			if !v.aiHost.Get().AIEnabled() {
				v.dropProps(v.Props, workspace.FateOff)
			}
			if ed := v.ed(); ed != nil {
				ed.SetProposals(nil, propRoom)
			}
			v.Rewriting = false
			v.Leaving = nil
			v.placed = placedKey{}
		})
	}}
}
func (v *View) ed() *editor.Editor { _, ed, _ := v.book.Get().Chapter(); return ed }
func (v *View) node() string       { ID, _, _ := v.book.Get().Chapter(); return ID }
func (v *View) title() string      { s, _ := v.book.Get().ChapterTitle(v.node()); return s }
func (v *View) talkOn() bool       { return len(v.starterItems()) > 0 }
func (v *View) canRewrite() bool {
	ed := v.ed()
	return v.talkOn() && ed != nil && !ed.ReadOnly() && func() bool { s, e := ed.Selection(); return s != e }()
}
func (v *View) warn(err error) { v.notices.Get().Notices().Warn(err) }
func (v *View) beginTalk(c *ui.Context, e workspace.Event) bool {
	ok := false
	for _, i := range v.starterItems() {
		_ = v.starters.Guard(i.Owner, i.ID, func() { ok = i.Value.Start(c, e) })
	}
	return ok
}

type completedProposal struct {
	store                *workspace.Store
	session, call, model string
	p                    workspace.Proposal
}

func (v *View) tool() aiapi.Tool {
	tool := TalkTool()
	run := tool.Run
	tool.Run = func(ctx context.Context, env aiapi.ToolEnv, raw json.RawMessage) aiapi.ToolResult {
		v.completedMu.Lock()
		activation := ext.SessionOf(ctx)
		if activation == nil {
			activation = v.activation
		}
		v.completedMu.Unlock()
		result := run(ctx, env, raw)
		if result.Proposal != nil {
			v.completedMu.Lock()
			if activation != nil && activation.Active() && v.activation == activation && ctx.Err() == nil {
				v.completed = append(v.completed, completedProposal{env.Store, env.Session, env.Call, env.Model, *result.Proposal})
			}
			v.completedMu.Unlock()
		}
		return result
	}
	return tool
}
func (v *View) receiveCompleted() {
	v.completedMu.Lock()
	got := v.completed
	v.completed = nil
	v.completedMu.Unlock()
	for _, p := range got {
		if p.store == v.book.Get().Store() && p.session == v.Frame.Session {
			v.TakeProposal(p.call, p.p, false, p.model)
		}
	}
}

func (v *View) starterItems() []ext.Item[aiapi.TalkStarter] {
	revision := v.starters.Revision()
	if v.starterRevision != revision || v.starterRevision == 0 {
		v.starterCache = v.starters.Items()
		v.starterRevision = revision
	}
	return v.starterCache
}
