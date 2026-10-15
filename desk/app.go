// Package desk is the native discussion interface.
package desk

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/desk/aiport"
	"writer/editor"
	"writer/shelf"
	"writer/workspace"
)

type result struct {
	apply func()
	err   error
}

// live is what the reply in flight has streamed so far, and the events it
// wrote looking things up. The worker writes it as output arrives; the view
// reads it each frame.
type live struct {
	mu              sync.Mutex
	reasoning, text strings.Builder
	steps           []workspace.Event
	compacting      bool // a digest of the talk is asked for
}

func (l *live) add(reasoning, text string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.reasoning.WriteString(reasoning)
	l.text.WriteString(text)
}

func (l *live) step(e workspace.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.Role == workspace.RoleDigest {
		l.compacting = e.ID == "" // asked for, or written
		return
	}
	l.steps = append(l.steps, e)
	// What a step said before its calls is not the reply: the reply is
	// what streams after the last of them.
	l.text.Reset()
}

func (l *live) read() (reasoning, text string, steps []workspace.Event, compacting bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.reasoning.String(), l.text.String(), l.steps, l.compacting
}

func (l *live) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.reasoning.Reset()
	l.text.Reset()
	l.steps, l.compacting = nil, false
}

// flashFor is how long an entry a talk just noted stays tinted.
const flashFor = 3 * time.Second

type App struct {
	Store *workspace.Store
	// ai is all the AI does: what sends goes through it alone.
	ai    aiport.Service
	Error string
	// PickFolder shows a native folder dialog; it runs off the UI thread.
	// Nil hides the browse buttons (tests, platforms without dialogs).
	PickFolder func(title string) (string, error)
	// Backups asks, the first time a work opens here, where to back it up.
	Backups bool
	// SaveFile asks where to save a file named name, off the UI thread;
	// nil saves exports into the work's 导出 folder.
	SaveFile func(title, name string) (string, error)
	// Reveal shows a folder in the system's file manager; nil hides it.
	Reveal func(path string) error
	// SetTitle names the window, reporting false when there is no window
	// yet to name; nil leaves the title as it is.
	SetTitle func(title string) bool

	light, dark *palette
	pal         *palette

	page              int               // 0 discussion, 1 models, 2 categories, 3 writing, 5 checks
	shown             int               // the page the last frame showed
	notice            string            // what was done
	hint              string            // what to do first, or that there was nothing to do
	renamed           renamed           // a name the prose still has after the author renamed it
	leftWriting       bool              // the work was last on the writing page, not the discussion
	toastShown        map[string]string // what each toast shows, by its id
	toastsOpen        bool              // the folded toasts opened
	again             func()            // runs the last request again
	failed            string            // the error the last request ended with,
	fails             int               // and how many times in a row
	keysOpen          bool              // the shortcuts shown
	sideHidden        bool              // the author put the sidebar away
	bar               barState          // what the title bar keeps from frame to frame
	menu              menuState         // the menu open
	titled            string            // the window title last set
	shelved           map[string]bool   // the conversations put out of the list
	showShelved       bool              // the shelved conversations listed too
	path, name, genre string
	shelf             shelf.Shelf
	bk                backupView
	books             shelfView
	sums              summaries
	ck                checks
	session           string
	sessions          []string
	titles            map[string]string
	blank             map[string]bool // talks nothing was said in yet: not listed, and a new talk takes one
	events            []workspace.Event
	set               workspace.Setting
	seen              int // revisions of set the view has flashed
	flash             map[string]time.Time
	input             string
	source            *editor.Editor
	sourceMessage     string
	sourceOpen        bool
	sourceCategory    string
	contextReply      string
	busy              string
	talking           bool   // busy with a discussion turn, shown inline in its talk
	talkSession       string // the talk of that turn
	pending           string // the author's words while their turn is in flight
	// Prose the author quoted for their next turn, and whether the talk
	// beside the prose should take the keyboard.
	quotes                           []workspace.Quote
	focusTalk                        bool
	cancel                           context.CancelFunc
	results                          chan result
	picks                            chan func()
	chat                             ui.ListState
	rows                             []int // the chat list's: event indexes, -1 the empty chat, -2 the turn in flight
	providerID                       string
	provider                         aiport.Provider
	key                              string
	limits                           []limitRow // the models of the service in the form
	limitKeys                        int
	talkProvider, talkModel          string
	categoryName                     string
	categoryTarget                   string
	removeCategory                   string
	talkEffort                       string
	penProvider, penModel            string
	penEffort                        string
	live                             live
	md                               mdCache
	thoughts                         map[string]*bool // reasoning shown, by event or revision ID
	doc                              docView
	write                            writeView
	changes                          chan struct{} // the talk noted the setting
	LastChat, LastSetting, LastWrite ui.Rect
	// Appearance applies the appearance chosen: "light", "dark", or empty
	// to follow the system. cmd/poc hands it to the window.
	Appearance func(string)
}

