// Package desk is the native M1 discussion interface.
package desk

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/discussion"
	"writer/editor"
	"writer/model"
	"writer/workspace"
)

type result struct {
	apply func()
	err   error
}

// undo remembers the author's last decision so a misclick costs one click.
type undo struct {
	id, text string
	from, to workspace.Status
	at       time.Time
}

// live is what the steps of the running turn have streamed so far. The
// worker writes it as output arrives; the view reads it each frame.
type live struct {
	mu    sync.Mutex
	step  discussion.Step
	parts map[discussion.Step]*streamed
}

type streamed struct{ reasoning, text strings.Builder }

func (l *live) add(step discussion.Step, d model.Delta) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.parts == nil {
		l.parts = map[discussion.Step]*streamed{}
	}
	p := l.parts[step]
	if p == nil || d == (model.Delta{}) {
		// An empty delta announces the step; one that runs again starts empty.
		p = &streamed{}
		l.parts[step] = p
	}
	l.step = step
	p.reasoning.WriteString(d.Reasoning)
	p.text.WriteString(d.Text)
}

// read returns the running step and what step s has streamed.
func (l *live) read(s discussion.Step) (running discussion.Step, reasoning, text string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if p := l.parts[s]; p != nil {
		reasoning, text = p.reasoning.String(), p.text.String()
	}
	return l.step, reasoning, text
}

func (l *live) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.step, l.parts = "", nil
}

// draft holds the edits to the selected card until they are saved.
type draft struct {
	text, category, factSource string
	fact, verified             bool
}

type App struct {
	Store      *workspace.Store
	Config     model.Config
	ConfigFile string
	Error      string
	// PickFolder shows a native folder dialog; it runs off the UI thread.
	// Nil hides the browse buttons (tests, platforms without dialogs).
	PickFolder func(title string) (string, error)

	light, dark *palette
	pal         *palette

	page                    int // 0 discussion, 1 models, 2 categories
	notice                  string
	path, name, genre       string
	welcomeMode             int // 0 create, 1 open
	session                 string
	sessions                []string
	titles                  map[string]string
	events                  []workspace.Event
	ideas                   []workspace.Idea
	input                   string
	filter                  int // index into filters
	extra, search           string
	turn                    string // reply ID restricting the pool, or ""
	selected, expanded      string
	rejectID                string
	draft                   draft
	reason                  string
	source                  *editor.Editor
	sourceMessage           string
	sourceOpen              bool
	sourceCategory          string
	contextReply            string
	busy                    string
	talking                 bool   // busy with a discussion turn, shown inline in the chat
	pending                 string // the author's words while their turn is in flight
	cancel                  context.CancelFunc
	results                 chan result
	picks                   chan func()
	chat                    ui.ListState
	rows                    []int // the chat list's: event indexes, -1 the empty chat, -2 the turn in flight
	focusCard               string
	last                    *undo
	providerID              string
	provider                model.Provider
	key, models             string
	talkProvider, talkModel string
	bookProvider, bookModel string
	budget                  string
	categoryName            string
	categoryTarget          string
	removeCategory          string
	talkEffort, bookEffort  string
	live                    live
	md                      mdCache
	thoughts                map[string]*bool // reasoning shown, by event ID
	LastChat, LastPool      ui.Rect
}

func New() *App {
	path, err := model.ConfigPath()
	if err != nil {
		a := NewWithConfig("")
		a.Error = err.Error()
		return a
	}
	return NewWithConfig(path)
}

// NewWithConfig allows a separate profile, including isolated test profiles.
func NewWithConfig(path string) *App {
	a := &App{genre: "网文", results: make(chan result, 1), picks: make(chan func(), 1), ConfigFile: path,
		light: lightPalette(), dark: darkPalette(), titles: map[string]string{}, md: mdCache{}, thoughts: map[string]*bool{}}
	a.pal = a.light
	if path != "" {
		var err error
		a.Config, err = model.Load(path)
		if err != nil {
			a.Error = err.Error()
		}
	}
	if len(a.Config.Providers) > 0 {
		a.chooseProvider(a.Config.Providers[0].ID)
	}
	return a
}

func (a *App) Open(path string) error {
	s, err := workspace.Open(path)
	if err != nil {
		return err
	}
	return a.attach(s, path, false)
}

func (a *App) create() error {
	name := trim(a.name)
	if name == "" {
		name = filepath.Base(trim(a.path))
	}
	s, err := workspace.Create(trim(a.path), name, a.genre)
	if err != nil {
		return err
	}
	return a.attach(s, trim(a.path), true)
}

// attach makes s the current work, starting a session when it has none.
func (a *App) attach(s *workspace.Store, path string, fresh bool) error {
	if a.Store != nil {
		a.Store.Close()
	}
	a.Store, a.path = s, path
	a.titles = map[string]string{}
	a.resetView()
	a.loadWork()
	sessions, err := s.Sessions()
	if err != nil {
		return err
	}
	if fresh || len(sessions) == 0 {
		a.session, err = s.NewSession()
		if err != nil {
			return err
		}
	} else {
		a.session = sessions[len(sessions)-1]
	}
	return a.refresh()
}

func (a *App) resetView() {
	a.selected, a.expanded, a.rejectID, a.turn = "", "", "", ""
	a.source, a.sourceOpen, a.contextReply = nil, false, ""
	a.input, a.search, a.extra, a.filter, a.last = "", "", extras[0], 0, nil
	a.page = 0
}

