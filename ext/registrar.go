package ext

import (
	"fmt"
	"reflect"
	"strings"
)

type point struct {
	owner string
	typ   reflect.Type
}
type contribution struct {
	id, owner, key string
	value          any
	typ            reflect.Type
}
type provider struct {
	owner string
	typ   reflect.Type
	build func(*Session) serviceValue
}
type serviceValue struct{ value any }
type serviceUse struct {
	owner, key string
	optional   bool
	typ        reflect.Type
}
type entry struct {
	loadIndex int
	mod       Module
	ledger    Ledger
	ids       map[string]bool
	starts    []func(*Session) error
	stops     []func(*Session)
	session   *Session
}

// Registrar is valid only during its module's Setup. It exposes no IO or paths.
type Registrar struct {
	h      *Host
	e      *entry
	closed bool
}

func owner(key string) string { id, _, _ := strings.Cut(key, "."); return id }
func contains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
func (r *Registrar) check() {
	if r.closed {
		panic("registration outside Setup")
	}
}
func (r *Registrar) fail(format string, args ...any) { panic(fmt.Sprintf(format, args...)) }
func (r *Registrar) claim(id string) {
	r.check()
	if r.e.ids[id] {
		r.fail("duplicate registration id %q", id)
	}
	r.e.ids[id] = true
}
func (r *Registrar) owned(key, operation string) {
	r.check()
	if owner(key) != r.e.mod.ID {
		r.fail("%s key %q is not owned by %s", operation, key, r.e.mod.ID)
	}
}

// Declare opens a typed point owned by this module.
func Declare[T any](r *Registrar, key PointKey[T]) {
	k := string(key)
	r.owned(k, "Declare")
	r.claim(k)
	if !strings.Contains(k, ".") {
		r.fail("Declare key %q needs an owner prefix", k)
	}
	r.h.points[k] = point{r.e.mod.ID, reflect.TypeFor[T]()}
	r.e.ledger.Points = append(r.e.ledger.Points, k)
}

// Contribute records a value under an owned, declared dependency, or host point.
func Contribute[T any](r *Registrar, key PointKey[T], name string, value T) {
	r.check()
	k := string(key)
	o := owner(k)
	m := r.e.mod
	if o != m.ID && o != HostOwner && !contains(m.Requires, o) && !contains(m.Optional, o) {
		r.fail("Contribute target %q is not a declared dependency", k)
	}
	if name == "" {
		r.fail("Contribute needs a name")
	}
	id := m.ID + "." + name
	r.claim(id)
	r.h.contributions = append(r.h.contributions, contribution{id, m.ID, k, value, reflect.TypeFor[T]()})
	r.e.ledger.Contributions = append(r.e.ledger.Contributions, Contribution{id, k})
}

// Provide records a factory; construction is deferred until the session starts.
// In DeclareHost, factories run once after all Setup callbacks and before any
// module starts. They receive a host session with an empty Book, lasting for
// the Host's lifetime; host services remain available across Close and Open.
func Provide[T any](r *Registrar, key ServiceKey[T], factory func(*Session) T) {
	k := string(key)
	r.owned(k, "Provide")
	r.claim(k)
	if factory == nil {
		r.fail("Provide %q has a nil factory", k)
	}
	r.h.providers[k] = provider{r.e.mod.ID, reflect.TypeFor[T](), func(s *Session) serviceValue { return serviceValue{factory(s)} }}
	r.h.providerOrder = append(r.h.providerOrder, k)
	r.e.ledger.Provides = append(r.e.ledger.Provides, k)
}

func use[T any](r *Registrar, key ServiceKey[T], optional bool) {
	r.check()
	k := string(key)
	m := r.e.mod
	o := owner(k)
	if m.ID != HostOwner && o != HostOwner && !contains(m.Requires, o) && !(optional && contains(m.Optional, o)) {
		r.fail("Use service %q requires a declared %s dependency", k, map[bool]string{true: "Requires/Optional", false: "Requires"}[optional])
	}
	r.h.uses = append(r.h.uses, serviceUse{m.ID, k, optional, reflect.TypeFor[T]()})
	r.e.ledger.Uses = append(r.e.ledger.Uses, ServiceUse{k, optional})
}

// Ref resolves the current service on each call, including after a book switch.
type Ref[T any] struct {
	h   *Host
	key string
}

// Use records a required service dependency.
func Use[T any](r *Registrar, key ServiceKey[T]) *Ref[T] {
	use(r, key, false)
	return &Ref[T]{r.h, string(key)}
}

// Get panics during Setup or while the provider is inactive.
func (r *Ref[T]) Get() T {
	v, ok := serviceGet[T](r.h, r.key)
	if !ok {
		panic(fmt.Sprintf("service %q is inactive", r.key))
	}
	return v
}

// OptRef resolves a service that can disappear without restarting its consumer.
type OptRef[T any] struct {
	h   *Host
	key string
}

// UseOptional permits either a required or an optional declared dependency.
func UseOptional[T any](r *Registrar, key ServiceKey[T]) *OptRef[T] {
	use(r, key, true)
	return &OptRef[T]{r.h, string(key)}
}

// Get returns false while the provider is inactive, and panics during Setup.
func (r *OptRef[T]) Get() (T, bool) { return serviceGet[T](r.h, r.key) }

func serviceGet[T any](h *Host, key string) (T, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.setup {
		panic("Ref.Get during Setup is forbidden")
	}
	var zero T
	p, exists := h.providers[key]
	if !exists || p.owner != HostOwner && h.byID[p.owner].ledger.State != On {
		return zero, false
	}
	v, ok := h.services[key]
	if !ok {
		return zero, false
	}
	if v.value == nil {
		return zero, true
	}
	return v.value.(T), true
}

// OnStart registers initialization after required services become available.
func (r *Registrar) OnStart(f func(*Session) error) { r.check(); r.e.starts = append(r.e.starts, f) }

// OnStop registers cleanup after session tasks have exited; callbacks run in reverse order.
func (r *Registrar) OnStop(f func(*Session)) { r.check(); r.e.stops = append(r.e.stops, f) }
