package talk

import (
	"context"
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"maps"
	"net/url"
	"runtime"
	"slices"
	"time"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/context/contextapi"
	"writer/ext/hostapi"
	"writer/ext/talk/talkapi"
	"writer/locales"
	"writer/workspace"
)

// Panel owns the discussion's frame-thread view and conversation state.
// Its fields are exposed for native UI inspection; consumers use talkapi.Service.
type Panel struct {
	// Store is the book for this activation.
	Store *workspace.Store
	// Session is the conversation currently shown.
	Session string
	// Sessions lists the stored conversations.
	Sessions []string
	// Titles caches language-independent titles.
	Titles map[string]workspace.ConversationTitle
	// Blank identifies unused conversations.
	Blank map[string]bool
	// Events is the conversation currently read by the view.
	Events []workspace.Event
	// Input is the unsent composer text.
	Input string
	// Source is the read-only extraction editor.
	Source *editor.Editor
	// SourceMessage identifies the event being extracted.
	SourceMessage string
	// SourceOpen controls the extraction dialog.
	SourceOpen bool
	// SourceCategory selects the extraction category.
	SourceCategory string
	// ContextReply identifies the expanded request audit.
	ContextReply string
	// Turn owns the current request mailbox.
	Turn *Turn
	// Quotes holds prose for the next message.
	Quotes []workspace.Quote
	// FocusTalk requests focus in the composer.
	FocusTalk bool
	// Chat tracks the virtual message list.
	Chat ui.ListState
	// Rows stores event indices for the virtual list.
	Rows []int
	// Thoughts tracks expanded reasoning and digests.
	Thoughts map[string]*bool
	// Shelved records hidden conversation rows.
	Shelved map[string]bool
	// ShowShelved includes hidden conversations in the list.
	ShowShelved bool
	// Talks selects the conversation list in the pane.
	Talks bool
	// TalkQuery filters the conversation list.
	TalkQuery string
	// Attach includes the open prose with the next message.
	Attach bool
	// LastChat is the last measured page conversation area.
	LastChat         ui.Rect
	bookHost         hostapi.Book
	desktop          hostapi.Desktop
	pane             hostapi.Pane
	paletteHost      hostapi.Palette
	noticeHost       hostapi.Notices
	jobs             hostapi.Jobs
	request          hostapi.SettingsHost
	suggestions      hostapi.Suggestions
	materialRef      *ext.Ref[contextapi.Context]
	aiRef            *ext.Ref[aiapi.Backend]
	tools            *ext.PointReader[aiapi.Tool]
	search           hostapi.Search
	renderer         *Markdown
	cancel           context.CancelFunc
	selectionChanged func(workspace.Selection)
	// Build creates the per-turn engine on the frame thread.
	Build func(*workspace.Store, aiapi.Material, []aiapi.Tool, *talkapi.Beside, func(talkapi.Progress), func(workspace.Event)) (talkapi.Talk, error)
}

func (a *Panel) content() *book.Book    { return a.bookHost.Content() }
func (a *Panel) palette() *kit.Palette  { return a.paletteHost.Colors() }
func (a *Panel) backend() aiapi.Backend { return a.aiRef.Get() }
func (a *Panel) notices() *kit.Status   { return a.noticeHost.Notices() }
func (a *Panel) selected() string       { id, _, _ := a.desktop.Selected(); return id }
func (a *Panel) lost() bool             { _, lost, _ := a.desktop.Selected(); return lost }
func (a *Panel) editor() *editor.Editor { _, ed, _ := a.bookHost.Chapter(); return ed }
func (a *Panel) held() bool             { return a.request.Held() }
func (a *Panel) focused() bool          { return a.desktop.Focused() }
func (a *Panel) talkTab() bool          { return a.pane.SelectedTab() == "talk" }
func (a *Panel) savePrefs()             { a.warn(a.desktop.SavePreferences()) }
func (a *Panel) warn(err error) {
	if err != nil {
		a.SetError(err)
	}
}

