package desk

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
	"writer/desk/book"
	"writer/editor"
	"writer/internal/textclip"
	"writer/shelf"
	"writer/workspace"
)

// live is what the reply in flight has streamed so far, the events it
// wrote looking things up, and the changes it is proposing in the prose as
// their text comes. The worker writes it as output arrives; the view reads
// it each frame.
type live struct {
	mu              sync.Mutex
	reasoning, text strings.Builder
	steps           []workspace.Event
	compacting      bool // a digest of the talk is asked for
	proposed        []streamed
}

// streamed is a change proposed in the prose by the call that proposes
// it, as far as its text came.
type streamed struct {
	call string
	p    workspace.Proposal
}

func (l *live) add(p aiport.Progress) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if p.Proposal != nil {
		if i := slices.IndexFunc(l.proposed, func(s streamed) bool { return s.call == p.Call }); i >= 0 {
			l.proposed[i].p = *p.Proposal
		} else {
			l.proposed = append(l.proposed, streamed{p.Call, *p.Proposal})
		}
		return
	}
	l.reasoning.WriteString(p.Reasoning)
	l.text.WriteString(p.Text)
}

// proposals are the changes the reply in flight proposed as far as they
// came, in the order they began.
func (l *live) proposals() []streamed {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.proposed)
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
	l.steps, l.compacting, l.proposed = nil, false, nil
}

// flashFor is how long an entry a talk just noted stays tinted.
const flashFor = 3 * time.Second

type App struct {
	Store *workspace.Store
	// book is the work's content as the view reads it; nil while no work is
	// open. It is used on the view thread only.
	book *book.Book
	// ai is all the AI does: what sends goes through it alone.
	ai    aiport.Service
	aiOff []func()
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

	page              page
	pages             [pageCategories + 1]pageView
	paneTabs          tabTable[paneTab]
	prefTabs          tabTable[string]
	offFrom           page              // the page of the AI the prose took the place of as the AI was turned off,
	offAway           bool              // while there is one
	shown             page              // the page the last frame showed
	renamed           renamed           // a name the prose still has after the author renamed it
	leftWriting       bool              // the work was last on the writing page, not the discussion
	toastShown        map[string]string // what each toast shows, by its id
	toastsOpen        bool              // the folded toasts opened
	status            status            // the error, the failures, and what the toasts say
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
	ck                checks
	session           string
	sessions          []string
	titles            map[string]string
	blank             map[string]bool // talks nothing was said in yet: not listed, and a new talk takes one
	events            []workspace.Event
	seen              int // revisions of the setting the view has flashed
	flash             map[string]time.Time
	input             string
	source            *editor.Editor
	sourceMessage     string
	sourceOpen        bool
	sourceCategory    string
	contextReply      string
	busy              string
	talking           bool              // busy with a discussion turn, shown inline in its talk
	talkSession       string            // the talk of that turn
	talkChapter       string            // the chapter that talk keeps to, when it was opened to draft or rewrite it
	pending           string            // the author's words while their turn is in flight,
	pendingQuotes     []workspace.Quote // and the prose they quote
	// Prose the author quoted for their next turn, and whether the talk
	// beside the prose should take the keyboard.
	quotes                           []workspace.Quote
	focusTalk                        bool
	cancel                           context.CancelFunc
	jobs                             jobs
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
	categoryShow                     bool // the form was opened on a category, to come into view
	removeCategory                   string
	talkEffort                       string
	penProvider, penModel            string
	penEffort                        string
	live                             live
	md                               markdownState
	thoughts                         map[string]*bool // reasoning shown, by event or revision ID
	doc                              docView
	exporting                        exportView
	phone                            phoneState
	write                            writeView
	plan                             planView
	LastChat, LastSetting, LastWrite ui.Rect
	LastProse                        ui.Rect
	// Appearance applies the appearance chosen: "light", "dark", or empty
	// to follow the system. cmd/inkgo hands it to the window.
	Appearance func(string)
}

// New is the app with ai, whose configuration file says where this
// machine keeps its settings: the shelf of works lies beside it, with the
// switch that turns the AI off, and with none there is no shelf either, as
// for a test.
func New(ai aiport.Service) *App {
	a := &App{ai: ai, genre: "网文",
		light: lightPalette(), dark: darkPalette(), titles: map[string]string{}, thoughts: map[string]*bool{}, flash: map[string]time.Time{}}
	a.exporting.host = a
	a.pal = a.light
	a.initPages()
	a.initTabs()
	a.aiOff = []func(){
		func() {
			if a.cancel != nil {
				a.cancel()
			}
		},
		func() {
			if a.Store != nil {
				a.dropProps(a.write.props, workspace.FateOff)
			}
		},
		func() {
			if a.Store != nil {
				a.stopChecks()
			}
		},
		func() { a.write.draftOpen, a.write.rewriting = false, false },
		a.doc.route.closeWaiting,
		a.status.clear,
	}
	if ai.ConfigFile() != "" {
		a.loadShelf()
	}
	ai.BindSwitch(a.aiOn)
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
		a.leaveWork()
		a.Store.Close()
	}
	a.Store, a.path = s, path
	a.book = book.Open(s)
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
	a.plan = planView{}
	a.exporting = exportView{host: a}
	a.page, a.shown, a.offAway = pageTalk, pageTalk, false
}

// closeWork returns to the welcome page, with its form cleared so a new work
// does not inherit the last one's folder or name.
func (a *App) closeWork() {
	a.leaveWork()
	a.SetError(a.Store.Close())
	a.Store = nil
	a.book = nil
	a.path, a.name = "", ""
	a.resetView()
	a.lookForBooks()
}

