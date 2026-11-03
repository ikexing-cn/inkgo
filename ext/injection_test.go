package ext_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
	"testing"

	"writer/ext"
)

const hostHook = ext.HookKey[int, int]("host.call")
const ownedHook = ext.HookKey[int, int]("base.call")

func hookOptions() ext.Options {
	return ext.Options{DeclareHost: func(r *ext.Registrar) { ext.DeclareHook(r, hostHook) }}
}
func baseModule() ext.Module {
	return ext.Module{ID: "base", Setup: func(r *ext.Registrar) { ext.DeclareHook(r, ownedHook) }}
}
func identity(n int) int { return n }
func callInt(t *testing.T, h *ext.Host, key ext.HookKey[int, int], in, want int) {
	t.Helper()
	got, err := ext.Call(h, key, in, identity)
	if err != nil || got != want {
		t.Fatalf("Call(%s,%d)=(%d,%v), want %d", key, in, got, err, want)
	}
}

func TestCallWithoutInjectionsAllocatesZero(t *testing.T) {
	h := host(t, hookOptions(), baseModule())
	for _, key := range []ext.HookKey[int, int]{hostHook, ownedHook} {
		if n := testing.AllocsPerRun(1000, func() {
			v, err := ext.Call(h, key, 7, identity)
			if err != nil || v != 7 {
				panic("bad call")
			}
		}); n != 0 {
			t.Fatalf("%s: %v allocations", key, n)
		}
	}
}

func TestCallUsesSuppliedOriginalWhenNoInjectionIsActive(t *testing.T) {
	var calls ext.CallPoint[command, int, int]
	h := host(t, commandOptions(&calls), exporter(), replacer("a", 10))
	must(t, h.Set("a", false))
	if v, err := ext.Call(h, ext.HookOf(calls, "export.docx"), 3, func(n int) int { return n * 2 }); v != 6 || err != nil {
		t.Fatal(v, err)
	}
}

func TestBeforeAfterAroundComposeInExecutionOrder(t *testing.T) {
	var log []string
	h := host(t, hookOptions(), ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Before(r, hostHook, "before1", func(n *int) { log = append(log, "b1"); *n += 2 })
		ext.Before(r, hostHook, "before2", func(n *int) { log = append(log, "b2"); *n *= 3 })
		ext.Around(r, hostHook, "around1", func(n int, next func(int) int) int {
			log = append(log, "r1+")
			v := next(n)
			log = append(log, "r1-")
			return v + 1
		})
		ext.Around(r, hostHook, "around2", func(n int, next func(int) int) int {
			log = append(log, "r2+")
			v := next(n)
			log = append(log, "r2-")
			return v * 2
		})
		ext.After(r, hostHook, "after1", func(n int, v *int) { log = append(log, "a1"); *v += n })
		ext.After(r, hostHook, "after2", func(_ int, v *int) { log = append(log, "a2"); *v *= 2 })
	}})
	v, err := ext.Call(h, hostHook, 1, func(n int) int { log = append(log, "original"); return n })
	must(t, err)
	if v != 56 || !reflect.DeepEqual(log, []string{"b1", "b2", "r1+", "r2+", "original", "r2-", "r1-", "a1", "a2"}) {
		t.Fatal(v, log)
	}
	links := h.Chain(string(hostHook))
	if len(links) != 7 || links[4].Original != true {
		t.Fatal(links)
	}
}

func TestAroundMaySkipOriginal(t *testing.T) {
	h := host(t, hookOptions(), ext.Module{ID: "a", Setup: func(r *ext.Registrar) { ext.Around(r, hostHook, "skip", func(int, func(int) int) int { return 42 }) }})
	v, err := ext.Call(h, hostHook, 0, func(int) int { t.Fatal("original called"); return 0 })
	if err != nil || v != 42 {
		t.Fatal(v, err)
	}
}

type injectionStore struct {
	prefs            map[string]ext.InjectionPreference
	saves            int
	loadErr, saveErr error
}

func (s *injectionStore) Load() (map[string]ext.InjectionPreference, error) {
	return s.prefs, s.loadErr
}
func (s *injectionStore) Save(v map[string]ext.InjectionPreference) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saves++
	s.prefs = v
	return nil
}

func orderModule() ext.Module {
	return ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Before(r, hostHook, "add", func(n *int) { *n += 1 })
		ext.Before(r, hostHook, "multiply", func(n *int) { *n *= 2 })
	}}
}