// SetError reports a discussion error on the frame thread.
func (a *Panel) SetError(err error)         { a.notices().SetError(err) }
func (a *Panel) entries() []workspace.Entry { e, _ := a.materialRef.Get().Entries(); return e }
func (a *Panel) hasEntries() bool           { _, ok := a.materialRef.Get().Entries(); return ok }
func (a *Panel) material() aiapi.Material {
	c := a.materialRef.Get()
	_, setting := c.Entries()
	return aiapi.Material{Setting: setting, Outline: c.HasOutlines()}
}
func (a *Panel) chapterOutline(id string) string { s, _ := a.materialRef.Get().Outline(id); return s }
func (a *Panel) nodeTitle(id string) string {
	n, ok := a.content().Tree().Node(id)
	if ok {
		return n.Title
	}
	return ""
}
func (a *Panel) findInChapter(id, text string)  { a.search.FindInChapter(id, text) }
func (a *Panel) entryNavigationAvailable() bool { return a.desktop.EntryAvailable() }
func (a *Panel) showEntry(e workspace.Entry)    { a.desktop.ShowEntry(e) }
func (a *Panel) post(sv ui.Services, fn func()) { a.jobs.Post(sv, fn) }
func (a *Panel) run(c *ui.Context, label string, fn func(context.Context) (func(), error)) {
	turn := a.Turn
	sv := c.Services()
	a.cancel = a.jobs.CancelRequest
	a.jobs.Request(sv, label, func(parent context.Context) (func(), error) {
		ctx, cancel := context.WithCancel(parent)

		defer cancel()
		apply, err := fn(ctx)
		return func() {
			if a.Turn != turn {
				return
			}
			a.UpdateSuggestions()
			if apply != nil {
				apply()
			}
			a.suggestions.Finish(turn.Session)
			turn.Active = false
			a.warn(a.Refresh())
		}, err
	})
}
func (a *Panel) tick(*ui.Context) {
	a.UpdateSuggestions()
	if !a.request.Busy() {
		a.Turn.Active = false
	}
}
func (a *Panel) stop() {
	if a.Turn.Active {
		if a.cancel != nil {
			a.cancel()
		} else {
			a.jobs.CancelRequest()
		}
		a.UpdateSuggestions()
		a.suggestions.Drop()
		a.Turn.Active = false
	}
	a.SourceOpen = false
}
func (a *Panel) loadState(m map[string]json.RawMessage) {
	var ids []string
	_ = json.Unmarshal(m["shelved_sessions"], &ids)
	a.Shelved = map[string]bool{}
	for _, id := range ids {
		a.Shelved[id] = true
	}
	var no bool
	_ = json.Unmarshal(m["no_attach"], &no)
	a.Attach = !no
}
func (a *Panel) saveState() map[string]json.RawMessage {
	ids, _ := json.Marshal(slices.Sorted(maps.Keys(a.Shelved)))
	no, _ := json.Marshal(!a.Attach)
	return map[string]json.RawMessage{"shelved_sessions": ids, "no_attach": no}
}
func keyHint(k string) string {
	if runtime.GOOS == "darwin" {
		return "⌘" + k
	}
	return "Ctrl+" + k
}
func sendKeys() string {
	if runtime.GOOS == "darwin" {
		return "⌘↩"
	}
	return "Ctrl+Enter"
}
func talkDay(id string, now time.Time) string {
	t, err := time.ParseInLocation("20060102-1504", id[:min(len(id), 13)], time.Local)
	if err != nil {
		return ""
	}
	return kit.DayOf(t, now)
}
func isLocal(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	h := u.Hostname()
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}
func (a *Panel) destination(sel workspace.Selection) string {
	for _, p := range a.backend().Providers() {
		if p.ID == sel.Provider {
			u, err := url.Parse(p.BaseURL)
			if err != nil {
				return locales.SettingsInvalidAddress()
			}
			kind := locales.SettingsRemote()
			if isLocal(p.BaseURL) {
				kind = locales.SettingsLocal()
			}
			return kind + " " + u.Host + " / " + sel.Model
		}
	}
	return locales.ChatNeedModel()
}

func (a *Panel) reset() {
	if a.cancel != nil {
		a.cancel()
	}
	a.Turn = &Turn{}
	a.Input = ""
	a.Quotes = nil
	a.Source = nil
	a.SourceOpen = false
	a.ContextReply = ""
	a.Talks = false
	a.Chat = ui.ListState{}
}

// ProgressVisible reports whether the current reply is shown on the frame thread.
func (a *Panel) ProgressVisible() bool {
	return a.Turn.Active && a.Session == a.Turn.Session && a.TalkShown()
}
