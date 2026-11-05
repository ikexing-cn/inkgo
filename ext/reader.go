package ext

import "reflect"

// PointReader reads a declared point at runtime without exposing host mutation.
// Obtain it during Setup; Items and CallReadPoint must run after Setup.
type PointReader[T any] struct {
	host *Host
	key  PointKey[T]
}

// ReadPoint records access to a point owned by this module or a dependency.
func ReadPoint[T any](r *Registrar, key PointKey[T]) *PointReader[T] {
	r.check()
	id := owner(string(key))
	if id != r.e.mod.ID && id != HostOwner && !contains(r.e.mod.Requires, id) && !contains(r.e.mod.Optional, id) {
		r.fail("ReadPoint requires a declared dependency: %s", key)
	}
	r.h.reads = append(r.h.reads, serviceUse{owner: r.e.mod.ID, key: string(key), typ: reflect.TypeFor[T]()})
	r.e.ledger.Reads = append(r.e.ledger.Reads, string(key))
	return &PointReader[T]{r.h, key}
}

// Items returns the point's currently active contributions; never call in Setup.
func (r *PointReader[T]) Items() []Item[T] {
	r.host.mu.RLock()
	setup := r.host.setup
	r.host.mu.RUnlock()
	if setup {
		panic("PointReader.Items during Setup is forbidden")
	}
	return Items(r.host, r.key)
}

// CallReadPoint invokes a contribution through its injection and failure boundary.
func CallReadPoint[T, In, Out any](r *PointReader[T], id string, in In, original func(In) Out) (Out, error) {
	return Call(r.host, HookOf(CallPoint[T, In, Out]{Point: r.key}, id), in, original)
}

// Lookup resolves an optional service for host consumers. Extension consumers
// must use Use or UseOptional so their dependency is recorded during Setup.
func Lookup[T any](h *Host, key ServiceKey[T]) (T, bool) { return serviceGet[T](h, string(key)) }

// Guard runs an action from this point while its contributing owner is active.
// It isolates failures in callbacks returned by a point contribution.
func (r *PointReader[T]) Guard(owner, target string, fn func()) error {
	return Guard(r.host, owner, target, fn)
}

// Revision returns the host's activation and declaration revision after Setup.
func (r *PointReader[T]) Revision() uint64 { return r.host.Changes() }

// Activation captures the contributor's current session on the frame thread.
func (r *PointReader[T]) Activation(owner string) *Session {
	r.host.mu.RLock()
	defer r.host.mu.RUnlock()
	if e := r.host.byID[owner]; e != nil {
		return e.session
	}
	return nil
}

// ActivationOf captures an active contributor's Session on the frame thread.
func ActivationOf(h *Host, owner string) *Session {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if e := h.byID[owner]; e != nil {
		return e.session
	}
	return nil
}

// CallReadOperation invokes a named operation through its typed injection boundary.
func CallReadOperation[T, In, Out any](r *PointReader[T], operation, id string, in In, original func(In) Out) (Out, error) {
	return Call(r.host, HookOf(CallPoint[T, In, Out]{Point: r.key, Operation: operation}, id), in, original)
}