func TestSetOrderPersistsAndNewHostRestoresOrder(t *testing.T) {
	store := &injectionStore{}
	opts := hookOptions()
	opts.Injections = store
	h := host(t, opts, orderModule())
	callInt(t, h, hostHook, 1, 4)
	must(t, h.SetOrder(string(hostHook), []string{"a.multiply", "a.add"}))
	callInt(t, h, hostHook, 1, 3)
	if store.saves != 1 || !slices.Equal(store.prefs[string(hostHook)].Order, []string{"a.multiply", "a.add"}) {
		t.Fatal(store)
	}
	h2 := host(t, opts, orderModule())
	callInt(t, h2, hostHook, 1, 3)
	if h2.Chain(string(hostHook))[0].ID != "a.multiply" {
		t.Fatal(h2.Chain(string(hostHook)))
	}
}

func overwriter(id string, n int) ext.Module {
	return ext.Module{ID: id, Setup: func(r *ext.Registrar) { ext.Overwrite(r, hostHook, "replace", func(int) int { return n }) }}
}

func TestOverwriteConflictsChoiceAndDisabledChoiceFallback(t *testing.T) {
	store := &injectionStore{}
	opts := hookOptions()
	opts.Injections = store
	h := host(t, opts, overwriter("a", 10), overwriter("b", 20), overwriter("c", 30))
	must(t, h.Set("c", false))
	callInt(t, h, hostHook, 1, 1)
	for _, id := range []string{"a", "b"} {
		i := h.Ledger(id).Injections[0]
		if i.Active || !i.Conflict || i.Reason != (ext.Reason{Kind: ext.ReasonConflict}) {
			t.Fatal(i)
		}
	}
	must(t, h.Choose(string(hostHook), "a.replace"))
	callInt(t, h, hostHook, 1, 10)
	if i := h.Ledger("b").Injections[0]; i.Active || i.Reason != (ext.Reason{Kind: ext.ReasonNotSelected, ID: "a.replace"}) {
		t.Fatal(i)
	}
	h2 := host(t, opts, overwriter("a", 10), overwriter("b", 20))
	callInt(t, h2, hostHook, 1, 10)
	must(t, h.Set("c", true))
	must(t, h.Set("a", false))
	callInt(t, h, hostHook, 1, 1)
	for _, id := range []string{"b", "c"} {
		if !h.Ledger(id).Injections[0].Conflict {
			t.Fatal(h.Ledger(id))
		}
	}
	must(t, h.Set("b", false))
	callInt(t, h, hostHook, 1, 30)
}

type command struct {
	Title string
	Run   func(int) int
}

const commandPoint = ext.PointKey[command]("host.commands")

func commandOptions(calls *ext.CallPoint[command, int, int]) ext.Options {
	return ext.Options{DeclareHost: func(r *ext.Registrar) {
		ext.Declare(r, commandPoint)
		*calls = ext.DeclareCalls(r, commandPoint, func(v command) func(int) int { return v.Run })
	}}
}
func exporter() ext.Module {
	return ext.Module{ID: "export", Setup: func(r *ext.Registrar) { ext.Contribute(r, commandPoint, "docx", command{"Original", identity}) }}
}
func replacer(id string, n int) ext.Module {
	return ext.Module{ID: id, Optional: []string{"export"}, Setup: func(r *ext.Registrar) {
		ext.Replace(r, commandPoint, "export.docx", "replace", command{id, func(int) int { return n }})
	}}
}

func TestReplaceChangesWholeItemAndRestoresOriginalOnDisable(t *testing.T) {
	var calls ext.CallPoint[command, int, int]
	h := host(t, commandOptions(&calls), exporter(), replacer("a", 10), replacer("b", 20))
	key := ext.HookOf(calls, "export.docx")
	if v := ext.Items(h, commandPoint)[0]; v.Value.Title != "Original" || v.ReplacedBy != "" {
		t.Fatal(v)
	}
	for _, id := range []string{"a", "b"} {
		if !h.Ledger(id).Injections[0].Conflict {
			t.Fatal(h.Ledger(id))
		}
	}
	must(t, h.Choose(string(key), "a.replace"))
	item := ext.Items(h, commandPoint)[0]
	if item.Value.Title != "a" || item.ReplacedBy != "a.replace" || item.Owner != "export" {
		t.Fatal(item)
	}
	callInt(t, h, key, 1, 10)
	must(t, h.Set("b", false))
	must(t, h.Set("a", false))
	item = ext.Items(h, commandPoint)[0]
	if item.Value.Title != "Original" || item.ReplacedBy != "" {
		t.Fatal(item)
	}
	callInt(t, h, key, 1, 1)
}

