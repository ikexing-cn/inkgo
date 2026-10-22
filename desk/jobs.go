package desk

import (
	"sync"

	"github.com/egoist/mygo/ui"
)

// jobs are the work done off the view thread: each hands back a function,
// and the view thread runs it in the next frame. Handing back only takes
// the lock, so a worker never waits for the view, even while the view
// waits for the worker.
type jobs struct {
	mu    sync.Mutex
	queue []func()
}

// start runs fn off the view thread; what it returns runs on the view
// thread in the next frame.
func (j *jobs) start(sv ui.Services, fn func() func()) {
	go func() { j.post(sv, fn()) }()
}

// post hands fn to the view thread and asks for a frame.
func (j *jobs) post(sv ui.Services, fn func()) {
	j.mu.Lock()
	j.queue = append(j.queue, fn)
	j.mu.Unlock()
	sv.Invalidate()
}

// pending says whether a function waits for the view thread.
func (j *jobs) pending() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.queue) > 0
}

// poll runs every function waiting, in the order they were handed back.
// One handed back while they run waits for the next frame.
func (j *jobs) poll() {
	j.mu.Lock()
	queue := j.queue
	j.queue = nil
	j.mu.Unlock()
	for _, fn := range queue {
		fn()
	}
}
