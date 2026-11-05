package ext

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// HookKey names a typed call owned by the prefix before its first dot.
type HookKey[In, Out any] string

// CallPoint retains the input/output types needed by HookOf without reflection.
type CallPoint[T, In, Out any] struct {
	Point     PointKey[T]
	Operation string
}

// HookOf identifies a contribution's call. Its owner is the contribution owner,
// not the owner of the point that collects it.
func HookOf[T, In, Out any](p CallPoint[T, In, Out], id string) HookKey[In, Out] {
	return HookKey[In, Out](callTarget(string(p.Point), id, p.Operation))
}

func itemTarget(point, id string) string { return id + "@" + point }
func callTarget(point, id, op string) string {
	k := itemTarget(point, id)
	if op != "" {
		k += ":" + op
	}
	return k
}

// InjectionPreference stores a target's order and exclusive provider choice.
// Order is a prefix; unlisted IDs retain assembly/registration order.
type InjectionPreference struct {
	Order  []string
	Choice string
}

// InjectionStore persists local author choices. Calls are serialized; Save must
// complete before returning. Unknown saved IDs are retained for future modules.
type InjectionStore interface {
	Load() (map[string]InjectionPreference, error)
	Save(map[string]InjectionPreference) error
}

// ChainLink describes a step in execution order. Original marks the base call;
// Provider names the replacement extension when the base value was replaced.
type ChainLink struct {
	Injection
	Original bool
	Provider string
}

type interception struct {
	Injection
	fn        any
	in, out   reflect.Type
	valueType reflect.Type
	call      any
}

type hook interface {
	types() (reflect.Type, reflect.Type)
	rebuild(*Host, string, []interception)
}

type hookDecl struct {
	owner, point string
	impl         hook
}

type callDeclaration struct {
	point, operation string
	typ              reflect.Type
	install          func(*Host, contribution)
	valueHook        func(any) any
}

// DeclareHook declares a module-owned call, sharing the module's ID namespace.
func DeclareHook[In, Out any](r *Registrar, key HookKey[In, Out]) {
	r.owned(string(key), "DeclareHook")
	if !strings.Contains(string(key), ".") {
		r.fail("DeclareHook needs an owner prefix and name")
	}
	r.claim(string(key))
	r.h.hooks[string(key)] = hookDecl{owner: r.e.mod.ID, impl: &typedHook[In, Out]{}}
}

// DeclareCalls identifies the function called on each value in an owned point.
// The returned typed point lets HookOf infer In and Out. It also works inside
// Options.DeclareHost, using the same Registrar as Declare.
func DeclareCalls[T, In, Out any](r *Registrar, p PointKey[T], get func(T) func(In) Out) CallPoint[T, In, Out] {
	return DeclareOperation(r, p, "", get)
}

// DeclareOperation declares one named typed operation on each point contribution.
func DeclareOperation[T, In, Out any](r *Registrar, p PointKey[T], operation string, get func(T) func(In) Out) CallPoint[T, In, Out] {
	r.owned(string(p), "DeclareOperation")
	if get == nil {
		r.fail("DeclareOperation needs an accessor")
	}
	key := string(p) + ":" + operation
	if _, exists := r.h.calls[key]; exists {
		r.fail("duplicate calls declaration %q", key)
	}
	r.h.calls[key] = callDeclaration{point: string(p), operation: operation, typ: reflect.TypeFor[T](),
		install: func(h *Host, c contribution) {
			k := callTarget(c.key, c.id, operation)
			h.hooks[k] = hookDecl{owner: c.owner, point: c.key, impl: &typedHook[In, Out]{fallback: get(c.value.(T))}}
		}, valueHook: func(v any) any { return get(v.(T)) },
	}
	r.e.ledger.Calls = append(r.e.ledger.Calls, CallInfo{Point: string(p), Operation: operation, Input: reflect.TypeFor[In]().String(), Output: reflect.TypeFor[Out]().String()})
	return CallPoint[T, In, Out]{Point: p, Operation: operation}
}

func (r *Registrar) injection(target, name string, kind InjectionKind, fn any, in, out, valueType reflect.Type) {
	r.check()
	if name == "" {
		r.fail("injection needs a name")
	}
	o := owner(target)
	m := r.e.mod
	if o != HostOwner && !contains(m.Requires, o) && !contains(m.Optional, o) {
		r.fail("injection target %q is not a declared dependency", target)
	}
	id := m.ID + "." + name
	r.claim(id)
	r.h.injections = append(r.h.injections, interception{Injection: Injection{ID: id, Target: target, Owner: m.ID, Kind: kind}, fn: fn, in: in, out: out, valueType: valueType})
}

