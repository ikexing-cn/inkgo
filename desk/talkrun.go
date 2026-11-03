package desk

import (
	"context"
	"slices"
	"strings"
	"sync"
	"writer/locales"

	"github.com/egoist/mygo/ui"
	"writer/aiport"
	"writer/desk/kit"
	"writer/workspace"
)

// talkTurn is one request's discussion context and streaming mailbox.
// Workers keep this instance even after App starts a different turn.
type talkTurn struct {
	active           bool
	session, chapter string
	pending          string
	quotes           []workspace.Quote
	live             live
}

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
	return l.reasoning.String(), l.text.String(), slices.Clone(l.steps), l.compacting
}

// talk is the discussion of the open work with the model it selects,
// streaming the reply into the view with what it looks up on the way, and
// what the author is writing beside it. What it notes in the setting
// shows at once.
func (a *App) talk(c *ui.Context) (aiport.Talk, error) {
	turn := &talkTurn{}
	a.turn = turn
	a.write.seen = map[string]bool{}
	sv := c.Services()
	return a.ai.Talk(a.Store, a.beside(), func(p aiport.Progress) {
		turn.live.add(p)
		sv.Invalidate()
	}, func(e workspace.Event) { a.stepped(turn, sv, e) })
}

// stepped hears an event the reply in flight wrote on its way, off the
// interface's thread: what it looked up shows as it goes, and what it
// noted in the setting at once.
func (a *App) stepped(turn *talkTurn, sv ui.Services, e workspace.Event) {
	turn.live.step(e)
	if e.Revision != "" || e.Role == workspace.RoleDigest && e.ID != "" { // the setting it noted, or the talk compacted
		a.jobs.post(sv, func() {
			if a.turn == turn {
				a.SetError(a.refresh())
			}
		})
	}
	sv.Invalidate()
}

// Requests run off the view thread and return an apply function to it.
// Discussion keeps other editing available; held describes what must wait.
func (a *App) run(c *ui.Context, label string, fn func(context.Context) (func(), error)) {
	a.work(c.Services(), label, fn)
}

// work is run with the window's services, which outlive the frame: the
// worker asks for the next frame through them, and so does trying again.
func (a *App) work(sv ui.Services, label string, fn func(context.Context) (func(), error)) {
	if a.busy != "" {
		return
	}
	a.workEpoch++
	epoch := a.workEpoch
	ctx, cancel := context.WithCancel(context.Background()) // each request has its own time limit; a turn takes as many as it needs
	a.cancel = cancel
	a.busy = label
	a.SetError(nil)
	a.status.Retry(func() {
		if a.workEpoch == epoch {
			a.work(sv, label, fn)
		}
	})
	a.jobs.start(sv, func() func() {
		apply, err := fn(ctx)
		cancel()
		return func() {
			if a.workEpoch == epoch {
				a.finished(apply, err)
			}
		}
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
		a.status.Fail(err)
	} else {
		a.status.Succeed()
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
func (a *App) held() bool { return a.busy != "" && !a.turn.active }

// replying tells whether a turn is in flight in the talk shown.
func (a *App) replying() bool { return a.turn.active && a.session == a.turn.session }

func (a *App) send(c *ui.Context) {
	if kit.Trim(a.input) == "" {
		return
	}
	input, quotes := a.input, a.quotes
	words := workspace.Event{Text: input, Quotes: quotes}
	if a.page == pageWrite {
		words.Chapter = a.write.node
	}
	a.input, a.quotes = "", nil
	a.say(c, a.session, words, func() {
		if a.input == "" {
			a.input, a.quotes = input, quotes // nothing was saved: give the words back
		}
	})
}

// say sends the author's words to the talk session, showing them there
// until they are saved; unsaid runs when they were not.
func (a *App) say(c *ui.Context, session string, words workspace.Event, unsaid func()) {
	talk, err := a.talk(c)
	if err != nil {
		a.SetError(err)
		if unsaid != nil {
			unsaid()
		}
		return
	}
	words.Role = workspace.RoleUser // as they are kept
	a.turn.pending, a.turn.quotes = words.Text, words.Quotes
	a.chat.ScrollToEnd()
	a.run(c, locales.ToastThinking(), func(ctx context.Context) (func(), error) {
		saved, err := talk.Say(ctx, session, words)
		return func() {
			a.turn.pending, a.turn.quotes = "", nil
			if !saved && unsaid != nil {
				unsaid()
			}
			if a.session == session { // or the author went on to another talk
				a.chat.ScrollToEnd()
			}
		}, err
	})
	a.turn.active, a.turn.session, a.turn.chapter = a.busy != "", session, originChapter(append(slices.Clip(a.events), words))
}

func (a *App) retry(c *ui.Context, userID string) {
	talk, err := a.talk(c)
	if err != nil {
		a.SetError(err)
		return
	}
	session := a.session
	a.run(c, locales.ChatRetrying(), func(ctx context.Context) (func(), error) {
		err := talk.Retry(ctx, session, userID)
		return func() {
			if a.session == session {
				a.chat.ScrollToEnd()
			}
		}, err
	})
	a.turn.active, a.turn.session, a.turn.chapter = a.busy != "", session, originChapter(a.events)
}
