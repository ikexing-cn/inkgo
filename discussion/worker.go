package discussion

import (
	"context"
	"time"
)

// Worker runs the bookkeeper in the background, one batch at a time: a
// kick after each reply starts it, and the turns that come in while it
// works are taken in together next. A batch failing every attempt is
// recorded as failed and the turns after it go on.
type Worker struct {
	// Book returns the bookkeeper as configured now; an error, as no model
	// being set, leaves the work until the next kick.
	Book func() (*Bookkeeper, error)
	// Backoff is the waits between attempts: a batch has len(Backoff)+1.
	Backoff []time.Duration
	// Changed hears each revision written, on the worker's goroutine.
	Changed func()
	kick    chan struct{}
}

// NewWorker returns a worker; Run starts it.
func NewWorker(book func() (*Bookkeeper, error), backoff []time.Duration, changed func()) *Worker {
	return &Worker{Book: book, Backoff: backoff, Changed: changed, kick: make(chan struct{}, 1)}
}

// Kick asks the worker to look for turns to take in. It never blocks.
func (w *Worker) Kick() {
	select {
	case w.kick <- struct{}{}:
	default:
	}
}

// Run works until ctx is done.
func (w *Worker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.kick:
		}
		for ctx.Err() == nil && w.pass(ctx) {
		}
	}
}

// pass takes in what every session has pending, reporting whether it
// wrote anything, which may have left more to do.
func (w *Worker) pass(ctx context.Context) bool {
	b, err := w.Book()
	if err != nil {
		return false
	}
	sessions, err := b.Pending()
	if err != nil {
		return false
	}
	wrote := false
	for _, s := range sessions {
		if w.session(ctx, b, s) {
			wrote = true
			if w.Changed != nil {
				w.Changed()
			}
		}
		if ctx.Err() != nil {
			return false
		}
	}
	return wrote
}

// session takes in what session has pending, retrying after the backoff,
// and reports whether it wrote a revision.
func (w *Worker) session(ctx context.Context, b *Bookkeeper, s string) bool {
	for attempt := 0; ; attempt++ {
		did, err := b.Run(ctx, s)
		if err == nil {
			return did
		}
		if ctx.Err() != nil {
			return false
		}
		if attempt == len(w.Backoff) {
			return b.Fail(s, err.Error()) == nil
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(w.Backoff[attempt]):
		}
	}
}
