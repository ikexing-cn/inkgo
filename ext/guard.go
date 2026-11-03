package ext

// Guard runs a plain callback while its owner is active, without declaring a
// hook or adding a Chain. Extension panics and inactive owners return
// ErrCallFailed (errors.Is); panic errors are also preserved for errors.Is.
// Host-owned panics escape. Calls are reentrant and concurrent.
// Use Call for work other extensions may alter, and Guard for plain queries
// and callbacks that should not be injectable.
func Guard(h *Host, owner, target string, fn func()) (err error) {
	if owner == HostOwner {
		fn()
		return nil
	}
	h.mu.RLock()
	live := h.byID[owner] != nil && h.active(owner)
	h.mu.RUnlock()
	if !live {
		return ErrCallFailed
	}
	defer func() {
		if v := recover(); v != nil {
			err = h.failCall(owner, "Guard", target, "", "", v)
		}
	}()
	fn()
	return nil
}
