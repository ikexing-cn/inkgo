package desk

import (
	"context"
	"errors"

	"github.com/egoist/mygo/ui"
	"writer/workspace"
)

// chapterChecker reads a chapter and checks it against the setting.
type chapterChecker interface {
	Check(context.Context, string) (workspace.Check, error)
}

// checkRunner owns cancellation and the one-result mailbox across work switches.
type checkRunner struct {
	cancel context.CancelFunc
	done   chan checkDone
}

// checkDone carries a result with the work it belongs to.
type checkDone struct {
	chapter string
	store   *workspace.Store
	check   workspace.Check
	err     error
}

// start checks copied inputs in the background and wakes the view on completion.
func (c *checkRunner) start(pen chapterChecker, store *workspace.Store, chapter string, services ui.Services) {
	if c.done == nil {
		c.done = make(chan checkDone, 1)
	}
	ctx, cancel := context.WithCancel(context.Background()) // the request has its own time limit
	c.cancel = cancel
	done := c.done
	go func() {
		check, err := pen.Check(ctx, chapter)
		if errors.Is(ctx.Err(), context.Canceled) {
			err = context.Canceled // stopped, not failed
		}
		cancel()
		done <- checkDone{chapter: chapter, store: store, check: check, err: err}
		services.Invalidate()
	}()
}

// stop cancels the running check; its result still returns through the mailbox.
func (c *checkRunner) stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

// finished takes a completed result if one is ready, without waiting.
func (c *checkRunner) finished() (checkDone, bool) {
	select {
	case done := <-c.done:
		c.cancel = nil
		return done, true
	default:
		return checkDone{}, false
	}
}
