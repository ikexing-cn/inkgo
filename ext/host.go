package ext

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sync"
	"sync/atomic"
)

// Host owns immutable declarations and synchronized runtime snapshots. Mutating
// methods belong to the UI thread and must not be called from lifecycle hooks.
// Reads, references and preferences may also be used by session tasks.
type Host struct {
	opMu                 sync.Mutex
	mu                   sync.RWMutex
	hostEntry            *entry
	keySources           []keySource
	keysDirty            atomic.Bool
	targetNames          map[string]func() string
	hooks                map[string]hookDecl
	calls                map[string]callDeclaration
	injections           []interception
	injectionPreferences map[string]InjectionPreference
	chains               map[string][]ChainLink
	replacements         map[string]interception
	pending              map[*entry]bool
	hasPending           atomic.Bool
	storeMu              sync.Mutex
	opts                 Options
	changes              uint64
	closed               bool
	setup                bool
	entries              []*entry // assembly order, used for display
	loadOrder            []*entry // stable dependency order, used for lifecycle and registration
	byID                 map[string]*entry
	points               map[string]point
	contributions        []contribution
	providers            map[string]provider
	providerOrder        []string
	uses                 []serviceUse
	reads                []serviceUse
	services             map[string]serviceValue
	running              []*entry
	book                 Book
	off                  map[string]bool
	bookSwitch           map[string]bool
	bookMemory           map[Book]map[string]bool
	settingMemory        map[Book]map[string][]byte
}

