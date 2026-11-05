package ext

import (
	"encoding/json"
	"reflect"
)

type preference interface {
	current() any
	changed(any)
}

// Pref reads persisted values on demand. Listener registration is synchronized;
// listeners run on the thread calling Set or Open, without host locks held.
type Pref[T any] struct {
	h         *Host
	id        string
	schema    Schema[T]
	listeners []func(T)
}

// Setting records a schema under the module's namespace. BookScope requires an
// open book for Set. Both writes the book override, or local storage on the shelf.
func Setting[T any](r *Registrar, name string, schema Schema[T]) *Pref[T] {
	r.check()
	if name == "" {
		r.fail("Setting needs a name")
	}
	if schema.Scope > Both {
		r.fail("Setting %q has an invalid scope", name)
	}
	id := r.e.mod.ID + "." + name
	r.claim(id)
	schema.Choices = append([]Choice[T](nil), schema.Choices...)
	p := &Pref[T]{h: r.h, id: id, schema: schema}
	r.h.prefs = append(r.h.prefs, p)
	r.e.ledger.Settings = append(r.e.ledger.Settings, SettingInfo{ID: id, Title: schema.Title, Desc: schema.Desc, Scope: schema.Scope, Group: schema.Group, Order: schema.Order, Schema: schema, Read: func() any { return p.Get() }, Write: func(v any) { p.Set(v.(T)) }})
	return p
}
func (p *Pref[T]) book() Book {
	p.h.mu.RLock()
	defer p.h.mu.RUnlock()
	if p.h.setup {
		panic("preference access during Setup is forbidden")
	}
	return p.h.book
}
func (h *Host) loadSetting(b Book, id string) ([]byte, bool) {
	if h.opts.Settings != nil {
		return h.opts.Settings.Load(b, id)
	}
	v, ok := h.settingMemory[b][id]
	return v, ok
}
func (p *Pref[T]) get(b Book) T {
	if p.schema.Scope == BookScope && b.Dir == "" {
		return p.schema.Default
	}
	if p.schema.Scope == Local {
		b = Book{}
	}
	if v, ok := p.h.loadSetting(b, p.id); ok {
		var value T
		if json.Unmarshal(v, &value) == nil {
			return value
		}
	}
	if p.schema.Scope == Both && b.Dir != "" {
		if v, ok := p.h.loadSetting(Book{}, p.id); ok {
			var value T
			if json.Unmarshal(v, &value) == nil {
				return value
			}
		}
	}
	return p.schema.Default
}

// Get reads the current scope, falling back to the schema default. Values with
// invalid JSON or an incompatible JSON type are ignored. It panics during Setup.
func (p *Pref[T]) Get() T {
	b := p.book()
	p.h.storeMu.Lock()
	defer p.h.storeMu.Unlock()
	return p.get(b)
}

// Set persists the value and notifies listeners if the effective value changed.
// The JSON bytes are independent of the caller's mutable value. Values that
// cannot be JSON encoded panic before storage changes.
func (p *Pref[T]) Set(value T) {
	b := p.book()
	if p.schema.Scope == BookScope && b.Dir == "" {
		panic("book preference Set requires an open book")
	}
	if p.schema.Scope == Local {
		b = Book{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	p.h.storeMu.Lock()
	old := p.get(b)
	if p.h.opts.Settings != nil {
		p.h.opts.Settings.Save(b, p.id, data)
	} else {
		if p.h.settingMemory[b] == nil {
			p.h.settingMemory[b] = map[string][]byte{}
		}
		p.h.settingMemory[b][p.id] = data
	}
	p.h.storeMu.Unlock()
	if !reflect.DeepEqual(old, value) {
		p.notify(value)
	}
}

// OnChange adds a listener for Set and effective value changes on Open.
func (p *Pref[T]) OnChange(f func(T)) {
	p.h.mu.Lock()
	p.listeners = append(p.listeners, f)
	p.h.mu.Unlock()
}
func (p *Pref[T]) notify(v T) {
	p.h.mu.RLock()
	listeners := append([]func(T){}, p.listeners...)
	p.h.mu.RUnlock()
	for _, f := range listeners {
		if err := protect(func() { f(v) }); err != nil {
			p.h.callFailure(owner(p.id), "OnChange", p.id, "", err)
			return
		}
	}
}
func (p *Pref[T]) current() any { return p.Get() }
func (p *Pref[T]) changed(old any) {
	if v := p.Get(); !reflect.DeepEqual(old, v) {
		p.notify(v)
	}
}