// New is the app with ai, whose configuration file says where this
// machine keeps its settings: the shelf of works lies beside it, with the
// switch that turns the AI off, and with none there is no shelf either, as
// for a test.
func New(ai aiport.Service) *App {
	a := &App{ai: ai, genre: "网文", results: make(chan result, 1), picks: make(chan func(), 1), changes: make(chan struct{}, 1),
		light: lightPalette(), dark: darkPalette(), titles: map[string]string{}, md: mdCache{}, thoughts: map[string]*bool{}, flash: map[string]time.Time{}}
	a.pal = a.light
	if ai.ConfigFile() != "" {
		a.loadShelf()
	}
	ai.SetOn(a.aiOn())
	if a.aiOn() {
		a.loadAI()
	}
	return a
}

func (a *App) Open(path string) error {
	s, err := workspace.Open(path)
	if err != nil {
		return fmt.Errorf("打开作品「%s」失败：%w", filepath.Base(path), err)
	}
	return a.attach(s, path, false)
}

// attach makes s the current work, starting a session when it has none.
func (a *App) attach(s *workspace.Store, path string, fresh bool) error {
	if a.Store != nil {
		a.flush()
		a.keepUnsettled()
		a.remember(false)
		a.backupNow()
		a.stopSummaries()
		a.stopChecks()
		a.Store.Close()
	}
	a.Store, a.path = s, path
	a.remember(true)
	a.titles, a.blank = map[string]string{}, map[string]bool{}
	a.resetView()
	a.loadPrefs()
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
	a.askBackup()
	return a.refresh()
}

func (a *App) resetView() {
	a.source, a.sourceOpen, a.contextReply = nil, false, ""
	a.input, a.seen, a.flash, a.doc, a.write = "", -1, map[string]time.Time{}, docView{}, writeView{}
	a.bk = backupView{}
	a.page, a.shown = 0, 0
}

// closeWork returns to the welcome page, with its form cleared so a new work
// does not inherit the last one's folder or name.
func (a *App) closeWork() {
	a.flush()
	a.keepUnsettled()
	a.remember(false)
	a.backupNow()
	a.stopSummaries()
	a.stopChecks()
	a.setError(a.Store.Close())
	a.Store = nil
	a.path, a.name = "", ""
	a.resetView()
	a.lookForBooks()
}

