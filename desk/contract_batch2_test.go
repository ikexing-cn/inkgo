package desk

import (
	"context"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"slices"
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

func TestCloseSettlesExtensionRequestAndClosesStore(t *testing.T) {
	var jobs hostapi.Jobs
	mod := ext.Module{ID: "worker", Setup: func(r *ext.Registrar) {
		ref := ext.Use(r, hostapi.JobsService)
		r.OnStart(func(*ext.Session) error { jobs = ref.Get(); return nil })
	}}
	a := newTestApp(ai.New(""), mod)
	requireExtension(t, a.Open(extensionBook(t)))
	store := a.Store
	started := make(chan struct{})
	stopped := make(chan struct{})
	jobs.Request(ui.Services{}, "request", func(ctx context.Context) (func(), error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return func() { t.Error("closed request applied") }, nil
	})
	<-started
	requireExtension(t, a.Close())
	select {
	case <-stopped:
	default:
		t.Fatal("worker not joined")
	}
	if a.busy != "" || a.requestSession != nil {
		t.Fatal("request not settled")
	}
	if _, err := store.Chapters(); err == nil {
		t.Fatal("store remains open")
	}
	a.poll()
}

func TestExtensionHelpOrderSurvivesLocaleChange(t *testing.T) {
	a, _, _ := fixture(t)
	tag := "zh-Hans"
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(testLocale) })
	var before []string
	var texts []string
	for _, locale := range []string{"zh-Hans", "en", "zh-Hans"} {
		tag = locale
		a.LocaleChanged()
		a.syncExtensions()
		var ids, labels []string
		for _, row := range a.extViews.help {
			ids = append(ids, row.id)
			labels = append(labels, row.does())
		}
		for _, pair := range [][2]string{{"save", "find.find"}, {"continue", "accept-drop"}} {
			left, right := slices.Index(ids, pair[0]), slices.Index(ids, pair[1])
			if left < 0 || right != left+1 {
				t.Fatalf("%s: %v not adjacent in %v", locale, pair, ids)
			}
		}
		if before != nil && !slices.Equal(before, ids) {
			t.Fatal("locale reordered help", ids)
		}
		if locale == "en" && slices.Equal(texts, labels) {
			t.Fatal("locale did not change help text")
		}
		before, texts = ids, labels
	}
}
