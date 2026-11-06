package desk

import (
	"context"
	"github.com/egoist/mygo/ui"
	"writer/ext"
)

type jobsHost struct{ extensionHost }

func (h jobsHost) BindSession(s *ext.Session) any { return &sessionJobs{a: h.a, s: s} }

type sessionJobs struct {
	a     *App
	s     *ext.Session
	epoch uint64
}

func (j *sessionJobs) Busy() bool { return j.a.busy != "" }
func (j *sessionJobs) Post(sv ui.Services, fn func()) {
	j.a.jobs.post(sv, func() {
		if j.s.Active() {
			j.guard(fn)
		}
	})
}
func (j *sessionJobs) StartJob(sv ui.Services, fn func(context.Context) func()) {
	j.s.Go(func(ctx context.Context) {
		apply := fn(ctx)
		j.Post(sv, func() {
			if apply != nil {
				apply()
			}
		})
	})
}
func (j *sessionJobs) CancelRequest() {
	if j.s.Active() && j.a.workEpoch == j.epoch && j.a.cancel != nil {
		j.a.cancel()
	}
}
func (j *sessionJobs) Request(sv ui.Services, label string, fn func(context.Context) (func(), error)) {
	a := j.a
	if !j.s.Active() || a.busy != "" {
		return
	}
	a.workEpoch++
	epoch := a.workEpoch
	j.epoch = epoch
	a.busy, a.nonblocking = label, true
	a.requestSession = j.s
	a.SetError(nil)
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.status.Retry(func() {
		if j.s.Active() {
			j.Request(sv, label, fn)
		}
	})
	j.s.Go(func(parent context.Context) {
		stop := context.AfterFunc(parent, cancel)
		defer stop()
		defer cancel()
		apply, err := fn(ctx)
		a.jobs.post(sv, func() {
			if a.workEpoch == epoch && j.s.Active() {
				a.finished(func() { j.guard(apply) }, err)
			}
		})
	})
}

func (j *sessionJobs) Run(work func(context.Context)) { j.s.Go(work) }

// Run rejects unbound worker dispatch; extensions obtain activation-bound handles.
func (j jobsHost) Run(work func(context.Context)) { panic("Jobs.Run requires a bound activation") }

func (j *sessionJobs) guard(fn func()) {
	if fn != nil && j.s.Active() {
		_ = ext.Guard(j.a.extensions, j.s.ID(), "Jobs callback", func() {
			if j.s.Active() {
				fn()
			}
		})
	}
}