// Before changes a call's input before any around layer executes.
func Before[In, Out any](r *Registrar, key HookKey[In, Out], name string, f func(*In)) {
	if f == nil {
		r.fail("nil Before")
	}
	r.injection(string(key), name, BeforeKind, f, reflect.TypeFor[In](), reflect.TypeFor[Out](), nil)
}

// After changes the result after every around layer has returned.
func After[In, Out any](r *Registrar, key HookKey[In, Out], name string, f func(In, *Out)) {
	if f == nil {
		r.fail("nil After")
	}
	r.injection(string(key), name, AfterKind, f, reflect.TypeFor[In](), reflect.TypeFor[Out](), nil)
}

// Around wraps the next layer; first in order is outermost. Calls to next must
// finish before the callback returns and must not escape to another goroutine.
func Around[In, Out any](r *Registrar, key HookKey[In, Out], name string, f func(In, func(In) Out) Out) {
	if f == nil {
		r.fail("nil Around")
	}
	r.injection(string(key), name, AroundKind, f, reflect.TypeFor[In](), reflect.TypeFor[Out](), nil)
}

// Overwrite supplies an exclusive base call; unresolved conflicts use the original.
func Overwrite[In, Out any](r *Registrar, key HookKey[In, Out], name string, f func(In) Out) {
	if f == nil {
		r.fail("nil Overwrite")
	}
	r.injection(string(key), name, OverwriteKind, f, reflect.TypeFor[In](), reflect.TypeFor[Out](), nil)
}

// Replace supplies an entire contribution, sharing exclusivity with Overwrite.
func Replace[T any](r *Registrar, p PointKey[T], id, name string, value T) {
	r.injection(itemTarget(string(p), id), name, ReplaceKind, value, nil, nil, reflect.TypeFor[T]())
}

func clonePreferences(src map[string]InjectionPreference) map[string]InjectionPreference {
	dst := make(map[string]InjectionPreference, len(src))
	for k, v := range src {
		v.Order = slices.Clone(v.Order)
		dst[k] = v
	}
	return dst
}

// SetOrder persists a target's ordering prefix and rebuilds the execution chains.
// Duplicate or foreign IDs are rejected; omitted IDs follow assembly order.
func (h *Host) SetOrder(target string, ids []string) error {
	return h.editInjectionPreference(target, ids, "", false)
}

// Choose persists an exclusive injection ID. Empty clears the choice. If the
// chosen provider is inactive, all remaining eligible providers conflict when
// there is more than one, or the sole remaining provider takes over.
func (h *Host) Choose(target, id string) error {
	return h.editInjectionPreference(target, nil, id, true)
}

func (h *Host) editInjectionPreference(target string, ids []string, choice string, choose bool) error {
	h.opMu.Lock()
	defer h.opMu.Unlock()
	h.drainFailures()
	h.mu.RLock()
	_, hookExists := h.hooks[target]
	known := map[string]InjectionKind{}
	for _, i := range h.injections {
		if i.Target == target {
			known[i.ID] = i.Kind
		}
	}
	prefs := clonePreferences(h.injectionPreferences)
	h.mu.RUnlock()
	if !hookExists && len(known) == 0 {
		return fmt.Errorf("unknown injection target %q", target)
	}
	p := prefs[target]
	if choose {
		if k, ok := known[choice]; choice != "" && (!ok || k < OverwriteKind) {
			return fmt.Errorf("invalid choice %q", choice)
		}
		p.Choice = choice
	} else {
		seen := map[string]bool{}
		for _, id := range ids {
			if _, ok := known[id]; !ok || seen[id] {
				return fmt.Errorf("invalid order ID %q", id)
			}
			seen[id] = true
		}
		p.Order = slices.Clone(ids)
	}
	prefs[target] = p
	if h.opts.Injections != nil {
		h.storeMu.Lock()
		err := h.opts.Injections.Save(clonePreferences(prefs))
		h.storeMu.Unlock()
		if err != nil {
			return err
		}
	}
	h.mu.Lock()
	h.injectionPreferences = prefs
	h.rebuildChains()
	h.mu.Unlock()
	return nil
}

// Chain returns a detached execution-order snapshot, including inactive entries.
// Disabled modules remain visible here for explanations, but have no active step.
func (h *Host) Chain(target string) []ChainLink {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return slices.Clone(h.chains[target])
}

