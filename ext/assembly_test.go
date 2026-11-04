package ext_test

import (
	"fmt"
	"strings"
	"testing"

	"writer/ext"
)

func assemblyError(t *testing.T, want string, mods ...ext.Module) {
	t.Helper()
	h, err := ext.New(ext.Options{}, mods...)
	if h != nil {
		h.Close()
	}
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("New error = %v, want %q", err, want)
	}
}
func host(t *testing.T, opts ext.Options, mods ...ext.Module) *ext.Host {
	t.Helper()
	h, err := ext.New(opts, mods...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return h
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func state(t *testing.T, h *ext.Host, id string, want ext.State) {
	t.Helper()
	if got := h.Ledger(id).State; got != want {
		t.Fatalf("%s state = %v, want %v", id, got, want)
	}
}
func TestAssemblyRejectsInvalidID(t *testing.T) {
	for _, id := range []string{"", "A", "a-b", "a.b", "1a", "a_b"} {
		t.Run(fmt.Sprintf("id=%s", id), func(t *testing.T) { assemblyError(t, "invalid", ext.Module{ID: id}) })
	}
}
func TestAssemblyReservesHostOwner(t *testing.T) {
	assemblyError(t, "host", ext.Module{ID: ext.HostOwner})
}
func TestAssemblyRejectsDuplicateModuleID(t *testing.T) {
	assemblyError(t, "duplicate ID", ext.Module{ID: "a"}, ext.Module{ID: "a"})
}
func TestAssemblyRejectsMissingRequiredDependency(t *testing.T) {
	assemblyError(t, "module a: dependency", ext.Module{ID: "a", Requires: []string{"b"}})
}
func TestAssemblyRejectsMissingOptionalDependency(t *testing.T) {
	assemblyError(t, "module a: dependency", ext.Module{ID: "a", Optional: []string{"b"}})
}
func TestAssemblyRejectsDependencyCycle(t *testing.T) {
	assemblyError(t, "module a: dependency cycle", ext.Module{ID: "a", Requires: []string{"b"}}, ext.Module{ID: "b", Optional: []string{"a"}})
	assemblyError(t, "cycle", ext.Module{ID: "a", Requires: []string{"a"}})
}
func TestAssemblyRejectsDuplicateRegistrationID(t *testing.T) {
	assemblyError(t, "module a: Setup: duplicate registration id", ext.Module{ID: "a", Setup: func(r *ext.Registrar) {
		ext.Declare(r, ext.PointKey[int]("a.same"))
		ext.Setting(r, "same", ext.Schema[int]{})
	}})
}
func TestAssemblyRejectsForeignDeclareKey(t *testing.T) {
	assemblyError(t, "module a: Setup: Declare key", ext.Module{ID: "a", Setup: func(r *ext.Registrar) { ext.Declare(r, ext.PointKey[int]("b.items")) }})
}
func TestAssemblyRejectsUndeclaredContributionDependency(t *testing.T) {
	assemblyError(t, "module a: Setup: Contribute target", ext.Module{ID: "b", Setup: func(r *ext.Registrar) { ext.Declare(r, ext.PointKey[int]("b.items")) }}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) { ext.Contribute(r, ext.PointKey[int]("b.items"), "item", 1) }})
}
func TestAssemblyRejectsUseWithoutRequiredDependency(t *testing.T) {
	for _, optional := range []bool{false, true} {
		t.Run(fmt.Sprint(optional), func(t *testing.T) {
			m := ext.Module{ID: "a", Setup: func(r *ext.Registrar) { ext.Use(r, ext.ServiceKey[int]("b")) }}
			if optional {
				m.Optional = []string{"b"}
			}
			assemblyError(t, "module a: Setup: Use service", ext.Module{ID: "b"}, m)
		})
	}
}
func TestAssemblyRejectsUseOptionalWithoutDependency(t *testing.T) {
	assemblyError(t, "module a: Setup: Use service", ext.Module{ID: "a", Setup: func(r *ext.Registrar) { ext.UseOptional(r, ext.ServiceKey[int]("b")) }})
}
func TestAssemblyRejectsForeignProvideKey(t *testing.T) {
	assemblyError(t, "module a: Setup: Provide key", ext.Module{ID: "a", Setup: func(r *ext.Registrar) { ext.Provide(r, ext.ServiceKey[int]("b"), func(*ext.Session) int { return 1 }) }})
}
func TestSetupRefGetBecomesAssemblyError(t *testing.T) {
	for _, optional := range []bool{false, true} {
		t.Run(fmt.Sprint(optional), func(t *testing.T) {
			assemblyError(t, "module a: Setup: Ref.Get during Setup", ext.Module{ID: "b", Setup: func(r *ext.Registrar) { ext.Provide(r, ext.ServiceKey[int]("b"), func(*ext.Session) int { return 1 }) }}, ext.Module{ID: "a", Requires: []string{"b"}, Setup: func(r *ext.Registrar) {
				if optional {
					ext.UseOptional(r, ext.ServiceKey[int]("b")).Get()
				} else {
					ext.Use(r, ext.ServiceKey[int]("b")).Get()
				}
			}})
		})
	}
}
func TestAssemblyChecksPointAndServiceTypes(t *testing.T) {
	base := ext.Module{ID: "b", Setup: func(r *ext.Registrar) {
		ext.Declare(r, ext.PointKey[int]("b.items"))
		ext.Provide(r, ext.ServiceKey[int]("b"), func(*ext.Session) int { return 1 })
	}}
	assemblyError(t, "module a: Contribute point", base, ext.Module{ID: "a", Requires: []string{"b"}, Setup: func(r *ext.Registrar) { ext.Contribute(r, ext.PointKey[string]("b.items"), "x", "x") }})
	assemblyError(t, "module a: Use service", base, ext.Module{ID: "a", Requires: []string{"b"}, Setup: func(r *ext.Registrar) { ext.Use(r, ext.ServiceKey[string]("b")) }})
}