func TestReplaceAndOverwriteShareOneExclusiveSlot(t *testing.T) {
	var calls ext.CallPoint[command, int, int]
	opts := commandOptions(&calls)
	h := host(t, opts, exporter(), replacer("a", 10), ext.Module{ID: "b", Optional: []string{"export"}, Setup: func(r *ext.Registrar) {
		ext.Overwrite(r, ext.HookOf(calls, "export.docx"), "overwrite", func(int) int { return 20 })
	}})
	key := ext.HookOf(calls, "export.docx")
	callInt(t, h, key, 1, 1)
	must(t, h.Choose(string(key), "b.overwrite"))
	callInt(t, h, key, 1, 20)
	if ext.Items(h, commandPoint)[0].Value.Title != "Original" {
		t.Fatal("Overwrite replaced metadata")
	}
}

func TestInactiveTargetDoesNotRestartOptionalInjector(t *testing.T) {
	starts := 0
	h := host(t, ext.Options{LocalSwitches: &localStore{off: []string{"base"}}}, baseModule(), ext.Module{ID: "a", Optional: []string{"base"}, Setup: func(r *ext.Registrar) {
		ext.Before(r, ownedHook, "add", func(n *int) { *n++ })
		r.OnStart(func(*ext.Session) error { starts++; return nil })
	}})
	state(t, h, "a", ext.On)
	i := h.Ledger("a").Injections[0]
	if i.Active || i.Reason != (ext.Reason{Kind: ext.ReasonTargetOff, ID: "base"}) {
		t.Fatal(i)
	}
	must(t, h.Set("base", true))
	callInt(t, h, ownedHook, 1, 2)
	if starts != 1 || !h.Ledger("a").Injections[0].Active {
		t.Fatal(starts, h.Ledger("a"))
	}
}

func TestInjectionRejectsUndeclaredDependency(t *testing.T) {
	assemblyError(t, "not a declared dependency", baseModule(), ext.Module{ID: "a", Setup: func(r *ext.Registrar) { ext.Before(r, ownedHook, "bad", func(*int) {}) }})
}
func TestInjectionRejectsWrongHookTypes(t *testing.T) {
	assemblyError(t, "different type", baseModule(), ext.Module{ID: "a", Optional: []string{"base"}, Setup: func(r *ext.Registrar) { ext.Before(r, ext.HookKey[string, int](ownedHook), "bad", func(*string) {}) }})
}
func TestInjectionRejectsMissingHook(t *testing.T) {
	assemblyError(t, "undeclared", baseModule(), ext.Module{ID: "a", Optional: []string{"base"}, Setup: func(r *ext.Registrar) { ext.Before(r, ext.HookKey[int, int]("base.missing"), "bad", func(*int) {}) }})
}

func TestDisableRemovesInjectorFromEveryChain(t *testing.T) {
	other := ext.HookKey[int, int]("host.other")
	opts := hookOptions()
	opts.DeclareHost = func(r *ext.Registrar) { ext.DeclareHook(r, hostHook); ext.DeclareHook(r, other) }
	h := host(t, opts, ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Before(r, hostHook, "one", func(n *int) { *n++ })
		ext.After(r, other, "two", func(_ int, n *int) { *n++ })
	}})
	must(t, h.Set("a", false))
	for _, key := range []ext.HookKey[int, int]{hostHook, other} {
		for _, link := range h.Chain(string(key)) {
			if link.Owner == "a" {
				t.Fatal(link)
			}
		}
		callInt(t, h, key, 1, 1)
	}
	if len(h.Ledger("a").Injections) != 2 {
		t.Fatal("inactive declarations lost")
	}
}