func (h *Host) active(id string) bool {
	return id == HostOwner || h.byID[id] != nil && h.byID[id].ledger.State == On
}
func (h *Host) targetActive(target string) bool {
	if d, ok := h.hooks[target]; ok {
		return h.active(d.owner) && (d.point == "" || h.active(owner(d.point)) && !h.pointConflict(d.point))
	}
	for _, c := range h.contributions {
		if itemTarget(c.key, c.id) == target {
			return h.active(c.owner) && h.active(owner(c.key)) && !h.pointConflict(c.key)
		}
	}
	return false
}

// pointConflict is read under mu, including while rebuilding call chains.
func (h *Host) pointConflict(key string) bool {
	if !h.points[key].single {
		return false
	}
	n := 0
	for _, c := range h.contributions {
		if c.key == key && h.active(c.owner) {
			n++
		}
	}
	return n > 1
}

func (h *Host) inactiveTargetReason(target string) Reason {
	if d, ok := h.hooks[target]; ok {
		if !h.active(d.owner) {
			return Reason{Kind: ReasonTargetOff, ID: d.owner}
		}
		if d.point != "" && !h.active(owner(d.point)) {
			return Reason{Kind: ReasonTargetOff, ID: owner(d.point)}
		}
	}
	for _, c := range h.contributions {
		if itemTarget(c.key, c.id) == target {
			if !h.active(c.owner) {
				return Reason{Kind: ReasonTargetOff, ID: c.owner}
			}
			if !h.active(owner(c.key)) {
				return Reason{Kind: ReasonTargetOff, ID: owner(c.key)}
			}
		}
	}
	return Reason{Kind: ReasonTargetOff, ID: owner(target)}
}

func (h *Host) validateInjections() error {
	for key, c := range h.calls {
		p, ok := h.points[c.point]
		if !ok || p.typ != c.typ {
			return fmt.Errorf("DeclareCalls point %q is missing or has a different type", key)
		}
	}
	for _, c := range h.contributions {
		for _, d := range h.calls {
			if d.point != c.key {
				continue
			}
			if err := protect(func() { d.install(h, c) }); err != nil {
				return fmt.Errorf("DeclareCalls %s: %w", c.key, err)
			}
		}
	}
	for n := range h.injections {
		i := h.injections[n]
		if i.Kind == ReplaceKind {
			found := false
			for _, c := range h.contributions {
				if itemTarget(c.key, c.id) == i.Target {
					found = c.typ == i.valueType
				}
			}
			if !found {
				return fmt.Errorf("Replace target %q missing or different type", i.Target)
			}
			for _, c := range h.contributions {
				if itemTarget(c.key, c.id) == i.Target {
					if d, ok := h.calls[c.key+":"]; ok {
						if err := protect(func() { h.injections[n].call = d.valueHook(i.fn) }); err != nil {
							return err
						}
					}
				}
			}
		} else {
			d, ok := h.hooks[i.Target]
			if !ok {
				return fmt.Errorf("hook %q is undeclared", i.Target)
			}
			in, out := d.impl.types()
			if in != i.in || out != i.out {
				return fmt.Errorf("hook %q has a different type", i.Target)
			}
		}
	}
	return nil
}

