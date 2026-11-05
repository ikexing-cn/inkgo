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
			fn()
		}
	})
}
func (j *sessionJobs) StartJob(sv ui.Services, fn func() func()) {
	j.s.Go(func(context.Context) {
		apply := fn()
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
	a.SetError(nil)
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	settle := func() {
		if a.workEpoch == epoch {
			a.workEpoch++
			a.busy, a.nonblocking, a.cancel = "", false, nil
			a.status.Retry(nil)
		}
	}
	j.s.OnStop(func() { cancel(); settle() })
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
		j.Post(sv, func() {
			if a.workEpoch == epoch {
				a.finished(apply, err)
			}
		})
	})
}

func (j *sessionJobs) Run(work func(context.Context)) { j.s.Go(work) }

// Run rejects unbound worker dispatch; extensions obtain activation-bound handles.
func (j jobsHost) Run(work func(context.Context)) { panic("Jobs.Run requires a bound activation") }
