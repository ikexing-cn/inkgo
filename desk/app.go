package desk

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"time"
	"writer/ext/hostapi"
	"writer/ext/lore/loreapi"

	"github.com/egoist/mygo/ui"
	"writer/desk/book"
	"writer/desk/kit"
	"writer/editor"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/locales"
	"writer/shelf"
	"writer/workspace"
)

// flashFor is how long an entry a talk just noted stays tinted.
const flashFor = 3 * time.Second

type App struct {
	hostapi.ModelChoices
	// Window callbacks only touch these atomics. View consumes the return
	// signal, even when blur and focus both happened between two frames.
	windowInactive atomic.Bool
	windowReturned atomic.Bool
	fold           foldState
	Store          *workspace.Store
	// book is the work's content as the view reads it; nil while no work is
	// open. It is used on the view thread only.
	book *book.Book
	// ai is all the AI does: what sends goes through it alone.
	configFile string
	aiRef      *ext.OptRef[aiapi.Backend]
	aiWasOn    bool
	aiResolved aiapi.Backend
	ai         aiapi.Backend
	aiOff      []func()
	// PickFolder shows a native folder dialog; it runs off the UI thread.
	// Nil hides the browse buttons (tests, platforms without dialogs).
	PickFolder func(title string) (string, error)
	// SaveFile asks where to save a file named name, off the UI thread;
	// nil saves exports into the work's 导出 folder.
	SaveFile func(title, name string) (string, error)
	// Reveal shows a folder in the system's file manager; nil hides it.
	Reveal func(path string) error
	// SetTitle names the window, reporting false when there is no window
	// yet to name; nil leaves the title as it is.
	SetTitle func(title string) bool
	// MenuChanged gets the system menu as each frame leaves it, on the view
	// thread; nil where there is no system menu (see system_menu.go).
	MenuChanged func([]SystemMenuItem)
	// nativeMenu holds the system menu's clicks until the next frame.
	nativeMenu systemMenuQueue

	light, dark *kit.Palette
	pal         *kit.Palette

	page              page
	pages             []pageView
	pageOrder         []page
	pageIDs           map[string]page
	paneTabs          tabTable[paneTab]
	hostPaneTabs      tabTable[paneTab]
	extViews          extensionViews
	prefTabs          tabTable[string]
	hostPrefTabs      tabTable[string]
	offFrom           page              // the page of the AI the prose took the place of as the AI was turned off,
	offAway           bool              // while there is one
	shown             page              // the page the last frame showed
	leftWriting       bool              // the work was last on the writing page, not the discussion
	toastShown        map[string]string // what each toast shows, by its id
	toastsOpen        bool              // the folded toasts opened
	status            kit.Status        // the error, the failures, and what the toasts say
	keysOpen          bool              // the shortcuts shown
	sideHidden        bool              // the author put the sidebar away
	bar               barState          // what the title bar keeps from frame to frame
	menu              kit.MenuState     // the menu open
	titled            string            // the window title last set
	shelved           map[string]bool   // the conversations put out of the list
	showShelved       bool              // the shelved conversations listed too
	path, name, genre string
	shelf             shelf.Shelf
	ranNoGPU          bool // the shelf's NoGPU as the software opened, which cmd/inkgo draws by
	settings          settingsView
	record            recordView
	books             shelfView
	ck                checks
	session           string
	sessions          []string
	titles            map[string]workspace.ConversationTitle
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
	turn              *talkTurn
	workEpoch         uint64
	busy              string
	// Prose the author quoted for their next turn, and whether the talk
	// beside the prose should take the keyboard.
	quotes                           []workspace.Quote
	focusTalk                        bool
	cancel                           context.CancelFunc
	jobs                             jobs
	chat                             ui.ListState
	rows                             []int // the chat list's: event indexes, -1 the empty chat, -2 the turn in flight
	categoryName                     string
	categoryTarget                   string
	categoryShow                     bool // the form was opened on a category, to come into view
	removeCategory                   string
	md                               kit.MarkdownState
	thoughts                         map[string]*bool // reasoning shown, by event or revision ID
	doc                              docView
	extensions                       *ext.Host
	extStore                         *extensionStore
	extNotices                       extensionNotices
	write                            writeView
	LastChat, LastSetting, LastWrite ui.Rect
	LastProse                        ui.Rect
	// Appearance applies the appearance chosen: "light", "dark", or empty
	// to follow the system. cmd/inkgo hands it to the window.
	Appearance func(string)
	// ZoomFits says whether the current display can hold the window at z.
	// Nil, every step fits. cmd/inkgo supplies the display's cached work area.
	ZoomFits func(float32) bool
	// ZoomChanged asks the host to resize the window outside the frame.
	ZoomChanged func(float32)
	// Relaunch closes the software and opens it again, on the same work.
	// cmd/inkgo hands it mygo's; nil, the settings only say that a change
	// waits for the software to open again.
	Relaunch func()

	// ErrorText contains translators registered by application features.
	ErrorText []ErrorTranslator
	// SetLocale applies a language chosen in settings through the window host.
	SetLocale func(string)
	// localeRevision counts the language changes LocaleChanged heard, from
	// mygo's main thread; caches of words keep the one they were built at.
	localeRevision atomic.Uint64
	// localeShown is the revision the window title was last set at.
	localeShown uint64
}

