package talk

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"writer/ext/hostapi"
	"writer/ext/talk/talkapi"
	"writer/ext/talk/talktext"

	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext/ai/aiapi"
	"writer/locales"
	"writer/workspace"
)

// Turn is one request's discussion context and streaming mailbox.
// Workers keep this instance even after App starts a different turn.
type Turn struct {
	Active           bool
	jobs             hostapi.Jobs
	Session, Chapter string
	Pending          string
	Quotes           []workspace.Quote
	Live             Live
}

// Live is what the reply in flight has Streamed so far, the events it
// wrote looking things up, and the changes it is proposing in the prose as
// their text comes. The worker writes it as output arrives; the view reads
// it each frame.
type Live struct {
	mu              sync.Mutex
	reasoning, text strings.Builder
	steps           []workspace.Event
	compacting      bool // a digest of the talk is asked for
	proposed        []Streamed
}

// Streamed is a change proposed in the prose by the call that proposes
// it, as far as its text came.
type Streamed struct {
	Call     string
	Proposal workspace.Proposal
}

// Add receives worker progress without retaining frame objects.
func (l *Live) Add(p talkapi.Progress) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if p.Proposal != nil {
		if i := slices.IndexFunc(l.proposed, func(s Streamed) bool { return s.Call == p.Call }); i >= 0 {
			l.proposed[i].Proposal = *p.Proposal
		} else {
			l.proposed = append(l.proposed, Streamed{p.Call, *p.Proposal})
		}
		return
	}
	l.reasoning.WriteString(p.Reasoning)
	l.text.WriteString(p.Text)
}

// proposals are the changes the reply in flight proposed as far as they
// came, in the order they began.
// Proposals returns a copy of the streaming proposals.
func (l *Live) Proposals() []Streamed {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.proposed)
}

// Step receives a completed worker event.
func (l *Live) Step(e workspace.Event) {
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

// Read returns a thread-safe view snapshot.
func (l *Live) Read() (reasoning, text string, steps []workspace.Event, compacting bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.reasoning.String(), l.text.String(), slices.Clone(l.steps), l.compacting
}

// talk is the discussion of the open work with the model it selects,
// streaming the reply into the view with what it looks up on the way, and
// what the author is writing beside it. What it notes in the setting
// shows at once.
// Talk runs on the frame thread.
func (a *Panel) Talk(c *ui.Context) (talkapi.Talk, error) { return a.talk(c.Services()) }
func (a *Panel) talk(sv ui.Services) (talkapi.Talk, error) {
	if a.Build == nil {
		return nil, &aiapi.SetupError{Err: errors.New(locales.ChatNeedModel())}
	}
	turn := &Turn{jobs: a.jobs}
	a.Turn = turn
	a.suggestions.Begin()
	return a.Build(a.Store, a.material(), aiapi.SnapshotReadTools(a.tools), a.Beside(), func(p talkapi.Progress) {
		turn.Live.Add(p)
		sv.Invalidate()
	}, func(e workspace.Event) { a.Stepped(turn, sv, e) })
}

// stepped hears an event the reply in flight wrote on its way, off the
// interface's thread: what it looked up shows as it goes, and what it
// noted in the setting at once.
// Stepped receives worker events on the worker thread and posts refreshes to the frame thread.
func (a *Panel) Stepped(turn *Turn, sv ui.Services, e workspace.Event) {
	turn.Live.Step(e)
	if e.Revision != "" || e.Role == workspace.RoleDigest && e.ID != "" { // the setting it noted, or the talk compacted
		jobs := turn.jobs
		if jobs == nil {
			jobs = a.jobs
		}
		jobs.Post(sv, func() {
			if a.Turn == turn {
				a.SetError(a.editingHost.CategoriesChanged())
			}
		})
	}
	sv.Invalidate()
}

// replying tells whether a turn is in flight in the talk shown.
// Replying runs on the frame thread.
func (a *Panel) Replying() bool { return a.Turn.Active && a.Session == a.Turn.Session }

// Send runs on the frame thread.
func (a *Panel) Send(c *ui.Context) {
	if kit.Trim(a.Input) == "" {
		return
	}
	input, quotes := a.Input, a.Quotes
	words := workspace.Event{Text: input, Quotes: quotes}
	if a.desktop.Writing() {
		words.Chapter = a.selected()
	}
	a.Input, a.Quotes = "", nil
	a.Say(c, a.Session, words, func() {
		if a.Input == "" {
			a.Input, a.Quotes = input, quotes // nothing was saved: give the words back
		}
	})
}

// say sends the author's words to the talk session, showing them there
// until they are saved; unsaid runs when they were not.
// Say runs on the frame thread.
func (a *Panel) Say(c *ui.Context, session string, words workspace.Event, unsaid func()) {
	a.say(c.Services(), session, words, unsaid)
}
func (a *Panel) say(sv ui.Services, session string, words workspace.Event, unsaid func()) {
	talk, err := a.talk(sv)
	if err != nil {
		a.SetError(err)
		if unsaid != nil {
			unsaid()
		}
		return
	}
	words.Role = workspace.RoleUser // as they are kept
	a.Turn.Pending, a.Turn.Quotes = words.Text, words.Quotes
	a.Chat.ScrollToEnd()
	a.runServices(sv, locales.ToastThinking(), func(ctx context.Context) (func(), error) {
		saved, err := talk.Say(ctx, session, words)
		return func() {
			a.Turn.Pending, a.Turn.Quotes = "", nil
			if !saved && unsaid != nil {
				unsaid()
			}
			if a.Session == session { // or the author went on to another talk
				a.Chat.ScrollToEnd()
			}
		}, err
	})
	a.Turn.Active, a.Turn.Session, a.Turn.Chapter = a.jobs.Busy(), session, originChapter(append(slices.Clip(a.Events), words))
}

// Retry runs on the frame thread.
func (a *Panel) Retry(c *ui.Context, userID string) {
	talk, err := a.Talk(c)
	if err != nil {
		a.SetError(err)
		return
	}
	session := a.Session
	a.run(c, talktext.ChatRetrying(), func(ctx context.Context) (func(), error) {
		err := talk.Retry(ctx, session, userID)
		return func() {
			if a.Session == session {
				a.Chat.ScrollToEnd()
			}
		}, err
	})
	a.Turn.Active, a.Turn.Session, a.Turn.Chapter = a.jobs.Busy(), session, originChapter(a.Events)
}