func TestPrebuiltChainsTrackSwitchOrderChoiceAndBook(t *testing.T) {
	h := host(t, hookOptions(), orderModule(), overwriter("b", 10), overwriter("c", 20))
	check := func(want int, ids ...string) {
		t.Helper()
		callInt(t, h, hostHook, 1, want)
		var got []string
		for _, link := range h.Chain(string(hostHook)) {
			if link.Active {
				got = append(got, link.ID)
			}
		}
		if !slices.Equal(got, ids) {
			t.Fatal(got, ids)
		}
	}
	check(4, "a.add", "a.multiply", "host.call")
	must(t, h.Choose(string(hostHook), "b.replace"))
	check(10, "a.add", "a.multiply", "b.replace")
	must(t, h.SetOrder(string(hostHook), []string{"a.multiply", "a.add"}))
	check(10, "a.multiply", "a.add", "b.replace")
	must(t, h.Set("b", false))
	check(20, "a.multiply", "a.add", "c.replace")
	must(t, h.Set("c", false))
	check(3, "a.multiply", "a.add", "host.call")
	must(t, h.Open(ext.Book{Dir: "one"}))
	check(3, "a.multiply", "a.add", "host.call")
	must(t, h.SetBook("a", false))
	check(1, "host.call")
	must(t, h.Open(ext.Book{Dir: "two"}))
	check(3, "a.multiply", "a.add", "host.call")
}

func TestConcurrentCallsAndSwitchesUseSafeChainSnapshots(t *testing.T) {
	h := host(t, hookOptions(), orderModule(), ext.Module{ID: "b", Setup: func(r *ext.Registrar) {
		ext.Around(r, hostHook, "wrap", func(n int, next func(int) int) int { return next(n) + 10 })
	}})
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 300 {
				v, err := ext.Call(h, hostHook, 1, identity)
				if err != nil || (v != 4 && v != 14 && v != 1 && v != 11 && v != 2 && v != 12) {
					t.Errorf("result %d %v", v, err)
				}
				h.Chain(string(hostHook))
			}
		}()
	}
	for range 60 {
		must(t, h.Set("a", false))
		must(t, h.Set("b", false))
		must(t, h.Set("a", true))
		must(t, h.Set("b", true))
	}
	wg.Wait()
}

func TestInjectionPanicFallsBackAndIsolatesRequiredConsumers(t *testing.T) {
	for _, kind := range []string{"Before", "After", "Around", "Overwrite"} {
		t.Run(kind, func(t *testing.T) {
			boom := errors.New("injection original panic")
			stops := []string{}
			var notified []ext.Failure
			opts := ext.Options{OnFailure: func(_ string, f ext.Failure) { notified = append(notified, f) }}
			h := host(t, opts, baseModule(), ext.Module{ID: "a", Optional: []string{"base"}, Setup: func(r *ext.Registrar) {
				switch kind {
				case "Before":
					ext.Before(r, ownedHook, "bad", func(n *int) { *n = 999; panic(boom) })
				case "After":
					ext.After(r, ownedHook, "bad", func(_ int, n *int) { *n = 999; panic(boom) })
				case "Around":
					ext.Around(r, ownedHook, "bad", func(int, func(int) int) int { panic(boom) })
				case "Overwrite":
					ext.Overwrite(r, ownedHook, "bad", func(int) int { panic(boom) })
				}
				r.OnStop(func(*ext.Session) { stops = append(stops, "a") })
			}}, ext.Module{ID: "dep", Requires: []string{"a"}, Setup: func(r *ext.Registrar) { r.OnStop(func(*ext.Session) { stops = append(stops, "dep") }) }}, ext.Module{ID: "other", Optional: []string{"a", "base"}, Setup: func(r *ext.Registrar) { ext.After(r, ownedHook, "good", func(_ int, n *int) { *n += 2 }) }})
			callInt(t, h, ownedHook, 3, 5)
			state(t, h, "a", ext.Failed)
			state(t, h, "dep", ext.Off)
			state(t, h, "base", ext.On)
			state(t, h, "other", ext.On)
			if len(notified) != 1 || notified[0].Err != boom || notified[0].Stage != kind || notified[0].Target != string(ownedHook) || notified[0].Injection != "a.bad" || notified[0].Chain != string(ownedHook) || notified[0].Module != "a" {
				t.Fatal(notified)
			}
			must(t, h.SetOrder(string(ownedHook), nil))
			if !slices.Equal(stops, []string{"dep", "a"}) {
				t.Fatal(stops)
			}
			callInt(t, h, ownedHook, 3, 5)
		})
	}
}

func TestOriginalPanicReturnsRecognizableErrorAndStopsRequiredConsumers(t *testing.T) {
	boom := errors.New("original panic")
	stopped := false
	h := host(t, ext.Options{}, baseModule(), ext.Module{ID: "dep", Requires: []string{"base"}, Setup: func(r *ext.Registrar) { r.OnStop(func(*ext.Session) { stopped = true }) }})
	v, err := ext.Call(h, ownedHook, 1, func(int) int { panic(boom) })
	if v != 0 || !errors.Is(err, ext.ErrCallFailed) || !errors.Is(err, boom) {
		t.Fatal(v, err)
	}
	state(t, h, "base", ext.Failed)
	state(t, h, "dep", ext.Off)
	must(t, h.Open(ext.Book{}))
	if !stopped {
		t.Fatal("dependent not stopped before next mutation")
	}
}