func (a *App) Close() error {
	if a.cancel != nil {
		a.cancel()
	}
	a.flush()
	a.keepUnsettled()
	a.remember(false)
	if a.Store != nil {
		a.backupNow()
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
	if a.set, err = a.Store.Setting(); err != nil {
		return err
	}
	a.flashNew()
	if a.events, err = a.Store.Events(a.session); err != nil {
		return err
	}
	if a.sessions, err = a.Store.Sessions(); err != nil {
		return err
	}
	a.titles[a.session], a.blank[a.session] = sessionTitle(a.events), len(a.events) == 0
	if a.talkSession != a.session {
		delete(a.titles, a.talkSession) // the last turn may have gone on there since: read it again
	}
	for _, s := range a.sessions {
		if _, ok := a.titles[s]; !ok {
			events, err := a.Store.Events(s)
			if err != nil {
				return err
			}
			a.titles[s], a.blank[s] = sessionTitle(events), len(events) == 0
		}
	}
	return nil
}

// sessionTitle names a conversation by its first words.
func sessionTitle(events []workspace.Event) string {
	for _, e := range events {
		if e.Role == workspace.RoleUser {
			return clip(e.Text, 14)
		}
	}
	return "新对话"
}

// talkDay is the day a conversation began, from its ID: 今天, 昨天, 10-06.
func talkDay(id string, now time.Time) string {
	t, err := time.ParseInLocation("20060102-1504", id[:min(len(id), 13)], time.Local)
	if err != nil {
		return ""
	}
	return dayOf(t, now)
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

// talk is the discussion of the open work with the model it selects,
// streaming the reply into the view with what it looks up on the way, and
// what the author is writing beside it. What it notes in the setting
// shows at once.
func (a *App) talk(c *ui.Context) (aiport.Talk, error) {
	a.live.reset()
	sv := c.Services()
	return a.ai.Talk(a.Store, a.beside(), func(reasoning, text string) {
		a.live.add(reasoning, text)
		sv.Invalidate()
	}, func(e workspace.Event) { a.stepped(sv, e) })
}

// stepped hears an event the reply in flight wrote on its way, off the
// interface's thread: what it looked up shows as it goes, and what it
// noted in the setting at once.
func (a *App) stepped(sv ui.Services, e workspace.Event) {
	a.live.step(e)
	if e.Revision != "" || e.Role == workspace.RoleDigest && e.ID != "" { // the setting it noted, or the talk compacted
		select {
		case a.changes <- struct{}{}:
		default:
		}
	}
	sv.Invalidate()
}

// flashNew tints the entries a talk noted since the view last looked; the
// revisions there were when the work opened are not news.
func (a *App) flashNew() {
	if a.seen >= 0 {
		for _, e := range a.set.Entries {
			if e.Rev > a.seen && a.set.Revisions[e.Rev-1].By == workspace.ByTalk {
				a.flash[e.ID] = time.Now()
			}
		}
	}
	a.seen = len(a.set.Revisions)
}

// All state visible to the view is changed on the view thread. Workers own the
// store until their result arrives; navigation/mutations are disabled meanwhile.
// What runs this way is the AI's: the talk and the list of models.
func (a *App) run(c *ui.Context, label string, fn func(context.Context) (func(), error)) {
	a.work(c.Services(), label, fn)
}

// work is run with the window's services, which outlive the frame: the
// worker asks for the next frame through them, and so does trying again.
func (a *App) work(sv ui.Services, label string, fn func(context.Context) (func(), error)) {
	if a.busy != "" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background()) // each request has its own time limit; a turn takes as many as it needs
	a.cancel = cancel
	a.busy = label
	a.Error = ""
	a.again = func() { a.work(sv, label, fn) }
	go func() { apply, err := fn(ctx); cancel(); a.results <- result{apply: apply, err: err}; sv.Invalidate() }()
}

func (a *App) poll() {
	select {
	case fn := <-a.picks:
		fn()
	default:
	}
	select {
	case <-a.changes:
		a.setError(a.refresh())
	default:
	}
	select {
	case r := <-a.results:
		a.busy = ""
		a.cancel = nil
		if r.apply != nil {
			r.apply()
		}
		if !a.aiOn() {
			r.err = nil // what runs is the AI's: turned off, it was stopped, and says nothing
		}
		if r.err != nil {
			a.Error = r.err.Error()
			if a.Error == a.failed {
				a.fails++
			} else {
				a.failed, a.fails = a.Error, 1
			}
		} else {
			a.failed, a.fails = "", 0
		}
		if err := a.refresh(); err != nil {
			a.Error = err.Error()
		}
	default:
	}
}

// held tells whether the work in flight holds what it could be changed
// under: the list of models holds the window. A talk turn holds only
// sending another and switching the work; the rest goes on as it replies,
// and a model changed meanwhile takes the next turn.
func (a *App) held() bool { return a.busy != "" && !a.talking }

// replying tells whether a turn is in flight in the talk shown.
func (a *App) replying() bool { return a.talking && a.session == a.talkSession }

func (a *App) setError(err error) {
	if err != nil {
		a.Error = err.Error()
	} else {
		a.Error = ""
	}
}

func (a *App) loadWork() {
	w := a.Store.Work()
	a.talkProvider = w.Discussion.Provider
	a.talkModel = w.Discussion.Model
	a.talkEffort = w.Discussion.Effort
	a.penProvider, a.penModel, a.penEffort = w.Writing.Provider, w.Writing.Model, w.Writing.Effort
}