// rebuildChains requires mu. Execution order and chain functions are built here;
// Call only binds the immutable chain to its own invocation frame.
func (h *Host) rebuildChains() {
	h.changes++
	groups := map[string][]interception{}
	statuses := map[string]Injection{}
	for key := range h.hooks {
		groups[key] = nil
	}
	for _, e := range h.entries {
		e.ledger.Injections = nil
	}
	for _, i := range h.injections {
		groups[i.Target] = append(groups[i.Target], i)
	}
	h.replacements = map[string]interception{}
	h.chains = map[string][]ChainLink{}
	for target, list := range groups {
		pref := h.injectionPreferences[target]
		rank := func(id string) int {
			n := slices.Index(pref.Order, id)
			if n < 0 {
				return len(pref.Order)
			}
			return n
		}
		slices.SortStableFunc(list, func(a, b interception) int { return rank(a.ID) - rank(b.ID) })
		eligible := []int{}
		for n := range list {
			i := &list[n]
			i.Order = n
			if !h.active(i.Owner) {
				i.Reason = Reason{Kind: ReasonInjectorOff, ID: i.Owner}
			} else if !h.targetActive(target) {
				i.Reason = h.inactiveTargetReason(target)
			} else {
				i.Active = true
				if i.Kind >= OverwriteKind {
					eligible = append(eligible, n)
				}
			}
		}
		chosen := -1
		for _, n := range eligible {
			if list[n].ID == pref.Choice {
				chosen = n
			}
		}
		if chosen < 0 && len(eligible) == 1 {
			chosen = eligible[0]
		}
		for _, n := range eligible {
			if n != chosen {
				list[n].Active = false
				list[n].Conflict = chosen < 0
				if chosen < 0 {
					list[n].Reason = Reason{Kind: ReasonConflict}
				} else {
					list[n].Reason = Reason{Kind: ReasonNotSelected, ID: list[chosen].ID}
				}
			}
		}
		if chosen >= 0 && list[chosen].Kind == ReplaceKind {
			h.replacements[target] = list[chosen]
		}
		for _, i := range list {
			statuses[i.ID] = i.Injection
		}
		links := []ChainLink{}
		for _, kind := range []InjectionKind{BeforeKind, AroundKind} {
			for _, i := range list {
				if i.Kind == kind && h.active(i.Owner) {
					links = append(links, ChainLink{Injection: i.Injection})
				}
			}
		}
		base := ChainLink{Original: true, Injection: Injection{ID: target, Target: target, Owner: owner(target), Active: h.targetActive(target)}}
		if !base.Active {
			base.Reason = h.inactiveTargetReason(target)
		}
		if chosen >= 0 {
			base.Active = false
			base.Reason = Reason{Kind: ReasonReplaced, ID: list[chosen].ID}
			base.Provider = list[chosen].Owner
		}
		links = append(links, base)
		for _, i := range list {
			if i.Kind >= OverwriteKind && h.active(i.Owner) {
				links = append(links, ChainLink{Injection: i.Injection, Provider: i.Owner})
			}
		}
		for _, i := range list {
			if i.Kind == AfterKind && h.active(i.Owner) {
				links = append(links, ChainLink{Injection: i.Injection})
			}
		}
		h.chains[target] = links
		if d, ok := h.hooks[target]; ok {
			d.impl.rebuild(h, target, list)
		}
	}
	for _, i := range h.injections {
		h.byID[i.Owner].ledger.Injections = append(h.byID[i.Owner].ledger.Injections, statuses[i.ID])
	}
}

// ErrCallFailed identifies a failed or inactive extension-owned original call.
var ErrCallFailed = errors.New("extension call failed")

type typedHook[In, Out any] struct {
	plan     *callPlan[In, Out]
	fallback func(In) Out
}

func (*typedHook[In, Out]) types() (reflect.Type, reflect.Type) {
	return reflect.TypeFor[In](), reflect.TypeFor[Out]()
}

// A plan is immutable after publication. Every invocation owns its mutable frame,
// so recursion and concurrent callers share only the prebuilt execution chain.
type callPlan[In, Out any] struct {
	h             *Host
	target        string
	before, after []interception
	next          func(*callFrame[In, Out], In) Out
	base          interception
	plain         bool
	fallback      func(In) Out
}

// callFrame holds only one invocation's original function and failure state.
type callFrame[In, Out any] struct {
	*callPlan[In, Out]
	original func(In) Out
	err      error
}

func (d *typedHook[In, Out]) rebuild(h *Host, target string, list []interception) {
	p := &callPlan[In, Out]{h: h, target: target, fallback: d.fallback}
	var wraps []interception
	for _, i := range list {
		if i.Active {
			switch i.Kind {
			case BeforeKind:
				p.before = append(p.before, i)
			case AfterKind:
				p.after = append(p.after, i)
			case AroundKind:
				wraps = append(wraps, i)
			default:
				p.base = i
			}
		}
	}
	p.next = func(f *callFrame[In, Out], in In) Out { return f.callBase(in) }
	p.plain = len(p.before) == 0 && len(p.after) == 0 && len(wraps) == 0 && p.base.ID == ""
	for n := len(wraps) - 1; n >= 0; n-- {
		i, next := wraps[n], p.next
		p.next = func(frame *callFrame[In, Out], in In) (out Out) {
			if !h.injectionLive(i) {
				return next(frame, in)
			}
			// An around that panics after calling next keeps next's result,
			// so the rest of the chain does not run twice.
			var called bool
			var inner Out
			defer func() {
				if v := recover(); v != nil {
					if _, ok := v.(hostPanic); ok {
						panic(v)
					}
					h.callFailure(i.Owner, "Around", target, i.ID, v)
					if called {
						out = inner
					} else {
						out = next(frame, in)
					}
				}
			}()
			return i.fn.(func(In, func(In) Out) Out)(in, func(in In) Out {
				called, inner = true, next(frame, in)
				return inner
			})
		}
	}
	d.plan = p
}