// New assembles the host with a stable machine configuration location and an
// explicit extension list. The shelf and extension switches live beside it.
func New(configFile string, mods ...ext.Module) *App {
	a := &App{configFile: configFile, turn: &talkTurn{}, genre: "网文",
		light: kit.LightPalette(), dark: kit.DarkPalette(), titles: map[string]workspace.ConversationTitle{}, thoughts: map[string]*bool{}, flash: map[string]time.Time{}}
	a.status.Translate = a.errText
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
		func() { a.entryNavigation(func(n loreapi.EntryNavigation) { n.CloseWaiting() }) },
		a.status.Clear,
	}
	if configFile != "" {
		a.loadShelf()
	}
	a.initExtensions(mods)
	a.ranNoGPU = a.shelf.NoGPU
	return a
}

func (a *App) Open(path string) error {
	s, err := workspace.Open(path)
	if err != nil {
		return openBookError{book: filepath.Base(path), err: err}
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
	a.titles, a.blank = map[string]workspace.ConversationTitle{}, map[string]bool{}
	a.resetView()
	if err := a.extensions.Open(ext.Book{Dir: s.Root()}); err != nil {
		return err
	}
	a.syncExtensions()
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
	return a.refresh()
}

func (a *App) resetView() {
	if a.cancel != nil {
		a.cancel()
	}
	a.workEpoch++
	a.busy, a.cancel, a.turn = "", nil, &talkTurn{}
	a.resetPrefs()
	a.source, a.sourceOpen, a.contextReply = nil, false, ""
	a.input, a.seen, a.flash, a.doc, a.write = "", -1, map[string]time.Time{}, docView{}, writeView{}
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
	// Close ended the book sessions; shelf features run with no book.
	a.warn(a.extensions.Open(ext.Book{}))
	a.syncExtensions()
	a.lookForBooks()
}

func (a *App) Close() error {
	if a.cancel != nil {
		a.cancel()
	}
	a.leaveWork()
	a.resetPrefs()
	a.workEpoch++
	a.turn = &talkTurn{}
	// Window shutdown exits the process. A cancelled worker may still be
	// saving what it already has, so leave the store open for it.
	if a.Store != nil && a.busy == "" {
		return a.Store.Close()
	}
	return nil
}

// leaveWork leaves the store open for each caller to close; Close keeps it for a busy worker.
func (a *App) leaveWork() {
	defer func() {
		if a.extensions != nil {
			a.extensions.Close()
		}
	}()
	if a.Store == nil {
		return
	}
	a.dropProps(a.write.props, workspace.FateClosed)
	a.flush()
	a.keepUnsettled()
	a.remember(false)
	a.extensionLeave()
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
	var err error
	a.flashNew()
	if a.events, err = a.Store.Events(a.session); err != nil {
		return err
	}
	if a.sessions, err = a.Store.Sessions(); err != nil {
		return err
	}
	a.titles[a.session], a.blank[a.session] = a.talkTitle(a.events), len(a.events) == 0
	if a.turn.session != a.session {
		delete(a.titles, a.turn.session) // the last turn may have gone on there since: read it again
	}
	for _, s := range a.sessions {
		if _, ok := a.titles[s]; !ok {
			events, err := a.Store.Events(s)
			if err != nil {
				return err
			}
			a.titles[s], a.blank[s] = a.talkTitle(events), len(events) == 0
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
	return kit.DayOf(t, now)
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

// SetError shows err in a toast until the author closes it; nil clears it.
func (a *App) SetError(err error) { a.status.SetError(err) }

// errorText is the error shown, as text; "" when there is none.
func (a *App) errorText() string { return a.status.ErrorText() }

func (a *App) loadWork() {
	w := a.Store.Work()
	a.TalkProvider = w.Discussion.Provider
	a.TalkModel = w.Discussion.Model
	a.TalkEffort = w.Discussion.Effort
	a.PenProvider, a.PenModel, a.PenEffort = w.Writing.Provider, w.Writing.Model, w.Writing.Effort
}

// openBookError retains the original cause while localizing the application message.
type openBookError struct {
	book string
	err  error
}

func (e openBookError) Error() string { return locales.AppOpenBookFailed(e.book, e.err.Error()) }

func (e openBookError) Unwrap() error { return e.err }
