package ext

import "encoding/json"

// BookState describes persistent preferences owned by a module.
// Load runs after initialization and before activation.
// Save runs before Stop only after successful activation.
type BookState struct {
	// Load restores this module's namespace before the activation becomes available.
	Load func(map[string]json.RawMessage)
	// Save snapshots preferences before activation state is destroyed.
	Save func() map[string]json.RawMessage
}

// BookStateStore preserves opaque JSON namespaces, including inactive modules.
type BookStateStore interface {
	// LoadState reads the named module's namespace on the frame thread.
	LoadState(Book, string) map[string]json.RawMessage
	// SaveState preserves other namespaces while saving on the frame thread.
	SaveState(Book, string, map[string]json.RawMessage)
}

// Persist registers book preferences independently of active contributions.
func (r *Registrar) Persist(state BookState) { r.check(); r.e.state = &state }

// SaveStates snapshots active preferences before the host saves its own view.
func (h *Host) SaveStates() {
	for _, e := range h.running {
		h.saveState(e)
	}
}
func (h *Host) saveState(e *entry) {
	if h.opts.BookState != nil && e.state != nil && e.state.Save != nil && e.session != nil && e.session.activated && e.session.Book().Dir != "" {
		if err := protect(func() { h.opts.BookState.SaveState(e.session.Book(), e.mod.ID, e.state.Save()) }); err != nil {
			h.recordFailure(e.mod.ID, Failure{Stage: "SaveState", Err: err})
		}
	}
}