type hostPanic struct{ value any }

func (h *Host) injectionLive(i interception) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.active(i.Owner) && h.targetActive(i.Target)
}

func (p *callFrame[In, Out]) callBase(in In) (out Out) {
	i := p.base
	if i.ID != "" && p.h.injectionLive(i) {
		defer func() {
			if v := recover(); v != nil {
				stage := "Overwrite"
				if i.Kind == ReplaceKind {
					stage = "Replace"
				}
				p.h.callFailure(i.Owner, stage, p.target, i.ID, v)
				out = p.callOriginal(in)
			}
		}()
		if i.Kind == ReplaceKind {
			return i.call.(func(In) Out)(in)
		}
		return i.fn.(func(In) Out)(in)
	}
	return p.callOriginal(in)
}

func (p *callFrame[In, Out]) callOriginal(in In) (out Out) {
	o := owner(p.target)
	if o == HostOwner {
		defer func() {
			if v := recover(); v != nil {
				panic(hostPanic{v})
			}
		}()
		return p.original(in)
	}
	p.h.mu.RLock()
	live := p.h.targetActive(p.target)
	p.h.mu.RUnlock()
	if !live {
		p.err = ErrCallFailed
		return
	}
	defer func() {
		if v := recover(); v != nil {
			p.err = p.h.callFailure(o, "Original", p.target, "", v)
			var zero Out
			out = zero
		}
	}()
	if p.base.ID != "" && p.base.Kind == ReplaceKind && p.fallback != nil {
		return p.fallback(in)
	}
	return p.original(in)
}

func (p *callFrame[In, Out]) run(in In) (out Out, err error) {
	defer func() {
		p.original = nil
		if v := recover(); v != nil {
			if hp, ok := v.(hostPanic); ok {
				panic(hp.value)
			}
			panic(v)
		}
	}()
	for _, i := range p.before {
		if p.h.injectionLive(i) {
			in = p.callBefore(i, in)
		}
	}
	out = p.next(p, in)
	for _, i := range p.after {
		if p.h.injectionLive(i) {
			out = p.callAfter(i, in, out)
		}
	}
	if p.err != nil {
		var zero Out
		return zero, p.err
	}
	return out, nil
}

func (p *callFrame[In, Out]) callBefore(i interception, in In) (out In) {
	out = in
	defer func() {
		if v := recover(); v != nil {
			out = in
			p.h.callFailure(i.Owner, "Before", p.target, i.ID, v)
		}
	}()
	i.fn.(func(*In))(&out)
	return
}

func (p *callFrame[In, Out]) callAfter(i interception, in In, value Out) (out Out) {
	out = value
	defer func() {
		if v := recover(); v != nil {
			out = value
			p.h.callFailure(i.Owner, "After", p.target, i.ID, v)
		}
	}()
	i.fn.(func(In, *Out))(in, &out)
	return
}

// Call executes a prebuilt chain. With no active injections it allocates nothing.
// Extension panics return ErrCallFailed (errors.Is); host original panics escape.
// Calls are reentrant and concurrent; only their immutable plan is shared.
// Use Call for work other extensions may alter, and Guard for plain queries
// and callbacks that should not be injectable.
func Call[In, Out any](h *Host, key HookKey[In, Out], in In, original func(In) Out) (out Out, err error) {
	h.mu.RLock()
	d, exists := h.hooks[string(key)]
	if !exists {
		h.mu.RUnlock()
		return out, fmt.Errorf("unknown hook %q", key)
	}
	typed, ok := d.impl.(*typedHook[In, Out])
	if !ok {
		h.mu.RUnlock()
		return out, fmt.Errorf("hook %q has a different type", key)
	}
	p := typed.plan
	h.mu.RUnlock()
	if p.plain {
		return plainCall(h, string(key), in, original)
	}
	frame := callFrame[In, Out]{callPlan: p, original: original}
	return frame.run(in)
}

func plainCall[In, Out any](h *Host, key string, in In, original func(In) Out) (out Out, err error) {
	if owner(key) == HostOwner {
		return original(in), nil
	}
	h.mu.RLock()
	live := h.targetActive(key)
	h.mu.RUnlock()
	if !live {
		return out, ErrCallFailed
	}
	defer func() {
		if v := recover(); v != nil {
			err = h.callFailure(owner(key), "Original", key, "", v)
			var zero Out
			out = zero
		}
	}()
	return original(in), nil
}
