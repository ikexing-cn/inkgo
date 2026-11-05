package check

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
	// Cancel holds the active request cancellation function.
	Cancel context.CancelFunc
	// Done holds the result mailbox for this activation.
	Done chan checkDone
}

// checkDone carries a result with the work it belongs to.
type checkDone struct {
	// Chapter holds the checked chapter identity.
	Chapter string
	// Store holds the request store identity.
	Store *workspace.Store
	// Check holds the completed check result.
	Check workspace.Check
	// Err holds the request error.
	Err error
}

// start checks copied inputs in the background and wakes the view on completion.
func (c *checkRunner) start(pen chapterChecker, store *workspace.Store, chapter string, services ui.Services) {
	if c.Done == nil {
		c.Done = make(chan checkDone, 1)
	}
	ctx, cancel := context.WithCancel(context.Background()) // the request has its own time limit
	c.Cancel = cancel
	done := c.Done
	go func() {
		check, err := pen.Check(ctx, chapter)
		if errors.Is(ctx.Err(), context.Canceled) {
			err = context.Canceled // stopped, not failed
		}
		cancel()
		done <- checkDone{Chapter: chapter, Store: store, Check: check, Err: err}
		services.Invalidate()
	}()
}

// stop cancels the running check; its result still returns through the mailbox.
func (c *checkRunner) stop() {
	if c.Cancel != nil {
		c.Cancel()
	}
}

// finished takes a completed result if one is ready, without waiting.
func (c *checkRunner) finished() (checkDone, bool) {
	select {
	case done := <-c.Done:
		c.Cancel = nil
		return done, true
	default:
		return checkDone{}, false
	}
}