func (a *App) Close() error {
	if a.cancel != nil {
		a.cancel()
	}
	a.leaveWork()
	// Window shutdown exits the process. A cancelled worker may still be
	// saving what it already has, so leave the store open for it.
	if a.Store != nil && a.busy == "" {
		return a.Store.Close()
	}
	return nil
}

// leaveWork leaves the store open for each caller to close; Close keeps it for a busy worker.
func (a *App) leaveWork() {
	if a.Store == nil {
		return
	}
	a.dropProps(a.write.props, workspace.FateClosed)
	a.flush()
	a.finishPlanOutline()
	a.keepUnsettled()
	a.remember(false)
	a.backupNow()
	a.stopChecks()
}

func (a *App) refresh() error {
	if a.Store == nil {
		return nil
	}
	if err := a.book.ReloadSetting(); err != nil {
		return err
	}
	return a.refreshFromSetting()
}

// refreshFromSetting does what refresh does after the setting has been
// read: the flash of a talk's change, and the talks' titles.
func (a *App) refreshFromSetting() error {
	if a.Store == nil {
		return nil
	}
	defer a.exporting.replan()
	var err error
	a.flashNew()
	if a.events, err = a.Store.Events(a.session); err != nil {
		return err
	}
	if a.sessions, err = a.Store.Sessions(); err != nil {
		return err
	}
	a.titles[a.session], a.blank[a.session] = workspace.TalkTitle(a.events, a.nodeTitle), len(a.events) == 0
	if a.talkSession != a.session {
		delete(a.titles, a.talkSession) // the last turn may have gone on there since: read it again
	}
	for _, s := range a.sessions {
		if _, ok := a.titles[s]; !ok {
			events, err := a.Store.Events(s)
			if err != nil {
				return err
			}
			a.titles[s], a.blank[s] = workspace.TalkTitle(events, a.nodeTitle), len(events) == 0
		}
	}
	return nil
}

// nodeTitle is the title of the chapter of id, as the book has its tree;
// "" when the book has none.
func (a *App) nodeTitle(id string) string {
	if id == "" {
		return ""
	}
	if n, ok := a.book.Tree().Node(id); ok {
		return n.Title
	}
	return ""
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
	s, _, _ = strings.Cut(s, "\n")
	return textclip.Clip(s, n)
}

// talk is the discussion of the open work with the model it selects,
// streaming the reply into the view with what it looks up on the way, and
// what the author is writing beside it. What it notes in the setting
// shows at once.
func (a *App) talk(c *ui.Context) (aiport.Talk, error) {
	a.live.reset()
	a.write.seen = map[string]bool{}
	sv := c.Services()
	return a.ai.Talk(a.Store, a.beside(), func(p aiport.Progress) {
		a.live.add(p)
		sv.Invalidate()
	}, func(e workspace.Event) { a.stepped(sv, e) })
}

// stepped hears an event the reply in flight wrote on its way, off the
// interface's thread: what it looked up shows as it goes, and what it
// noted in the setting at once.
func (a *App) stepped(sv ui.Services, e workspace.Event) {
	a.live.step(e)
	if e.Revision != "" || e.Role == workspace.RoleDigest && e.ID != "" { // the setting it noted, or the talk compacted
		a.jobs.post(sv, func() { a.SetError(a.refresh()) })
	}
	sv.Invalidate()
}

// flashNew tints the entries a talk noted since the view last looked; the
// revisions there were when the work opened are not news.
func (a *App) flashNew() {
	if a.seen >= 0 {
		for _, e := range a.book.Setting().Entries {
			if e.Rev > a.seen && a.book.Setting().Revisions[e.Rev-1].By == workspace.ByTalk {
				a.flash[e.ID] = time.Now()
			}
		}
	}
	a.seen = len(a.book.Setting().Revisions)
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
	a.SetError(nil)
	a.status.retry(func() { a.work(sv, label, fn) })
	a.jobs.start(sv, func() func() {
		apply, err := fn(ctx)
		cancel()
		return func() { a.finished(apply, err) }
	})
}

// finished is what a request ended with, run on the view thread: what it
// applied, whether it failed, and the view after it.
func (a *App) finished(apply func(), err error) {
	a.followProps() // the reply ended: all it proposed is in
	a.busy = ""
	a.cancel = nil
	if apply != nil {
		apply()
	}
	a.unfinishedProps()
	if !a.aiOn() {
		err = nil // what runs is the AI's: turned off, it was stopped, and says nothing
	}
	if err != nil {
		a.status.fail(err)
	} else {
		a.status.succeed()
	}
	if err := a.refresh(); err != nil {
		a.SetError(err)
	}
}

func (a *App) poll() {
	a.followProps() // what the reply in flight proposes, in the prose
	a.jobs.poll()
}

// held tells whether the work in flight holds what it could be changed
// under: the list of models holds the window. A talk turn holds only
// sending another and switching the work; the rest goes on as it replies,
// and a model changed meanwhile takes the next turn.
func (a *App) held() bool { return a.busy != "" && !a.talking }

// replying tells whether a turn is in flight in the talk shown.
func (a *App) replying() bool { return a.talking && a.session == a.talkSession }

// SetError shows err in a toast until the author closes it; nil clears it.
func (a *App) SetError(err error) { a.status.setError(err) }

// errorText is the error shown, as text; "" when there is none.
func (a *App) errorText() string { return a.status.errorText() }

func (a *App) loadWork() {
	w := a.Store.Work()
	a.talkProvider = w.Discussion.Provider
	a.talkModel = w.Discussion.Model
	a.talkEffort = w.Discussion.Effort
	a.penProvider, a.penModel, a.penEffort = w.Writing.Provider, w.Writing.Model, w.Writing.Effort
}