func TestHostOriginalPanicEscapesEvenInsideAround(t *testing.T) {
	h := host(t, hookOptions(), ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Around(r, hostHook, "wrap", func(n int, next func(int) int) int { return next(n) })
	}})
	defer func() {
		if v := recover(); v != "host boom" {
			t.Fatalf("panic %v", v)
		}
		state(t, h, "a", ext.On)
	}()
	ext.Call(h, hostHook, 1, func(int) int { panic("host boom") })
}

func TestOnStartPanicIsFailureAndBlocksRequiredStart(t *testing.T) {
	h := host(t, ext.Options{}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) { r.OnStart(func(*ext.Session) error { panic("start boom") }) }}, ext.Module{ID: "b", Requires: []string{"a"}})
	state(t, h, "a", ext.Failed)
	state(t, h, "b", ext.Off)
	f := h.Ledger("a").Errors[0]
	if f.Stage != "OnStart" || f.Err.Error() != "start boom" {
		t.Fatal(f)
	}
}
func TestFactoryPanicIsFailureAndCleansPartialServices(t *testing.T) {
	cleaned := false
	h := host(t, ext.Options{}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Provide(r, ext.ServiceKey[int]("a.one"), func(*ext.Session) int { return 1 })
		ext.Provide(r, ext.ServiceKey[int]("a.two"), func(*ext.Session) int { panic("factory boom") })
		r.OnStop(func(*ext.Session) { cleaned = true })
	}}, ext.Module{ID: "b", Requires: []string{"a"}})
	state(t, h, "a", ext.Failed)
	state(t, h, "b", ext.Off)
	if f := h.Ledger("a").Errors[0]; f.Stage != "Provide a.two" || f.Err.Error() != "factory boom" || !cleaned {
		t.Fatal(f, cleaned)
	}
}
func TestOnStopPanicDoesNotInterruptRemainingCleanup(t *testing.T) {
	var log []string
	h := host(t, ext.Options{}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		r.OnStop(func(*ext.Session) { log = append(log, "a1") })
		r.OnStop(func(*ext.Session) { panic("stop boom") })
	}}, ext.Module{ID: "b", Requires: []string{"a"}, Setup: func(r *ext.Registrar) { r.OnStop(func(*ext.Session) { log = append(log, "b") }) }})
	h.Close()
	if !slices.Equal(log, []string{"b", "a1"}) {
		t.Fatal(log)
	}
	f := h.Ledger("a").Errors[0]
	if f.Stage != "OnStop" || f.Err.Error() != "stop boom" {
		t.Fatal(f)
	}
}

func TestTaskCanFailOwnCallWithoutWaitingForItself(t *testing.T) {
	var h *ext.Host
	ready, done := make(chan struct{}), make(chan struct{})
	h = host(t, ext.Options{}, ext.Module{ID: "base", Setup: func(r *ext.Registrar) {
		ext.DeclareHook(r, ownedHook)
		r.OnStart(func(s *ext.Session) error {
			s.Go(func(context.Context) {
				<-ready
				ext.Call(h, ownedHook, 1, func(int) int { panic("task failure") })
				close(done)
			})
			return nil
		})
	}})
	close(ready)
	<-done
	h.Close()
	state(t, h, "base", ext.Failed)
}

func TestReplacePanicBlamesReplacementAndUsesOriginalValueCall(t *testing.T) {
	var calls ext.CallPoint[command, int, int]
	h := host(t, commandOptions(&calls), exporter(), ext.Module{ID: "a", Optional: []string{"export"}, Setup: func(r *ext.Registrar) {
		ext.Replace(r, commandPoint, "export.docx", "bad", command{"Bad", func(int) int { panic("replacement boom") }})
	}})
	item := ext.Items(h, commandPoint)[0]
	v, err := ext.Call(h, ext.HookOf(calls, item.ID), 7, item.Value.Run)
	if v != 7 || err != nil {
		t.Fatal(v, err)
	}
	state(t, h, "a", ext.Failed)
	state(t, h, "export", ext.On)
	f := h.Ledger("a").Errors[0]
	if f.Stage != "Replace" || f.Err.Error() != "replacement boom" {
		t.Fatal(f)
	}
	if ext.Items(h, commandPoint)[0].ReplacedBy != "" {
		t.Fatal("failed replacement remains")
	}
}

