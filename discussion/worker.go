package discussion

import (
	"context"
	"fmt"
	"sync/atomic"
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
	// Changed hears each revision written, and the worker starting and
	// stopping work, on the worker's goroutine.
	Changed func()
	// Log, when set, hears what the worker does and why it stops, on the
	// worker's goroutine: nothing of it shows in the view.
	Log  func(string)
	kick chan struct{}
	busy atomic.Bool
}

// Busy reports whether the worker is taking turns in, retries and the
// waits before them included.
func (w *Worker) Busy() bool { return w.busy.Load() }

func (w *Worker) setBusy(busy bool) {
	w.busy.Store(busy)
	if w.Changed != nil {
		w.Changed()
	}
}

// attemptLimit bounds one attempt, so a stalled connection cannot hold the
// worker for good.
const attemptLimit = 5 * time.Minute

func (w *Worker) log(format string, v ...any) {
	if w.Log != nil {
		w.Log(fmt.Sprintf(format, v...))
	}
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
		w.log("没有开始整理：%v", err)
		return false
	}
	sessions, err := b.Pending()
	if err != nil {
		w.log("读不出待整理的对话：%v", err)
		return false
	}
	if len(sessions) == 0 {
		return false
	}
	w.setBusy(true)
	defer w.setBusy(false)
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
		w.log("整理会话 %s，第 %d 次", s, attempt+1)
		start := time.Now()
		run, cancel := context.WithTimeout(ctx, attemptLimit)
		did, err := b.Run(run, s)
		cancel()
		if err == nil {
			w.log("完成，用时 %v，写入修订：%v", time.Since(start).Round(time.Second), did)
			return did
		}
		w.log("失败，用时 %v：%v", time.Since(start).Round(time.Second), err)
		if ctx.Err() != nil {
			return false
		}
		if attempt == len(w.Backoff) {
			if err = b.Fail(s, err.Error()); err != nil {
				w.log("记录失败时出错：%v", err)
			}
			return err == nil
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(w.Backoff[attempt]):
		}
	}
}
