package ext

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sync"
)

// Host owns immutable declarations and synchronized runtime snapshots. Mutating
// methods belong to the UI thread and must not be called from lifecycle hooks.
// Reads, references and preferences may also be used by session tasks.
type Host struct {
	mu            sync.RWMutex
	storeMu       sync.Mutex
	opts          Options
	setup         bool
	entries       []*entry
	byID          map[string]*entry
	points        map[string]point
	contributions []contribution
	providers     map[string]provider
	providerOrder []string
	uses          []serviceUse
	services      map[string]serviceValue
	running       []*entry
	book          Book
	off           map[string]bool
	bookSwitch    map[string]bool
	bookMemory    map[Book]map[string]bool
	settingMemory map[Book]map[string]any
	prefs         []preference
}

var moduleID = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// New validates the entire graph, runs Setup in assembly order, reads local
// switches and starts eligible modules on the shelf. Each Setup runs once for
// this assembly; Open and switch changes never repeat it.
func New(opts Options, mods ...Module) (*Host, error) {
	h := &Host{opts: opts, setup: true, byID: map[string]*entry{}, points: map[string]point{}, providers: map[string]provider{}, services: map[string]serviceValue{}, off: map[string]bool{}, bookSwitch: map[string]bool{}, bookMemory: map[Book]map[string]bool{}, settingMemory: map[Book]map[string]any{}}
	positions := map[string]int{}
	for i, m := range mods {
		if !moduleID.MatchString(m.ID) || m.ID == HostOwner {
			return nil, fmt.Errorf("module %q: invalid or reserved ID", m.ID)
		}
		if _, ok := h.byID[m.ID]; ok {
			return nil, fmt.Errorf("module %s: duplicate ID", m.ID)
		}
		if m.Kind != Feature && m.Kind != Library {
			return nil, fmt.Errorf("module %s: invalid Kind", m.ID)
		}
		m = cloneModule(m)
		e := &entry{mod: m, ids: map[string]bool{}, ledger: Ledger{ID: m.ID}}
		h.entries = append(h.entries, e)
		h.byID[m.ID] = e
		positions[m.ID] = i
	}
	for _, e := range h.entries {
		for _, id := range append(slices.Clone(e.mod.Requires), e.mod.Optional...) {
			if h.byID[id] == nil {
				return nil, fmt.Errorf("module %s: dependency %q does not exist", e.mod.ID, id)
			}
		}
	}
	visited := map[string]uint8{}
	var visit func(string) error
	visit = func(id string) error {
		if visited[id] == 1 {
			return fmt.Errorf("module %s: dependency cycle", id)
		}
		if visited[id] == 2 {
			return nil
		}
		visited[id] = 1
		m := h.byID[id].mod
		for _, dep := range append(slices.Clone(m.Requires), m.Optional...) {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visited[id] = 2
		return nil
	}
	for _, e := range h.entries {
		if err := visit(e.mod.ID); err != nil {
			return nil, err
		}
	}
	for i, e := range h.entries {
		for _, dep := range append(slices.Clone(e.mod.Requires), e.mod.Optional...) {
			if positions[dep] >= i {
				return nil, fmt.Errorf("module %s: dependency %s must be assembled before it", e.mod.ID, dep)
			}
		}
		if err := h.register(e); err != nil {
			return nil, err
		}
	}
	pointTypes := map[string]reflect.Type{}
	for _, c := range h.contributions {
		p, ok := h.points[c.key]
		if owner(c.key) != HostOwner && !ok {
			return nil, fmt.Errorf("module %s: Contribute point %q is undeclared", c.owner, c.key)
		}
		if ok && p.typ != c.typ {
			return nil, fmt.Errorf("module %s: Contribute point %q has a different type", c.owner, c.key)
		}
		if typ, exists := pointTypes[c.key]; exists && typ != c.typ {
			return nil, fmt.Errorf("module %s: Contribute point %q has a different type", c.owner, c.key)
		}
		pointTypes[c.key] = c.typ
	}
	for _, u := range h.uses {
		p, ok := h.providers[u.key]
		if !ok || p.typ != u.typ {
			return nil, fmt.Errorf("module %s: Use service %q is missing or has a different type", u.owner, u.key)
		}
	}
	h.setup = false
	if opts.LocalSwitches != nil {
		off, err := opts.LocalSwitches.Load()
		if err != nil {
			return nil, fmt.Errorf("load local switches: %w", err)
		}
		for _, id := range off {
			h.off[id] = true
		}
	}
	h.reconcile()
	return h, nil
}

func cloneModule(m Module) Module {
	m.Requires = slices.Clone(m.Requires)
	m.Optional = slices.Clone(m.Optional)
	return m
}
func (h *Host) register(e *entry) (err error) {
	r := &Registrar{h: h, e: e}
	defer func() {
		r.closed = true
		if p := recover(); p != nil {
			err = fmt.Errorf("module %s: Setup: %v", e.mod.ID, p)
		}
	}()
	if e.mod.Setup != nil {
		e.mod.Setup(r)
	}
	return nil
}
func (h *Host) own(id string) bool {
	if h.book.Dir != "" && h.byID[id].mod.Kind != Library {
		if on, ok := h.bookSwitch[id]; ok {
			return on
		}
	}
	return !h.off[id]
}

// Modules returns independent descriptors in assembly order.
func (h *Host) Modules() []Module {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Module, len(h.entries))
	for i, e := range h.entries {
		out[i] = cloneModule(e.mod)
	}
	return out
}