// closeWork returns to the welcome page, with its form cleared so a new work
// does not inherit the last one's folder or name.
func (a *App) closeWork() {
	a.setError(a.Store.Close())
	a.Store = nil
	a.path, a.name = "", ""
	a.resetView()
}

func (a *App) Close() error {
	if a.cancel != nil {
		a.cancel()
	}
	// Window shutdown exits the process. A cancelled worker may still be
	// saving what it already has, so leave the store open for it.
	if a.Store != nil && a.busy == "" {
		return a.Store.Close()
	}
	return nil
}
func (a *App) refresh() error {
	if a.Store == nil {
		return nil
	}
	var err error
	if a.ideas, err = a.Store.Ideas(); err != nil {
		return err
	}
	if a.events, err = a.Store.Events(a.session); err != nil {
		return err
	}
	if a.sessions, err = a.Store.Sessions(); err != nil {
		return err
	}
	a.titles[a.session] = sessionTitle(a.session, a.events)
	for _, s := range a.sessions {
		if _, ok := a.titles[s]; !ok {
			events, err := a.Store.Events(s)
			if err != nil {
				return err
			}
			a.titles[s] = sessionTitle(s, events)
		}
	}
	return nil
}

// sessionTitle names a conversation by its first words, after its date.
func sessionTitle(id string, events []workspace.Event) string {
	when := id
	if t, err := time.ParseInLocation("20060102-1504", id[:min(len(id), 13)], time.Local); err == nil {
		when = t.Format("01-02 15:04")
	}
	for _, e := range events {
		if e.Kind == workspace.KindMessage && e.Role == workspace.RoleUser {
			return when + " · " + clip(e.Text, 14)
		}
	}
	return when + " · 新对话"
}

func clip(s string, n int) string {
	r := []rune(s)
	for i, ch := range r {
		if ch == '\n' {
			r = r[:i]
			break
		}
	}
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return string(r)
}

// engine returns a discussion engine for the open work with the models it
// selects, streaming its progress into the view when c is given. Without
// models it still returns an engine and says why: saving a decision or
// replaying a journaled plan needs no model.
func (a *App) engine(c *ui.Context) (*discussion.Engine, error) {
	e := &discussion.Engine{Store: a.Store}
	if c != nil {
		a.live.reset()
		e.Progress = func(s discussion.Step, d model.Delta) {
			a.live.add(s, d)
			c.Invalidate()
		}
	}
	work := a.Store.Work()
	if work.Discussion.Provider == "" || work.Discussion.Model == "" {
		return e, errors.New("还没有选择讨论模型：请到“模型设置”里选一个")
	}
	talk, err := model.NewClient(a.Config, work.Discussion.Provider, work.Discussion.Model)
	if err != nil {
		return e, err
	}
	sel := work.Bookkeeping
	if sel.Provider == "" {
		sel.Provider, sel.Model = work.Discussion.Provider, work.Discussion.Model
	}
	book, err := model.NewClient(a.Config, sel.Provider, sel.Model)
	if err != nil {
		return e, err
	}
	e.Talk = discussion.Model{Completer: talk, Name: work.Discussion.Provider + "/" + work.Discussion.Model, Effort: model.Effort(work.Discussion.Effort)}
	e.Book = discussion.Model{Completer: book, Name: sel.Provider + "/" + sel.Model, Effort: model.Effort(sel.Effort)}
	return e, nil
}

// All state visible to the view is changed on the view thread. Workers own the
// store until their result arrives; navigation/mutations are disabled meanwhile.
func (a *App) run(c *ui.Context, label string, fn func(context.Context) (func(), error)) {
	if a.busy != "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	a.cancel = cancel
	a.busy = label
	a.Error = ""
	go func() { apply, err := fn(ctx); cancel(); a.results <- result{apply: apply, err: err}; c.Invalidate() }()
}

func (a *App) poll() {
	select {
	case fn := <-a.picks:
		fn()
	default:
	}
	select {
	case r := <-a.results:
		a.busy = ""
		a.cancel = nil
		if r.apply != nil {
			r.apply()
		}
		if r.err != nil {
			a.Error = r.err.Error()
		}
		if err := a.refresh(); err != nil {
			a.Error = err.Error()
		}
	default:
	}
}

func (a *App) setError(err error) {
	if err != nil {
		a.Error = err.Error()
	} else {
		a.Error = ""
	}
}

// browse asks for a folder without blocking the view thread.
func (a *App) browse(c *ui.Context, title string, into *string) {
	pick := a.PickFolder
	go func() {
		path, err := pick(title)
		a.picks <- func() {
			if err != nil {
				a.Error = err.Error()
			} else if path != "" {
				*into = path
				if a.name == "" {
					a.name = filepath.Base(path)
				}
			}
		}
		c.Invalidate()
	}()
}

func (a *App) loadWork() {
	w := a.Store.Work()
	a.talkProvider = w.Discussion.Provider
	a.talkModel = w.Discussion.Model
	a.bookProvider = w.Bookkeeping.Provider
	a.bookModel = w.Bookkeeping.Model
	a.talkEffort = w.Discussion.Effort
	a.bookEffort = w.Bookkeeping.Effort
	a.budget = fmt.Sprint(w.TokenBudget)
}
