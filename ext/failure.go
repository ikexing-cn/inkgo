package ext

import (
	"errors"
	"fmt"
)

func panicError(v any) error {
	if err, ok := v.(error); ok {
		return err
	}
	return fmt.Errorf("%v", v)
}

func protect(f func()) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = panicError(v)
		}
	}()
	f()
	return nil
}

func (h *Host) recordFailure(id string, f Failure) {
	f.Module = id
	h.mu.Lock()
	h.byID[id].ledger.Errors = append(h.byID[id].ledger.Errors, f)
	h.mu.Unlock()
	if h.opts.OnFailure != nil {
		h.opts.OnFailure(id, f)
	}
}

func (h *Host) startFailure(e *entry, s *Session, stage string, err error) {
	h.mu.Lock()
	e.ledger.State = Failed
	h.removeServices(e)
	h.mu.Unlock()
	h.recordFailure(e.mod.ID, Failure{Stage: stage, Err: err})
	s.stop()
	for i := len(e.stops) - 1; i >= 0; i-- {
		h.stopCallback(e, s, e.stops[i])
	}
	h.mu.Lock()
	e.session = nil
	delete(h.pending, e)
	h.mu.Unlock()
}

func (h *Host) stopCallback(e *entry, s *Session, f func(*Session)) {
	if err := protect(func() { f(s) }); err != nil {
		h.recordFailure(e.mod.ID, Failure{Stage: "OnStop", Err: err})
	}
}

// callFailure invalidates effects immediately, but defers joining tasks and
// OnStop until the next mutation (Set/Open/SetOrder/Choose/Close), before that
// mutation changes registrations. A session task can therefore fail its own
// call without waiting for itself. No cleanup callback runs with mu held.
func (h *Host) callFailure(id, stage, target, injection string, v any) error {
	return h.failCall(id, stage, target, injection, target, v)
}

func (h *Host) failCall(id, stage, target, injection, chain string, v any) error {
	err := panicError(v)
	f := Failure{Module: id, Stage: stage, Target: target, Injection: injection, Chain: chain, Err: err}
	h.mu.Lock()
	e := h.byID[id]
	e.ledger.State = Failed
	e.ledger.Errors = append(e.ledger.Errors, f)
	blocked := map[string]bool{id: true}
	for _, other := range h.loadOrder {
		for _, dep := range other.mod.Requires {
			if blocked[dep] {
				blocked[other.mod.ID] = true
			}
		}
		if blocked[other.mod.ID] {
			if other != e && other.ledger.State != Failed {
				other.ledger.State = Off
			}
			h.removeServices(other)
			if other.session != nil {
				h.pending[other] = true
				other.session.cancel()
			}
		}
	}
	h.rebuildChains()
	h.mu.Unlock()
	if h.opts.OnFailure != nil {
		h.opts.OnFailure(id, f)
	}
	return errors.Join(ErrCallFailed, err)
}

// drainFailures requires opMu and runs before registration mutations.
func (h *Host) drainFailures() {
	for n := len(h.running) - 1; n >= 0; n-- {
		e := h.running[n]
		h.mu.Lock()
		pending := h.pending[e]
		delete(h.pending, e)
		h.mu.Unlock()
		if pending {
			h.stop(e)
		}
	}
}

// Own reports the own switch independently of required dependencies. Unknown
// IDs return false. Libraries always use their local switch.
func (h *Host) Own(id string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.byID[id] != nil && h.own(id)
}

// BookOverride reports an explicit switch for this book. Libraries, unknown IDs
// and the shelf have no book override.
func (h *Host) BookOverride(id string) (on, ok bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	e := h.byID[id]
	if e == nil || e.mod.Kind == Library || h.book.Dir == "" {
		return false, false
	}
	on, ok = h.bookSwitch[id]
	return
}

// Book returns the current book; a zero Book represents the shelf.
func (h *Host) Book() Book { h.mu.RLock(); defer h.mu.RUnlock(); return h.book }