// Starts lists required modules whose own switches are off, excluding id itself.
// Traversal matches the prototype: direct dependencies before their ancestors.
func (h *Host) Starts(id string) []string { h.mu.RLock(); defer h.mu.RUnlock(); return h.starts(id) }
func (h *Host) starts(id string) []string {
	var out []string
	seen := map[string]bool{}
	var walk func(string)
	walk = func(id string) {
		e := h.byID[id]
		if e == nil {
			return
		}
		for _, dep := range e.mod.Requires {
			if seen[dep] {
				continue
			}
			seen[dep] = true
			if !h.own(dep) {
				out = append(out, dep)
			}
			walk(dep)
		}
	}
	walk(id)
	return out
}

// Stops lists currently active required consumers, excluding id itself.
func (h *Host) Stops(id string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []string
	seen := map[string]bool{}
	var walk func(string)
	walk = func(id string) {
		for _, e := range h.entries {
			if !seen[e.mod.ID] && e.ledger.State == On && contains(e.mod.Requires, id) {
				seen[e.mod.ID] = true
				out = append(out, e.mod.ID)
				walk(e.mod.ID)
			}
		}
	}
	walk(id)
	return out
}

// Set changes local switches, including disabled required ancestors when enabling.
// Existing book overrides still take precedence.
func (h *Host) Set(id string, on bool) error { return h.set(id, on, false, false) }

// SetBook changes this book only; required libraries are enabled locally.
func (h *Host) SetBook(id string, on bool) error { return h.set(id, on, true, false) }

// ClearBook removes the current book override and follows the local switch.
func (h *Host) ClearBook(id string) error { return h.set(id, false, true, true) }

