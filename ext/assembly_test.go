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
func TestAssemblyRejectsLateRequiredDependency(t *testing.T) {
	assemblyError(t, "module a: dependency b must be assembled before", ext.Module{ID: "a", Requires: []string{"b"}}, ext.Module{ID: "b"})
}
func TestAssemblyRejectsLateOptionalDependency(t *testing.T) {
	assemblyError(t, "module a: dependency b must be assembled before", ext.Module{ID: "a", Optional: []string{"b"}}, ext.Module{ID: "b"})
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