func TestContributionOriginalPanicIsAttributedToContributor(t *testing.T) {
	var calls ext.CallPoint[command, int, int]
	h := host(t, commandOptions(&calls), ext.Module{ID: "export", Setup: func(r *ext.Registrar) {
		ext.Contribute(r, commandPoint, "docx", command{"Bad", func(int) int { panic("contribution boom") }})
	}}, ext.Module{ID: "dep", Requires: []string{"export"}})
	item := ext.Items(h, commandPoint)[0]
	v, err := ext.Call(h, ext.HookOf(calls, item.ID), 1, item.Value.Run)
	if v != 0 || !errors.Is(err, ext.ErrCallFailed) {
		t.Fatal(v, err)
	}
	state(t, h, "export", ext.Failed)
	state(t, h, "dep", ext.Off)
	if len(ext.Items(h, commandPoint)) != 0 || h.Ledger("export").Errors[0].Module != "export" {
		t.Fatal(h.Ledger("export"))
	}
}

func TestExtensionOwnedCallPointUsesContributionOwnerForDependencyBoundary(t *testing.T) {
	p := ext.PointKey[command]("point.commands")
	var calls ext.CallPoint[command, int, int]
	h := host(t, ext.Options{}, ext.Module{ID: "point", Setup: func(r *ext.Registrar) {
		ext.Declare(r, p)
		calls = ext.DeclareCalls(r, p, func(c command) func(int) int { return c.Run })
	}}, ext.Module{ID: "export", Optional: []string{"point"}, Setup: func(r *ext.Registrar) { ext.Contribute(r, p, "docx", command{"Original", identity}) }}, ext.Module{ID: "a", Optional: []string{"export"}, Setup: func(r *ext.Registrar) { ext.Before(r, ext.HookOf(calls, "export.docx"), "add", func(n *int) { *n++ }) }})
	key := ext.HookOf(calls, "export.docx")
	callInt(t, h, key, 1, 2)
	must(t, h.Set("point", false))
	i := h.Ledger("a").Injections[0]
	if i.Active || i.Reason != (ext.Reason{Kind: ext.ReasonTargetOff, ID: "point"}) {
		t.Fatal(i)
	}
	must(t, h.Set("point", true))
	callInt(t, h, key, 1, 2)
}

func TestInjectionIDsShareRegistrationNamespace(t *testing.T) {
	assemblyError(t, "duplicate registration id", ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Setting(r, "same", ext.Schema[int]{})
		ext.Before(r, hostHook, "same", func(*int) {})
	}})
}

func TestHookDeclarationsRejectForeignOwnerAndTypeMismatch(t *testing.T) {
	assemblyError(t, "not owned", ext.Module{ID: "a", Setup: func(r *ext.Registrar) { ext.DeclareHook(r, ownedHook) }})
	assemblyError(t, "different type", ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Declare(r, ext.PointKey[int]("a.items"))
		ext.DeclareCalls(r, ext.PointKey[string]("a.items"), func(string) func(int) int { return identity })
	}})
}

func TestInjectionPreferenceErrorsKeepLiveChainsAndRejectInvalidIDs(t *testing.T) {
	boom := errors.New("storage boom")
	store := &injectionStore{loadErr: boom}
	opts := hookOptions()
	opts.Injections = store
	if _, err := ext.New(opts, orderModule()); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	store.loadErr = nil
	h := host(t, opts, orderModule(), overwriter("b", 20))
	store.saveErr = boom
	if err := h.SetOrder(string(hostHook), []string{"a.multiply", "a.add"}); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if err := h.Choose(string(hostHook), "b.replace"); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	store.saveErr = nil
	must(t, h.Set("b", false))
	callInt(t, h, hostHook, 1, 4)
	for _, ids := range [][]string{{"missing"}, {"a.add", "a.add"}} {
		if h.SetOrder(string(hostHook), ids) == nil {
			t.Fatal(ids)
		}
	}
	if h.Choose(string(hostHook), "a.add") == nil || h.Choose(string(hostHook), "missing") == nil || h.SetOrder("host.missing", nil) == nil {
		t.Fatal("invalid preferences accepted")
	}
}