func cloneSwitches(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func (h *Host) set(id string, on, book, clear bool) error {
	h.mu.RLock()
	e := h.byID[id]
	if e == nil {
		h.mu.RUnlock()
		return fmt.Errorf("module %q: unknown ID", id)
	}
	if book && (h.book.Dir == "" || e.mod.Kind == Library) {
		h.mu.RUnlock()
		return fmt.Errorf("module %s: book switch requires an open book and a Feature", id)
	}
	local, overrides, b := cloneSwitches(h.off), cloneSwitches(h.bookSwitch), h.book
	ids := []string{id}
	if on && !clear {
		ids = append(h.starts(id), id)
	}
	h.mu.RUnlock()
	localChanged, bookChanged := false, false
	for _, target := range ids {
		if book && h.byID[target].mod.Kind != Library {
			if clear {
				if _, ok := overrides[target]; ok {
					delete(overrides, target)
					bookChanged = true
				}
			} else if v, ok := overrides[target]; !ok || v != on {
				overrides[target] = on
				bookChanged = true
			}
		} else if local[target] == on {
			if on {
				delete(local, target)
			} else {
				local[target] = true
			}
			localChanged = true
		}
	}
	h.storeMu.Lock()
	if localChanged && h.opts.LocalSwitches != nil {
		off := make([]string, 0, len(local))
		for k := range local {
			off = append(off, k)
		}
		slices.Sort(off)
		if err := h.opts.LocalSwitches.Save(off); err != nil {
			h.storeMu.Unlock()
			return fmt.Errorf("module %s: save local switches: %w", id, err)
		}
	}
	if bookChanged && h.opts.BookSwitches != nil {
		if err := h.opts.BookSwitches.Save(b, cloneSwitches(overrides)); err != nil {
			// A successful local save remains live even if the separate book store fails.
			h.storeMu.Unlock()
			if localChanged {
				h.mu.Lock()
				h.off = local
				h.mu.Unlock()
				h.reconcile()
			}
			return fmt.Errorf("module %s: save book switches: %w", id, err)
		}
	}
	h.storeMu.Unlock()
	h.mu.Lock()
	h.off, h.bookSwitch = local, overrides
	if bookChanged {
		h.bookMemory[b] = cloneSwitches(overrides)
	}
	h.mu.Unlock()
	h.reconcile()
	return nil
}

// Open stops all running sessions before starting the new book's effective set.
// Storage is loaded first so a read error leaves the old book running.
func (h *Host) Open(book Book) error {
	h.mu.RLock()
	same := h.book == book
	h.mu.RUnlock()
	if same {
		return nil
	}
	h.storeMu.Lock()
	switches := map[string]bool{}
	if book.Dir != "" {
		if h.opts.BookSwitches != nil {
			loaded, err := h.opts.BookSwitches.Load(book)
			if err != nil {
				h.storeMu.Unlock()
				return fmt.Errorf("load book switches: %w", err)
			}
			switches = cloneSwitches(loaded)
		} else {
			switches = cloneSwitches(h.bookMemory[book])
		}
	}
	h.storeMu.Unlock()
	old := make([]any, len(h.prefs))
	for i, p := range h.prefs {
		old[i] = p.current()
	}
	for len(h.running) > 0 {
		h.stop(h.running[len(h.running)-1])
	}
	h.mu.Lock()
	h.book, h.bookSwitch = book, switches
	for _, e := range h.entries {
		e.ledger.State = Off
	}
	h.mu.Unlock()
	h.reconcile()
	for i, p := range h.prefs {
		p.changed(old[i])
	}
	return nil
}

// Close cancels every session and waits for its tasks. Declarations remain readable.
func (h *Host) Close() {
	for len(h.running) > 0 {
		h.stop(h.running[len(h.running)-1])
	}
}

func (h *Host) reconcile() {
	wanted := map[string]bool{}
	h.mu.RLock()
	for _, e := range h.entries {
		on := h.own(e.mod.ID)
		for _, dep := range e.mod.Requires {
			on = on && wanted[dep]
		}
		wanted[e.mod.ID] = on
	}
	h.mu.RUnlock()
	for i := len(h.running) - 1; i >= 0; i-- {
		e := h.running[i]
		if !wanted[e.mod.ID] {
			h.stop(e)
		}
	}
	for _, e := range h.entries {
		h.mu.Lock()
		if !wanted[e.mod.ID] {
			e.ledger.State = Off
			h.mu.Unlock()
			continue
		}
		if e.ledger.State == On || e.ledger.State == Failed {
			h.mu.Unlock()
			continue
		}
		ready := true
		for _, id := range e.mod.Requires {
			if h.byID[id].ledger.State != On {
				ready = false
			}
		}
		h.mu.Unlock()
		if ready {
			h.start(e)
		}
	}
}
func (h *Host) start(e *entry) {
	h.mu.Lock()
	s := newSession(e.mod.ID, h.book, h.opts.LocalRoot)
	e.session = s
	e.ledger.State = Starting
	h.mu.Unlock()
	for _, key := range h.providerOrder {
		if h.providers[key].owner == e.mod.ID {
			v := h.providers[key].build(s)
			h.mu.Lock()
			h.services[key] = v
			h.mu.Unlock()
		}
	}
	for _, f := range e.starts {
		if err := f(s); err != nil {
			h.mu.Lock()
			e.ledger.State = Failed
			e.ledger.Errors = append(e.ledger.Errors, Failure{"OnStart", err})
			h.removeServices(e)
			h.mu.Unlock()
			s.stop()
			for i := len(e.stops) - 1; i >= 0; i-- {
				e.stops[i](s)
			}
			e.session = nil
			return
		}
	}
	h.mu.Lock()
	e.ledger.State = On
	h.mu.Unlock()
	h.running = append(h.running, e)
}
func (h *Host) removeServices(e *entry) {
	for _, key := range e.ledger.Provides {
		delete(h.services, key)
	}
}
func (h *Host) stop(e *entry) {
	h.mu.Lock()
	e.ledger.State = Off
	h.removeServices(e)
	h.mu.Unlock()
	s := e.session
	s.stop()
	for i := len(e.stops) - 1; i >= 0; i-- {
		e.stops[i](s)
	}
	e.session = nil
	for i, candidate := range h.running {
		if candidate == e {
			h.running = append(h.running[:i], h.running[i+1:]...)
			break
		}
	}
}

// Items returns active contributions only while their point owner is active.
func Items[T any](h *Host, key PointKey[T]) []Item[T] {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []Item[T]
	o := owner(string(key))
	if o != HostOwner && (h.byID[o] == nil || h.byID[o].ledger.State != On) {
		return out
	}
	for _, c := range h.contributions {
		if c.key == string(key) && h.byID[c.owner].ledger.State == On {
			if c.typ != reflect.TypeFor[T]() {
				panic(fmt.Sprintf("point %q requested with a different type", key))
			}
			var v T
			if c.value != nil {
				var ok bool
				v, ok = c.value.(T)
				if !ok {
					panic(fmt.Sprintf("point %q requested with a different type", key))
				}
			}
			out = append(out, Item[T]{c.id, c.owner, v})
		}
	}
	return out
}

// Ledger returns declaration and status snapshots; an unknown ID returns zero.
func (h *Host) Ledger(id string) Ledger {
	h.mu.RLock()
	defer h.mu.RUnlock()
	e := h.byID[id]
	if e == nil {
		return Ledger{}
	}
	l := e.ledger
	l.Points = slices.Clone(l.Points)
	l.Contributions = slices.Clone(l.Contributions)
	l.Provides = slices.Clone(l.Provides)
	l.Uses = slices.Clone(l.Uses)
	l.Settings = slices.Clone(l.Settings)
	l.Injections = slices.Clone(l.Injections)
	l.Errors = slices.Clone(l.Errors)
	for _, dep := range e.mod.Requires {
		l.Requires = append(l.Requires, Dependency{dep, h.byID[dep].ledger.State == On})
	}
	for _, dep := range e.mod.Optional {
		l.Optional = append(l.Optional, Dependency{dep, h.byID[dep].ledger.State == On})
	}
	for _, other := range h.entries {
		if contains(other.mod.Requires, id) {
			l.NeededBy = append(l.NeededBy, other.mod.ID)
		}
		if contains(other.mod.Optional, id) {
			l.UsedOptionallyBy = append(l.UsedOptionallyBy, other.mod.ID)
		}
	}
	return l
}