func TestAssemblyRejectsInconsistentHostPointTypes(t *testing.T) {
	assemblyError(t, "module b: Contribute point", ext.Module{ID: "a", Setup: func(r *ext.Registrar) { ext.Contribute(r, ext.PointKey[int]("host.items"), "one", 1) }}, ext.Module{ID: "b", Setup: func(r *ext.Registrar) { ext.Contribute(r, ext.PointKey[string]("host.items"), "two", "two") }})
}

func TestRegistrarRejectsRegistrationAfterSetup(t *testing.T) {
	var saved *ext.Registrar
	host(t, ext.Options{}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) { saved = r }})
	defer func() {
		if recover() == nil {
			t.Error("registration after Setup accepted")
		}
	}()
	ext.Declare(saved, ext.PointKey[int]("a.late"))
}
func TestSetupRunsOnceInAssemblyOrderEvenWhenDisabled(t *testing.T) {
	var log []string
	factories := 0
	starts := 0
	h := host(t, ext.Options{LocalSwitches: &localStore{off: []string{"b"}}}, ext.Module{ID: "b", Setup: func(r *ext.Registrar) {
		log = append(log, "b")
		ext.Provide(r, ext.ServiceKey[int]("b"), func(*ext.Session) int { factories++; return 1 })
		r.OnStart(func(*ext.Session) error { starts++; return nil })
	}}, ext.Module{ID: "a", Setup: func(r *ext.Registrar) { log = append(log, "a") }})
	if strings.Join(log, ",") != "b,a" || factories != 0 || starts != 0 {
		t.Fatalf("log %v factories %d starts %d", log, factories, starts)
	}
	must(t, h.Set("b", true))
	must(t, h.Open(ext.Book{Dir: "book"}))
	must(t, h.Set("b", false))
	must(t, h.Set("b", true))
	if len(log) != 2 || factories != 3 || starts != 3 {
		t.Fatalf("log %v factories %d starts %d", log, factories, starts)
	}
}