var moduleID = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// New validates the entire graph, runs Setup in stable dependency order, reads local
// switches and starts eligible modules on the shelf. Each Setup runs once for
// this assembly; Open and switch changes never repeat it.
func New(opts Options, mods ...Module) (*Host, error) {
	h := &Host{opts: opts, setup: true, byID: map[string]*entry{}, points: map[string]point{}, providers: map[string]provider{}, services: map[string]serviceValue{}, off: map[string]bool{}, bookSwitch: map[string]bool{}, bookMemory: map[Book]map[string]bool{}, settingMemory: map[Book]map[string][]byte{}}
	h.hooks = map[string]hookDecl{}
	h.calls = map[string]callDeclaration{}
	h.injectionPreferences = map[string]InjectionPreference{}
	h.pending = map[*entry]bool{}
	if opts.DeclareHost != nil {
		e := &entry{mod: Module{ID: HostOwner, Setup: opts.DeclareHost}, ids: map[string]bool{}}
		h.hostEntry = e
		if err := h.register(e); err != nil {
			return nil, err
		}
	}
	for _, m := range mods {
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
	// Select the earliest assembled ready module at every step. The graph was
	// checked above, so a ready module always exists until every entry is loaded.
	loaded := map[string]bool{}
	for len(h.loadOrder) < len(h.entries) {
		for _, e := range h.entries {
			if loaded[e.mod.ID] {
				continue
			}
			ready := true
			for _, dep := range append(slices.Clone(e.mod.Requires), e.mod.Optional...) {
				ready = ready && loaded[dep]
			}
			if !ready {
				continue
			}
			e.loadIndex = len(h.loadOrder)
			h.loadOrder = append(h.loadOrder, e)
			loaded[e.mod.ID] = true
			break
		}
	}
	for _, e := range h.loadOrder {
		if err := h.register(e); err != nil {
			return nil, err
		}
	}
	// Registration requires dependency order; contribution presentation retains
	// assembly order, with each module's own registration order unchanged.
	assembly := map[string]int{}
	for i, e := range h.entries {
		assembly[e.mod.ID] = i + 1
	}
	slices.SortStableFunc(h.contributions, func(a, b contribution) int { return assembly[a.owner] - assembly[b.owner] })
	// Explicit chain preferences still override the default assembly order.
	slices.SortStableFunc(h.injections, func(a, b interception) int { return assembly[a.Owner] - assembly[b.Owner] })
	for key, p := range h.points {
		if !validPlace(p.place) {
			return nil, fmt.Errorf("point %s needs an explicit valid place (empty means invisible)", key)
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
	for _, u := range h.reads {
		p, ok := h.points[u.key]
		if !ok || p.typ != u.typ {
			return nil, fmt.Errorf("module %s: ReadPoint %q is missing or has a different type", u.owner, u.key)
		}
	}
	for _, u := range h.uses {
		p, ok := h.providers[u.key]
		if !ok || p.typ != u.typ {
			return nil, fmt.Errorf("module %s: Use service %q is missing or has a different type", u.owner, u.key)
		}
	}
	if err := h.validateInjections(); err != nil {
		return nil, err
	}
	if opts.Injections != nil {
		prefs, err := opts.Injections.Load()
		if err != nil {
			return nil, err
		}
		h.injectionPreferences = clonePreferences(prefs)
	}
	h.setup = false
	h.rebuildChains()
	if opts.LocalSwitches != nil {
		off, err := opts.LocalSwitches.Load()
		if err != nil {
			return nil, fmt.Errorf("load local switches: %w", err)
		}
		for _, id := range off {
			h.off[id] = true
		}
	}
	hs := newSession(HostOwner, Book{}, opts.LocalRoot)
	for _, key := range h.providerOrder {
		if h.providers[key].owner == HostOwner {
			var v serviceValue
			if err := protect(func() { v = h.providers[key].build(hs) }); err != nil {
				return nil, fmt.Errorf("host service %s: %w", key, err)
			}
			h.services[key] = v
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
	h.opMu.Lock()
	defer h.opMu.Unlock()
	h.drainFailures()
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
	h.opMu.Lock()
	defer h.opMu.Unlock()
	h.drainFailures()
	h.mu.RLock()
	same := h.book == book && !h.closed
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
	for len(h.running) > 0 {
		h.stop(h.running[len(h.running)-1])
	}
	h.mu.Lock()
	h.book, h.bookSwitch = book, switches
	h.closed = false
	for _, e := range h.entries {
		e.ledger.State = Off
	}
	h.mu.Unlock()
	h.reconcile()
	return nil
}

// Close cancels every module session and waits for its tasks without restarting
// modules on the shelf. Open can activate the same book again. The prior book
// identity stays until Open, so preference listeners can compare old values.
// Declarations and host services remain readable.
func (h *Host) Close() {
	h.opMu.Lock()
	defer h.opMu.Unlock()
	defer func() { h.mu.Lock(); h.closed = true; h.mu.Unlock() }()
	h.drainFailures()
	defer func() { h.mu.Lock(); h.rebuildChains(); h.mu.Unlock() }()
	for len(h.running) > 0 {
		h.stop(h.running[len(h.running)-1])
	}
}

func (h *Host) reconcile() {
	defer func() { h.mu.Lock(); h.rebuildChains(); h.mu.Unlock() }()
	wanted := map[string]bool{}
	h.mu.RLock()
	for _, e := range h.loadOrder {
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
	for _, e := range h.loadOrder {
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
	s.onCleanupError = func(err error) { h.recordFailure(e.mod.ID, Failure{Stage: "OnStop", Err: err}) }
	s.bookDir = h.opts.BookDir
	s.onPanic = func(v any) { h.failCall(e.mod.ID, s, "Task", "", "", "", v) }
	e.session = s
	e.ledger.State = Starting
	h.mu.Unlock()

	for _, key := range h.providerOrder {
		if h.providers[key].owner == e.mod.ID {
			var v serviceValue
			if err := protect(func() { v = h.providers[key].build(s) }); err != nil {
				h.startFailure(e, s, "Provide "+key, err)
				return
			}
			h.mu.Lock()
			h.services[key] = v
			h.mu.Unlock()
		}
	}
	for _, f := range e.starts {
		var err error
		if panicErr := protect(func() { err = f(s) }); panicErr != nil {
			err = panicErr
		}
		if err != nil {
			h.startFailure(e, s, "OnStart", err)
			return
		}
	}
	if h.opts.BookState != nil && e.state != nil && e.state.Load != nil {
		if err := protect(func() { e.state.Load(h.opts.BookState.LoadState(s.Book(), e.mod.ID)) }); err != nil {
			h.startFailure(e, s, "LoadState", err)
			return
		}
	}
	h.mu.Lock()
	if e.ledger.State != Starting {
		h.mu.Unlock()
		s.stop()
		for i := len(e.stops) - 1; i >= 0; i-- {
			h.stopCallback(e, s, e.stops[i])
		}
		h.mu.Lock()
		e.session = nil
		delete(h.pending, e)
		h.hasPending.Store(len(h.pending) != 0)
		h.mu.Unlock()
		return
	}
	e.ledger.State = On
	s.activated = true
	h.rebuildChains()
	h.mu.Unlock()
	at := slices.IndexFunc(h.running, func(other *entry) bool { return other.loadIndex > e.loadIndex })
	if at < 0 {
		at = len(h.running)
	}
	h.running = slices.Insert(h.running, at, e)
}
func (h *Host) removeServices(e *entry) {
	for _, key := range e.ledger.Provides {
		delete(h.services, key)
	}
}
func (h *Host) stop(e *entry) {
	h.saveState(e)
	h.mu.Lock()
	if e.ledger.State != Failed {
		e.ledger.State = Off
	}
	h.removeServices(e)
	h.rebuildChains()
	s := e.session
	e.session = nil
	h.mu.Unlock()
	s.stop()
	for i := len(e.stops) - 1; i >= 0; i-- {
		h.stopCallback(e, s, e.stops[i])
	}
	for i, candidate := range h.running {
		if candidate == e {
			h.running = append(h.running[:i], h.running[i+1:]...)
			break
		}
	}
}

// Items returns active contributions only while their point owner is active.
func Items[T any](h *Host, key PointKey[T]) []Item[T] {
	return pointItems(h, key, false)
}

// Declared returns all Setup contributions in assembly and registration order,
// even when the contributor or point owner is inactive. Replacements follow Items.
// For inactive contributions, read data and localization-only names declared
// with DeclareName. Do not invoke runtime callbacks, which may depend on Start.
// Reading does not change Changes.
func Declared[T any](h *Host, key PointKey[T]) []Item[T] {
	return pointItems(h, key, true)
}

func pointItems[T any](h *Host, key PointKey[T], declared bool) []Item[T] {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return pointItemsLocked(h, key, declared)
}

func pointItemsLocked[T any](h *Host, key PointKey[T], declared bool) []Item[T] {
	var out []Item[T]
	o := owner(string(key))
	if !declared && o != HostOwner && (h.byID[o] == nil || h.byID[o].ledger.State != On) {
		return out
	}
	for _, c := range h.contributions {
		if c.key == string(key) && (declared || c.owner == HostOwner || h.byID[c.owner].ledger.State == On) {
			if c.typ != reflect.TypeFor[T]() {
				panic(fmt.Sprintf("point %q requested with a different type", key))
			}
			value, replacedBy := c.value, ""
			if replacement, ok := h.replacements[itemTarget(c.key, c.id)]; ok {
				value = replacement.fn
				replacedBy = replacement.ID
			}
			var v T
			if value != nil {
				var ok bool
				v, ok = value.(T)
				if !ok {
					panic(fmt.Sprintf("point %q requested with a different type", key))
				}
			}
			out = append(out, Item[T]{ID: c.id, Owner: c.owner, Value: v, ReplacedBy: replacedBy})
		}
	}
	return out
}

// Ledger returns declaration and status snapshots; an unknown ID returns zero.
func (h *Host) Ledger(id string) Ledger {
	if h.keysDirty.Load() {
		h.mu.Lock()
		if h.keysDirty.Swap(false) {
			h.rebuildKeys()
		}
		h.mu.Unlock()
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	e := h.byID[id]
	if id == HostOwner {
		e = h.hostEntry
	}
	if e == nil {
		return Ledger{}
	}
	l := e.ledger
	l.Points = slices.Clone(l.Points)
	l.Reads = slices.Clone(l.Reads)
	l.Calls = slices.Clone(l.Calls)
	l.Contributions = slices.Clone(l.Contributions)
	l.Provides = slices.Clone(l.Provides)
	l.Uses = slices.Clone(l.Uses)
	l.Settings = slices.Clone(l.Settings)
	l.Injections = slices.Clone(l.Injections)
	l.Errors = slices.Clone(l.Errors)
	l.Keys = slices.Clone(l.Keys)
	for i := range l.Keys {
		l.Keys[i].Owners = slices.Clone(l.Keys[i].Owners)
	}
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

// Changes returns the revision of effective contributions, switches and call
// chains. It is allocation-free and safe to read from any goroutine.
func (h *Host) Changes() uint64 { h.mu.RLock(); defer h.mu.RUnlock(); return h.changes }

// Enabled reports active state and the current own switch without allocating a ledger snapshot.
func (h *Host) Enabled(id string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	e := h.byID[id]
	return e != nil && h.own(id) && e.ledger.State == On
}
