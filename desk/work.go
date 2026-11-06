package desk

import (
	"context"
	"github.com/egoist/mygo/ui"
)

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
	a.requestSession = nil
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
	a.requestSession = nil
	a.busy = ""
	a.nonblocking = false
	a.cancel = nil
	if apply != nil {
		apply()
	}
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
	a.settleRequest()
	a.jobs.poll()
}

// held tells whether the work in flight holds what it could be changed
// under: the list of models holds the window. A talk turn holds only
// sending another and switching the work; the rest goes on as it replies,
// and a model changed meanwhile takes the next turn.
func (a *App) held() bool { return a.busy != "" && !a.nonblocking && !a.nonblockingActivity() }

// settleRequest releases only the current extension request, without replacing its failure notice.
func (a *App) settleRequest() {
	if a.requestSession == nil || a.requestSession.Active() {
		return
	}
	a.workEpoch++
	a.requestSession = nil
	if a.cancel != nil {
		a.cancel()
	}
	a.busy, a.nonblocking, a.cancel = "", false, nil
	a.status.Retry(nil)
}
