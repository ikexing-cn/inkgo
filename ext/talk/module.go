package talk

import (
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"os"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/context/contextapi"
	"writer/ext/hostapi"
	"writer/ext/talk/talkapi"
	"writer/ext/talk/talktext"
	"writer/locales"
	"writer/workspace"
)

// Module registers the conversation feature and its worker engine.
var Module = ext.Module{ID: "talk", Kind: ext.Feature, Group: hostapi.GroupAI, Requires: []string{"ai", "context"}, Name: talktext.ModTalk, Desc: talktext.ModTalkAbout, Setup: setup}

type service struct {
	ai       *ext.Ref[aiapi.Backend]
	panel    *Panel
	markdown *Markdown
	book     *ext.Ref[hostapi.Book]
	notices  *ext.Ref[hostapi.Notices]
	choices  talkapi.ModelChoices
}

// New returns a discussion module and its native frame-thread controller.
func New() (ext.Module, *Panel) {
	p := &Panel{}
	m := Module
	m.Setup = func(r *ext.Registrar) { setupPanel(r, p) }
	return m, p
}
func setup(r *ext.Registrar) { setupPanel(r, &Panel{}) }
func setupPanel(r *ext.Registrar, panel *Panel) {
	s := &service{book: ext.Use(r, hostapi.BookService), notices: ext.Use(r, hostapi.NoticesService), ai: ext.Use(r, aiapi.Service)}
	pane := ext.Use(r, hostapi.PaneService)
	desktop := ext.Use(r, hostapi.DesktopService)
	jobs := ext.Use(r, hostapi.JobsService)
	suggestions := &suggestionReader{point: ext.ReadPoint(r, aiapi.Proposals)}
	search := ext.Use(r, hostapi.SearchService)
	material := ext.Use(r, contextapi.Service)
	palette := ext.Use(r, hostapi.PaletteService)
	s.panel = panel
	*s.panel = Panel{Turn: &Turn{}, Titles: map[string]workspace.ConversationTitle{}, Blank: map[string]bool{}, Thoughts: map[string]*bool{}, Shelved: map[string]bool{}, Attach: true, aiRef: s.ai, materialRef: material, tools: ext.ReadPoint(r, aiapi.Tools)}
	r.OnStart(func(*ext.Session) error {
		p := s.panel
		p.bookHost = s.book.Get()
		p.desktop = desktop.Get()
		p.jobs = jobs.Get()

		p.suggestions = suggestions
		p.search = search.Get()
		p.pane = pane.Get()
		p.paletteHost = palette.Get()
		p.noticeHost = s.notices.Get()
		p.renderer = s.markdown
		p.Build = s.Build
		p.selectionChanged = func(sel workspace.Selection) {
			s.choices = talkapi.ModelChoices{TalkProvider: sel.Provider, TalkModel: sel.Model, TalkEffort: sel.Effort}
		}
		st := s.book.Get().Store()
		if st == p.Store {
			return nil
		}
		p.Store = st
		if st != nil {
			w := st.Work()
			s.choices = talkapi.ModelChoices{TalkProvider: w.Discussion.Provider, TalkModel: w.Discussion.Model, TalkEffort: w.Discussion.Effort}
		}
		p.Session = ""
		p.Sessions = nil
		p.Events = nil
		p.Titles = map[string]workspace.ConversationTitle{}
		p.Blank = map[string]bool{}
		p.Thoughts = map[string]*bool{}
		p.Shelved = map[string]bool{}
		p.Turn = &Turn{}
		p.Input = ""
		p.Quotes = nil
		p.Chat = ui.ListState{}
		p.Talks = false
		p.TalkQuery = ""
		p.Attach = true
		if st == nil {
			return nil
		}
		raw, _ := os.ReadFile(st.PrefsPath())
		var state map[string]json.RawMessage
		_ = json.Unmarshal(raw, &state)
		p.loadState(state)
		ids, err := st.Sessions()
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			p.Session, err = st.NewSession()
		} else {
			p.Session = ids[len(ids)-1]
		}
		if err != nil {
			return err
		}
		return p.Refresh()
	})
	ext.Contribute(r, hostapi.FrameTicks, "tick", hostapi.FrameTick{Tick: s.panel.tick})
	ext.Contribute(r, hostapi.Overlays, "source", hostapi.Overlay{Open: func() bool { return s.panel.SourceOpen }, Draw: func(c *ui.Context) {
		if s.panel.SourceOpen && s.panel.Source != nil {
			s.panel.Extraction(c)
		}
	}})
	ext.Contribute(r, hostapi.State, "view", hostapi.ViewState{PaneChanged: func(tab string) {
		if tab != "talk" {
			s.panel.Talks = false
		}
	}, Reset: s.panel.reset, Load: s.panel.loadState, Save: s.panel.saveState, Refresh: s.panel.Refresh})
	ext.Contribute(r, hostapi.Pages, "page", hostapi.Page{Title: locales.PageTalk, Icon: kit.IcChat, Order: 10, Sidebar: s.panel.SideBody, SidebarAction: func(c *ui.Context) {
		if kit.IconButton(c, kit.IcPlus, locales.SideNewTalk(), ui.Color{}).Disabled(s.panel.held()).Clicked() {
			s.panel.NewSession()
		}
	},
		Draw:       func(c *ui.Context) { s.panel.Discussion(c) },
		HasPane:    func() bool { return pane.Get().HasBeside() },
		PaneOpen:   func() bool { return pane.Get().PaneOpen() },
		TogglePane: func() { pane.Get().TogglePane() },
	})
	ext.Contribute(r, hostapi.PaneTabs, "pane", hostapi.PaneTab{Key: "talk", Before: "fore", Title: talktext.PaneTabChat,
		Draw: func(c *ui.Context) { s.panel.TalkPaneBody(c) },
	})
	canQuote := func() bool { _, ed, ok := s.book.Get().Chapter(); return ok && ed != nil && ed.SelectedText() != "" }
	ext.Contribute(r, hostapi.Commands, "send", hostapi.Command{Name: talktext.HelpSend, Key: ui.KeyEnter, Mods: ui.Cmd, Can: func() bool { return false }, HelpGroup: talktext.HelpGroupChat, HelpText: talktext.HelpSend, HelpAfter: locales.HelpZoomText})
	ext.Contribute(r, hostapi.Commands, "quote", hostapi.Command{Name: talktext.EditorQuote, Key: ui.KeyL, Mods: ui.Cmd,
		HelpGroup: locales.HelpGroupWrite, HelpText: talktext.HelpCite, HelpAfter: locales.HelpAcceptDrop,
		Can: canQuote, Run: func() { s.panel.QuoteToTalk() },
	})
	ext.Contribute(r, hostapi.SelectionMenus, "quote-menu", hostapi.SelectionMenu{Order: -10, Build: func(text string) []hostapi.SelectionAction {
		return []hostapi.SelectionAction{{Label: talktext.EditorQuote(), Command: "talk.quote", Disabled: text == "", Run: func() { s.panel.QuoteToTalk() }}}
	}})
	ext.Contribute(r, aiapi.TalkStarters, "starter", aiapi.TalkStarter{Start: s.panel.StartTalk, DraftContext: s.panel.draftContext, Open: func(id string) { s.panel.OpenTalk(id); pane.Get().Show("talk") }, Running: func() bool { return s.panel.Turn.Active }, ProgressVisible: s.panel.ProgressVisible})
	s.markdown = &Markdown{Palette: func() *kit.Palette { return palette.Get().Colors() }, Entries: func() []workspace.Entry { got, _ := material.Get().Entries(); return got }, ShowEntry: func(entry workspace.Entry) { s.panel.showEntry(entry) }}
	ext.Contribute(r, hostapi.ErrorTexts, "errors", hostapi.ErrorTranslator(translate))
	ext.Contribute(r, hostapi.SettingsSections, "model", hostapi.SettingsSection{Work: true, Placement: "ai.models", Draw: s.modelSettings})
	r.OnStop(func(*ext.Session) { s.panel.stop() })
	ext.Provide(r, talkapi.Service, func(*ext.Session) talkapi.ServiceAPI { return s })
}

// OpenTalk implements the frame-thread conversation capability.
func (s *service) OpenTalk(id string) { s.panel.OpenTalk(id) }

// TalkTitle implements the frame-thread conversation capability.
func (s *service) TalkTitle(id string) (string, bool) { return s.panel.TalkName(id) }

// Build implements the frame-thread conversation capability.
func (s *service) Build(st *workspace.Store, material aiapi.Material, tools []aiapi.Tool, beside *talkapi.Beside, progress func(talkapi.Progress), step func(workspace.Event)) (talkapi.Talk, error) {
	return Build(s.ai.Get(), st, material, tools, beside, progress, step)
}

// View exposes the native discussion controller for UI inspection on the frame thread.
func (s *service) View() *Panel { return s.panel }

// OpenTalk opens a saved conversation on the frame thread.
func (p *Panel) OpenTalk(id string) { p.desktop.ShowPage("talk.page"); p.SwitchSession(id) }
