package desk

import (
	"context"
	"github.com/egoist/mygo/ui"
	"testing"
	"time"
	"writer/ai"
	"writer/ext"
	"writer/ext/hostapi"
)

func TestFrameCallbackFailuresSettle(t *testing.T) {
	for _, kind := range []string{"post", "job", "request", "worker", "pick", "save"} {
		t.Run(kind, func(t *testing.T) {
			var jobs hostapi.Jobs
			var dialogs hostapi.Dialogs
			stopped := false
			mod := ext.Module{ID: "worker", Setup: func(r *ext.Registrar) {
				ref := ext.Use(r, hostapi.JobsService)
				dr := ext.Use(r, hostapi.DialogsService)
				r.OnStart(func(*ext.Session) error { jobs = ref.Get(); dialogs = dr.Get(); return nil })
				r.OnStop(func(*ext.Session) { stopped = true })
			}}
			a := newTestApp(ai.New(""), mod)
			defer a.Close()
			boom := func() { panic("callback") }
			switch kind {
			case "pick":
				a.PickFolder = func(string) (string, error) { return "picked", nil }
				stillTester(func(c *ui.Context) { dialogs.PickInto(c, "pick", func(string) { boom() }) }, 64, 64)
			case "save":
				a.SaveFile = func(string, string) (string, error) { return "saved", nil }
				stillTester(func(c *ui.Context) { dialogs.SaveInto(c, "save", "name", func(string, error) { boom() }) }, 64, 64)
			case "post":
				jobs.Post(ui.Services{}, boom)
			case "job":
				jobs.StartJob(ui.Services{}, func(context.Context) func() { return boom })
			case "request":
				jobs.Request(ui.Services{}, "request", func(context.Context) (func(), error) { return boom, nil })
			case "worker":
				jobs.Request(ui.Services{}, "request", func(context.Context) (func(), error) { panic("worker") })
			}
			if kind != "worker" {
				awaitJob(t, a)
				a.poll()
			} else {
				for end := time.Now().Add(time.Second); a.extensions.Ledger("worker").State != ext.Failed; {
					if time.Now().After(end) {
						t.Fatal("no failure")
					}
					time.Sleep(time.Millisecond)
				}
			}
			stillTester(func(c *ui.Context) { a.extensionFrame(c); a.poll() }, 64, 64)
			if !stopped || a.busy != "" || a.requestSession != nil || a.extensions.Ledger("worker").State != ext.Failed {
				t.Fatal("failure not settled", stopped, a.busy)
			}
		})
	}
}

func TestRequestCompletionClearsOwnershipBeforeApply(t *testing.T) {
	var jobs hostapi.Jobs
	mod := ext.Module{ID: "worker", Setup: func(r *ext.Registrar) {
		ref := ext.Use(r, hostapi.JobsService)
		r.OnStart(func(*ext.Session) error { jobs = ref.Get(); return nil })
	}}
	a := newTestApp(ai.New(""), mod)
	defer a.Close()
	applied := false
	jobs.Request(ui.Services{}, "request", func(context.Context) (func(), error) {
		return func() {
			if a.busy != "" || a.requestSession != nil {
				t.Error("ownership retained during apply")
			}
			applied = true
		}, nil
	})
	awaitJob(t, a)
	a.poll()
	if !applied || a.busy != "" {
		t.Fatal("completion not applied")
	}
}