func TestConsumersBeforeRequiredAndOptionalProvidersUseStableLoadOrder(t *testing.T) {
	var events []string
	required := ext.ServiceKey[int]("required.value")
	optional := ext.ServiceKey[int]("optional.value")
	point := ext.PointKey[int]("optional.items")
	consumer := ext.Module{ID: "consumer", Requires: []string{"required"}, Optional: []string{"optional"}, Setup: func(r *ext.Registrar) {
		events = append(events, "setup consumer")
		req := ext.Use(r, required)
		opt := ext.UseOptional(r, optional)
		ext.Contribute(r, point, "item", 7)
		ext.Provide(r, ext.ServiceKey[int]("consumer.value"), func(*ext.Session) int {
			value, ok := opt.Get()
			if !ok {
				t.Fatal("optional provider not ready for consumer factory")
			}
			return req.Get() + value
		})
		r.OnStart(func(*ext.Session) error {
			value, ok := opt.Get()
			if req.Get() != 3 || !ok || value != 4 {
				t.Fatal("providers not ready for consumer start")
			}
			events = append(events, "start consumer")
			return nil
		})
		r.OnStop(func(*ext.Session) {
			if req.Get() != 3 {
				t.Fatal("required provider stopped before consumer")
			}
			if _, ok := opt.Get(); !ok {
				t.Fatal("optional provider stopped before consumer")
			}
			events = append(events, "stop consumer")
		})
	}}
	provider := func(id string, key ext.ServiceKey[int], value int) ext.Module {
		return ext.Module{ID: id, Setup: func(r *ext.Registrar) {
			events = append(events, "setup "+id)
			if id == "optional" {
				ext.Declare(r, point)
			}
			ext.Provide(r, key, func(*ext.Session) int { events = append(events, "provide "+id); return value })
			r.OnStart(func(*ext.Session) error { events = append(events, "start "+id); return nil })
			r.OnStop(func(*ext.Session) { events = append(events, "stop "+id) })
		}}
	}
	free := ext.Module{ID: "free", Setup: func(r *ext.Registrar) {
		events = append(events, "setup free")
		r.OnStart(func(*ext.Session) error { events = append(events, "start free"); return nil })
		r.OnStop(func(*ext.Session) { events = append(events, "stop free") })
	}}
	h := host(t, ext.Options{}, consumer, free, provider("optional", optional, 4), provider("required", required, 3))
	want := "setup free,setup optional,setup required,setup consumer,start free,provide optional,start optional,provide required,start required,start consumer"
	if strings.Join(events, ",") != want {
		t.Fatal(events)
	}
	if value, ok := ext.Lookup(h, ext.ServiceKey[int]("consumer.value")); !ok || value != 7 {
		t.Fatal(value, ok)
	}
	if items := ext.Items(h, point); len(items) != 1 || items[0].ID != "consumer.item" {
		t.Fatal(items)
	}
	var ids []string
	for _, m := range h.Modules() {
		ids = append(ids, m.ID)
	}
	if strings.Join(ids, ",") != "consumer,free,optional,required" {
		t.Fatal("display order changed", ids)
	}
	events = nil
	h.Close()
	if strings.Join(events, ",") != "stop consumer,stop required,stop optional,stop free" {
		t.Fatal(events)
	}
}

func TestReenabledOptionalProviderStillStopsAfterItsConsumer(t *testing.T) {
	var stopped []string
	h := host(t, ext.Options{LocalSwitches: &localStore{off: []string{"provider"}}}, ext.Module{ID: "consumer", Optional: []string{"provider"}, Setup: func(r *ext.Registrar) { r.OnStop(func(*ext.Session) { stopped = append(stopped, "consumer") }) }}, ext.Module{ID: "provider", Setup: func(r *ext.Registrar) { r.OnStop(func(*ext.Session) { stopped = append(stopped, "provider") }) }})
	must(t, h.Set("provider", true))
	h.Close()
	if strings.Join(stopped, ",") != "consumer,provider" {
		t.Fatal(stopped)
	}
}

func TestFailurePropagatesInLoadOrderWhenConsumersAreAssembledFirst(t *testing.T) {
	h := host(t, ext.Options{}, ext.Module{ID: "last", Requires: []string{"middle"}}, ext.Module{ID: "middle", Requires: []string{"base"}}, ext.Module{ID: "base"})
	_ = ext.Guard(h, "base", "failure", func() { panic("failed provider") })
	state(t, h, "base", ext.Failed)
	state(t, h, "middle", ext.Off)
	state(t, h, "last", ext.Off)
}

func TestContributionsKeepAssemblyOrderWhileRegistrationUsesLoadOrder(t *testing.T) {
	point := ext.PointKey[int]("provider.items")
	h := host(t, ext.Options{}, ext.Module{ID: "consumer", Requires: []string{"provider"}, Setup: func(r *ext.Registrar) { ext.Contribute(r, point, "first", 1); ext.Contribute(r, point, "second", 2) }}, ext.Module{ID: "provider", Setup: func(r *ext.Registrar) { ext.Declare(r, point); ext.Contribute(r, point, "own", 3) }})
	items := ext.Items(h, point)
	if len(items) != 3 || items[0].ID != "consumer.first" || items[1].ID != "consumer.second" || items[2].ID != "provider.own" {
		t.Fatal(items)
	}
}

func TestStableLoadOrderChoosesEarliestReadyAssemblyEntryAfterEachProvider(t *testing.T) {
	var setup []string
	mod := func(id string, requires []string) ext.Module {
		return ext.Module{ID: id, Requires: requires, Setup: func(*ext.Registrar) { setup = append(setup, id) }}
	}
	host(t, ext.Options{}, mod("consumer", []string{"provider"}), mod("provider", nil), mod("free", nil))
	if strings.Join(setup, ",") != "provider,consumer,free" {
		t.Fatal(setup)
	}
}
