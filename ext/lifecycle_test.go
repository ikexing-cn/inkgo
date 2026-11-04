package ext_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"writer/ext"
	"writer/workspace"
)

func TestOptionalDependencyDisappearsAndReturnsWithoutConsumerRestart(t *testing.T) {
	point := ext.PointKey[string]("b.items")
	key := ext.ServiceKey[string]("b")
	var ref *ext.OptRef[string]
	starts, stops := 0, 0
	h := host(t, ext.Options{LocalSwitches: &localStore{off: []string{"a", "b"}}}, ext.Module{ID: "b", Setup: func(r *ext.Registrar) {
		ext.Declare(r, point)
		ext.Provide(r, key, func(*ext.Session) string { return "service" })
	}}, ext.Module{ID: "a", Optional: []string{"b"}, Setup: func(r *ext.Registrar) {
		ref = ext.UseOptional(r, key)
		ext.Contribute(r, point, "first", "1")
		ext.Contribute(r, point, "second", "2")
		r.OnStart(func(*ext.Session) error { starts++; return nil })
		r.OnStop(func(*ext.Session) { stops++ })
	}})
	must(t, h.Set("a", true))
	state(t, h, "a", ext.On)
	state(t, h, "b", ext.Off)
	if _, ok := ref.Get(); ok || len(ext.Items(h, point)) != 0 {
		t.Fatal("inactive optional provider exposed")
	}
	must(t, h.Set("b", true))
	if v, ok := ref.Get(); !ok || v != "service" {
		t.Fatal(v, ok)
	}
	if got := ext.Items(h, point); len(got) != 2 || got[0].ID != "a.first" || got[1].ID != "a.second" {
		t.Fatal(got)
	}
	must(t, h.Set("b", false))
	state(t, h, "a", ext.On)
	if _, ok := ref.Get(); ok || len(ext.Items(h, point)) != 0 {
		t.Fatal("optional effects remain")
	}
	must(t, h.Set("b", true))
	if len(ext.Items(h, point)) != 2 || starts != 1 || stops != 0 {
		t.Fatalf("items %v starts %d stops %d", ext.Items(h, point), starts, stops)
	}
}
func TestDisabledLedgerRetainsEveryDeclarationAndReverseDependency(t *testing.T) {
	key := ext.ServiceKey[int]("b")
	point := ext.PointKey[int]("a.items")
	h := host(t, ext.Options{LocalSwitches: &localStore{off: []string{"a"}}}, ext.Module{ID: "b", Setup: func(r *ext.Registrar) { ext.Provide(r, key, func(*ext.Session) int { return 1 }) }}, ext.Module{ID: "a", Requires: []string{"b"}, Setup: func(r *ext.Registrar) {
		ext.Declare(r, point)
		ext.Contribute(r, point, "item", 1)
		ext.Provide(r, ext.ServiceKey[int]("a"), func(*ext.Session) int { return 2 })
		ext.Use(r, key)
		ext.Setting(r, "mode", ext.Schema[int]{Title: func() string { return "Mode" }, Scope: ext.Both, Choices: []ext.Choice[int]{{Value: 2, Desc: func() string { return "Two" }}}})
	}}, ext.Module{ID: "c", Optional: []string{"a"}})
	l := h.Ledger("a")
	if l.State != ext.Off || len(l.Points) != 1 || len(l.Contributions) != 1 || len(l.Provides) != 1 || len(l.Uses) != 1 || len(l.Settings) != 1 || l.Settings[0].Title() != "Mode" || l.Settings[0].Scope != ext.Both || !reflect.DeepEqual(l.Requires, []ext.Dependency{{ID: "b", Active: true}}) || !reflect.DeepEqual(l.UsedOptionallyBy, []string{"c"}) {
		t.Fatalf("%+v", l)
	}
	if got := h.Ledger("b").NeededBy; !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatal(got)
	}
	if l.Settings[0].Schema.(ext.Schema[int]).Choices[0].Value != 2 {
		t.Fatal("schema lost")
	}
	mods := h.Modules()
	if mods[0].ID != "b" || mods[1].ID != "a" || mods[2].ID != "c" {
		t.Fatal(mods)
	}
	mods[1].Requires[0] = "mutated"
	l.Points[0] = "mutated"
	if h.Modules()[1].Requires[0] != "b" || h.Ledger("a").Points[0] != "a.items" {
		t.Fatal("snapshot aliases internal declarations")
	}
}
func TestDisableRemovesItemsServicesAndJoinsTasksBeforeOnStop(t *testing.T) {
	point := ext.PointKey[int]("host.items")
	key := ext.ServiceKey[int]("a")
	var ref *ext.Ref[int]
	var opt *ext.OptRef[int]
	var session *ext.Session
	finished := make(chan struct{})
	stopped := false
	var late atomic.Bool
	h := host(t, ext.Options{}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Contribute(r, point, "item", 1)
		ext.Provide(r, key, func(*ext.Session) int { return 9 })
		r.OnStart(func(s *ext.Session) error {
			session = s
			s.Go(func(ctx context.Context) { <-ctx.Done(); close(finished) })
			return nil
		})
		r.OnStop(func(*ext.Session) {
			select {
			case <-finished:
			default:
				t.Error("OnStop preceded task exit")
			}
			stopped = true
		})
	}}, ext.Module{ID: "b", Requires: []string{"a"}, Setup: func(r *ext.Registrar) { ref = ext.Use(r, key); opt = ext.UseOptional(r, key) }})
	if ref.Get() != 9 || len(ext.Items(h, point)) != 1 {
		t.Fatal("missing live effects")
	}
	must(t, h.Set("a", false))
	if !stopped || len(ext.Items(h, point)) != 0 {
		t.Fatal("effects remain")
	}
	if _, ok := opt.Get(); ok {
		t.Fatal("service remains")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("inactive required ref did not panic")
			}
		}()
		ref.Get()
	}()
	session.Go(func(context.Context) { late.Store(true) })
	if late.Load() {
		t.Fatal("stopped session accepted task")
	}
}
func TestStartIsTopologicalAndStopReversesLoadOrder(t *testing.T) {
	var log []string
	mod := func(id string, req, opt []string) ext.Module {
		return ext.Module{ID: id, Requires: req, Optional: opt, Setup: func(r *ext.Registrar) {
			r.OnStart(func(*ext.Session) error { log = append(log, "+"+id); return nil })
			r.OnStop(func(*ext.Session) { log = append(log, "-"+id) })
		}}
	}
	h := host(t, ext.Options{}, mod("c", nil, nil), mod("d", nil, nil), mod("b", []string{"c"}, []string{"d"}), mod("a", []string{"b"}, nil))
	h.Close()
	if want := []string{"+c", "+d", "+b", "+a", "-a", "-b", "-d", "-c"}; !reflect.DeepEqual(log, want) {
		t.Fatal(log)
	}
}
func TestOnStartFailureBlocksRequiredConsumersAndRecordsOriginalError(t *testing.T) {
	boom := errors.New("original start failure")
	finished := make(chan struct{})
	cleaned := false
	var opt *ext.OptRef[int]
	h := host(t, ext.Options{}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Provide(r, ext.ServiceKey[int]("a"), func(*ext.Session) int { return 1 })
		ext.Contribute(r, ext.PointKey[int]("host.items"), "item", 1)
		r.OnStart(func(s *ext.Session) error {
			s.Go(func(ctx context.Context) { <-ctx.Done(); close(finished) })
			return boom
		})
		r.OnStop(func(*ext.Session) {
			select {
			case <-finished:
				cleaned = true
			default:
				t.Error("failed start did not join tasks")
			}
		})
	}}, ext.Module{ID: "b", Requires: []string{"a"}, Setup: func(r *ext.Registrar) {
		r.OnStart(func(*ext.Session) error { t.Error("required consumer started"); return nil })
	}}, ext.Module{ID: "c", Optional: []string{"a"}, Setup: func(r *ext.Registrar) { opt = ext.UseOptional(r, ext.ServiceKey[int]("a")) }}, ext.Module{ID: "d"})
	state(t, h, "a", ext.Failed)
	state(t, h, "b", ext.Off)
	state(t, h, "c", ext.On)
	state(t, h, "d", ext.On)
	l := h.Ledger("a")
	if len(l.Errors) != 1 || l.Errors[0].Err != boom || l.Errors[0].Stage != "OnStart" || !cleaned {
		t.Fatalf("%+v cleaned %v", l, cleaned)
	}
	if _, ok := opt.Get(); ok || len(ext.Items(h, ext.PointKey[int]("host.items"))) != 0 {
		t.Fatal("failed effects remain")
	}
}
func TestBookSwitchStopsAllBeforeRestartAndRefreshesPathsAndServices(t *testing.T) {
	var log []string
	var dirs []string
	var ref *ext.Ref[string]
	root := t.TempDir()
	var newest *ext.Session
	h := host(t, ext.Options{BookDir: workspace.ExtDir, LocalRoot: root}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Provide(r, ext.ServiceKey[string]("a"), func(s *ext.Session) string { d, _ := s.BookDir(); return d })
		r.OnStart(func(s *ext.Session) error {
			newest = s
			d, ok := s.BookDir()
			if ok {
				dirs = append(dirs, d)
			}
			log = append(log, "+a")
			if s.ID() != "a" || s.LocalDir() != filepath.Join(root, "ext", "a") {
				t.Error("wrong session identity")
			}
			return nil
		})
		r.OnStop(func(*ext.Session) { log = append(log, "-a") })
	}}, ext.Module{ID: "b", Requires: []string{"a"}, Setup: func(r *ext.Registrar) {
		ref = ext.Use(r, ext.ServiceKey[string]("a"))
		r.OnStart(func(*ext.Session) error { log = append(log, "+b"); return nil })
		r.OnStop(func(*ext.Session) { log = append(log, "-b") })
	}})
	if d, ok := newest.BookDir(); ok || d != "" {
		t.Fatal(d, ok)
	}
	for _, dir := range []string{"one", "two"} {
		log = nil
		must(t, h.Open(ext.Book{Dir: dir}))
		if !reflect.DeepEqual(log, []string{"-b", "-a", "+a", "+b"}) {
			t.Fatal(log)
		}
		if ref.Get() != filepath.Join(dir, "ext", "a") {
			t.Fatal(ref.Get())
		}
	}
	if len(dirs) != 2 {
		t.Fatal(dirs)
	}
	must(t, h.Open(ext.Book{}))
	if _, ok := newest.BookDir(); ok {
		t.Fatal("book path on shelf")
	}
}
func TestThirtyModulesBookSwitchHasBoundedOverhead(t *testing.T) {
	mods := make([]ext.Module, 30)
	starts, stops := 0, 0
	for i := range mods {
		mods[i] = ext.Module{ID: fmt.Sprintf("m%d", i), Setup: func(r *ext.Registrar) {
			r.OnStart(func(*ext.Session) error { starts++; return nil })
			r.OnStop(func(*ext.Session) { stops++ })
		}}
	}
	h := host(t, ext.Options{}, mods...)
	must(t, h.Open(ext.Book{Dir: "one"}))
	begin := time.Now()
	must(t, h.Open(ext.Book{Dir: "two"}))
	elapsed := time.Since(begin)
	// No IO or task work is included. 200ms leaves scheduler and race-build
	// headroom on ordinary CI machines while catching blocking host overhead.
	if elapsed > 200*time.Millisecond {
		t.Fatalf("book switch took %v", elapsed)
	}
	if starts != 90 || stops != 60 {
		t.Fatal(starts, stops)
	}
	t.Logf("30 modules switched in %v (limit 200ms)", elapsed)
}
func TestItemsKeepAssemblyAndRegistrationOrder(t *testing.T) {
	point := ext.PointKey[int]("a.items")
	h := host(t, ext.Options{}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Declare(r, point)
		ext.Contribute(r, point, "one", 1)
		ext.Contribute(r, point, "two", 2)
	}}, ext.Module{ID: "b", Requires: []string{"a"}, Setup: func(r *ext.Registrar) { ext.Contribute(r, point, "three", 3) }})
	got := ext.Items(h, point)
	want := []ext.Item[int]{{ID: "a.one", Owner: "a", Value: 1}, {ID: "a.two", Owner: "a", Value: 2}, {ID: "b.three", Owner: "b", Value: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
}

func TestStartingStateAndErrorHistorySurviveSuccessfulRetry(t *testing.T) {
	var h *ext.Host
	attempts := 0
	boom := errors.New("first attempt failed")
	h = host(t, ext.Options{LocalSwitches: &localStore{off: []string{"a"}}}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		r.OnStart(func(*ext.Session) error {
			state(t, h, "a", ext.Starting)
			attempts++
			if attempts == 1 {
				return boom
			}
			return nil
		})
	}})
	must(t, h.Set("a", true))
	state(t, h, "a", ext.Failed)
	must(t, h.Set("a", false))
	must(t, h.Set("a", true))
	state(t, h, "a", ext.On)
	if got := h.Ledger("a").Errors; len(got) != 1 || got[0].Err != boom {
		t.Fatal(got)
	}
}
func TestConcurrentTaskReadsAndSessionShutdown(t *testing.T) {
	var ref *ext.OptRef[int]
	var pref *ext.Pref[int]
	var h *ext.Host
	var saved *ext.Session
	ready := make(chan struct{})
	var reads atomic.Int64
	h = host(t, ext.Options{}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) { ext.Provide(r, ext.ServiceKey[int]("a"), func(*ext.Session) int { return 1 }) }}, ext.Module{ID: "b", Optional: []string{"a"}, Setup: func(r *ext.Registrar) {
		ref = ext.UseOptional(r, ext.ServiceKey[int]("a"))
		pref = ext.Setting(r, "n", ext.Schema[int]{})
		r.OnStart(func(s *ext.Session) error {
			saved = s
			s.Go(func(ctx context.Context) {
				<-ready
				for {
					select {
					case <-ctx.Done():
						return
					default:
						ref.Get()
						pref.Get()
						h.Ledger("a")
						ext.Items(h, ext.PointKey[int]("host.items"))
						reads.Add(1)
					}
				}
			})
			return nil
		})
	}})
	close(ready)
	for range 30 {
		must(t, h.Set("a", false))
		pref.Set(1)
		must(t, h.Set("a", true))
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				saved.Go(func(ctx context.Context) { <-ctx.Done() })
			}
		}()
	}
	must(t, h.Set("b", false))
	wg.Wait()
}
