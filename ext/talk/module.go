package talk

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/context/contextapi"
	"writer/ext/hostapi"
	"writer/ext/talk/talkapi"
	"writer/ext/talk/talktext"
	"writer/workspace"
)

// Module registers the conversation feature and its worker engine.
var Module = ext.Module{ID: "talk", Kind: ext.Feature, Group: hostapi.GroupAI, Requires: []string{"ai", "context"}, Name: talktext.ModTalk, Desc: talktext.ModTalkAbout, Setup: setup}

type service struct {
	ai         *ext.Ref[aiapi.Backend]
	navigation *ext.Ref[hostapi.Talk]
	markdown   *Markdown
	book       *ext.Ref[hostapi.Book]
	notices    *ext.Ref[hostapi.Notices]
	choices    talkapi.ModelChoices
}

func setup(r *ext.Registrar) {
	s := &service{book: ext.Use(r, hostapi.BookService), notices: ext.Use(r, hostapi.NoticesService), ai: ext.Use(r, aiapi.Service), navigation: ext.Use(r, hostapi.TalkService)}
	palette := ext.Use(r, hostapi.PaletteService)
	material := ext.Use(r, contextapi.Service)
	s.markdown = &Markdown{Palette: func() *kit.Palette { return palette.Get().Colors() }, Entries: func() []workspace.Entry { got, _ := material.Get().Entries(); return got }, ShowEntry: func(entry workspace.Entry) { s.navigation.Get().ShowEntry(entry) }}
	ext.Contribute(r, hostapi.ErrorTexts, "errors", hostapi.ErrorTranslator(translate))
	ext.Contribute(r, hostapi.SettingsSections, "model", hostapi.SettingsSection{Work: true, Placement: "ai.models", Draw: s.modelSettings})
	r.OnStop(func(*ext.Session) { s.navigation.Get().StopTalk() })
	ext.Provide(r, talkapi.Service, func(*ext.Session) talkapi.ServiceAPI { return s })
}

func (s *service) OpenTalk(id string)                 { s.navigation.Get().OpenTalk(id) }
func (s *service) TalkTitle(id string) (string, bool) { return s.navigation.Get().TalkTitle(id) }
func (s *service) StartTalk(c *ui.Context, words workspace.Event) bool {
	return s.navigation.Get().StartTalk(c, words)
}
func (s *service) Build(st *workspace.Store, material aiapi.Material, tools []aiapi.Tool, beside *talkapi.Beside, progress func(talkapi.Progress), step func(workspace.Event)) (talkapi.Talk, error) {
	return Build(s.ai.Get(), st, material, tools, beside, progress, step)
}
func (s *service) Preloaded(st *workspace.Store, m aiapi.Material, origin, chapter string) (talkapi.Preload, error) {
	return Preloaded(st, m, origin, chapter)
}
func (s *service) Extract(st *workspace.Store, session, message string, start, end int, category string) (workspace.Entry, error) {
	return Extract(st, session, message, start, end, category)
}
