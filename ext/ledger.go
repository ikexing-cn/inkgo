package ext

import (
	"path/filepath"
	"reflect"
	"slices"
)

// DefaultChoice explicitly keeps the original when several replacements compete.
const DefaultChoice = "host.default"

// ShortcutSource supplies immutable binding identities for non-macOS and macOS.
// Empty identities do not claim a key. Metadata must not call extension code.
type ShortcutSource interface{ ShortcutBindings() [2]string }

// KeyClaim records the host's winner for one platform binding.
type KeyClaim struct {
	ID, Binding, Winner, WinnerOwner string
	Mac                              bool
	Owners                           []string
}

// HostKey is one of the host's own commands and its bindings, as
// ShortcutSource gives them.
type HostKey struct {
	ID       string
	Bindings [2]string
}

// keySource is a contribution whose value can claim keys, with the target
// its replacements are filed under and its own bindings.
type keySource struct {
	c        contribution
	target   string
	bindings [2]string
}

// rebuildKeys requires mu held for writing.
func (h *Host) rebuildKeys() {
	if h.hostEntry != nil {
		h.hostEntry.ledger.Keys = nil
	}
	for _, e := range h.entries {
		e.ledger.Keys = nil
	}
	if h.keySources == nil {
		// Contributions are fixed once assembled, so the values that can claim
		// keys are found once. Replacements follow the chains, so they are
		// looked up on every rebuild.
		h.keySources = []keySource{}
		for _, c := range h.contributions {
			if source, ok := c.value.(ShortcutSource); ok {
				h.keySources = append(h.keySources, keySource{c, itemTarget(c.key, c.id), source.ShortcutBindings()})
			}
		}
	}
	type claimant struct {
		id, owner string
		bindings  [2]string
	}
	var on []claimant
	for _, k := range h.opts.HostKeys {
		on = append(on, claimant{k.ID, HostOwner, k.Bindings})
	}
	for _, s := range h.keySources {
		if !h.active(s.c.owner) || !h.active(owner(s.c.key)) {
			continue
		}
		b := s.bindings
		if replacement, ok := h.replacements[s.target]; ok {
			if r, ok := replacement.fn.(ShortcutSource); ok {
				b = r.ShortcutBindings()
			}
		}
		if b != [2]string{} {
			on = append(on, claimant{s.c.id, s.c.owner, b})
		}
	}
	installed := func(id string) bool { return id != HostOwner && h.byID[id].mod.Installed }
	for platform := range 2 {
		groups := map[string][]claimant{}
		var order []string
		for _, x := range on {
			key := x.bindings[platform]
			if key == "" {
				continue
			}
			if len(groups[key]) == 0 {
				order = append(order, key)
			}
			groups[key] = append(groups[key], x)
		}
		for _, binding := range order {
			list := groups[binding]
			// The host first, then bundled before installed, then the earlier.
			winner := list[0]
			for _, c := range list {
				if winner.owner != HostOwner && (c.owner == HostOwner || installed(winner.owner) && !installed(c.owner)) {
					winner = c
				}
			}
			owners := []string{}
			for _, c := range list {
				if !contains(owners, c.owner) {
					owners = append(owners, c.owner)
				}
			}
			for _, c := range list {
				e := h.hostEntry
				if c.owner != HostOwner {
					e = h.byID[c.owner]
				}
				e.ledger.Keys = append(e.ledger.Keys, KeyClaim{ID: c.id, Binding: binding, Winner: winner.id, WinnerOwner: winner.owner, Mac: platform == 1, Owners: owners})
			}
		}
	}
}

// NameTarget declares human-readable metadata for a target owned by the registrar.
// The callback is read only on the frame thread, including while disabled.
func NameTarget(r *Registrar, target string, name func() string) {
	r.owned(target, "NameTarget")
	if name == nil {
		r.fail("NameTarget needs a name")
	}
	if r.h.targetNames == nil {
		r.h.targetNames = map[string]func() string{}
	}
	r.h.targetNames[target] = name
}

// TargetName returns declared human-readable metadata, falling back to identity.
func (h *Host) TargetName(target string) string {
	h.mu.RLock()
	name := h.targetNames[target]
	h.mu.RUnlock()
	if name != nil {
		return name()
	}
	return target
}

// DataPaths resolves existing or prospective directories without creating them.
// Call on a worker: the application supplied BookDir resolver may perform IO.
func (h *Host) DataPaths(id string) (book, local string) {
	h.mu.RLock()
	b, opts := h.book, h.opts
	known := h.byID[id] != nil
	h.mu.RUnlock()
	if !known {
		return
	}
	if b.Dir != "" && opts.BookDir != nil {
		book, _ = opts.BookDir(b.Dir, id)
	}
	if opts.LocalRoot != "" {
		local = filepath.Join(opts.LocalRoot, "ext", id)
	}
	return
}

// DeclareName declares how a point value supplies its display name.
// Name functions are localization only, like Module.Name: they must be safe
// while disabled and must not access session state or services.
func DeclareName[T any](r *Registrar, key PointKey[T], get func(T) func() string) {
	r.owned(string(key), "DeclareName")
	p, ok := r.h.points[string(key)]
	if !ok || p.typ != reflect.TypeFor[T]() || get == nil {
		r.fail("DeclareName needs a matching declared point")
	}
	if p.named != nil {
		r.fail("duplicate DeclareName")
	}
	p.named = func(v any) func() string { return get(v.(T)) }
	r.h.points[string(key)] = p
}

// Place names where the author sees a point's contributions, in the words of
// the details page's modAdds message: "pane" (a pane tab), "page", "settings"
// (a settings page), "prefs", "dialog", "prose" (the prose's menus),
// "file", "edit", "view" (those menus), "marks", "pending", "keys", "tools",
// "footer", "export", "chead" (the line under a chapter's title), "beside",
// "names", "usage". Hidden is for points the author does not see.
type Place string

// Hidden marks a point the author does not see, such as state or layout.
const Hidden Place = ""

func validPlace(p Place) bool {
	return slices.Contains([]Place{Hidden, "pane", "page", "settings", "prefs", "dialog", "prose", "file", "edit", "view", "marks", "pending", "keys", "tools", "footer", "export", "chead", "beside", "names", "usage"}, p)
}

// ContributionDisplay reads place and localization metadata from declared values,
// including disabled contributions. Call on the frame thread after Setup.
func (h *Host) ContributionDisplay(id string) (place Place, name string) {
	h.mu.RLock()
	var get func(any) func() string
	var value any
	for _, c := range h.contributions {
		if c.id == id {
			p := h.points[c.key]
			place, get, value = p.place, p.named, c.value
			if replacement, ok := h.replacements[itemTarget(c.key, c.id)]; ok {
				value = replacement.fn
			}
			break
		}
	}
	h.mu.RUnlock()
	if get != nil {
		if title := get(value); title != nil {
			name = title()
		}
	}
	return
}

// KindWord returns the modInjected select word for an interception kind.
func (k InjectionKind) KindWord() string {
	switch k {
	case BeforeKind:
		return "before"
	case AfterKind:
		return "after"
	case AroundKind:
		return "around"
	default:
		return "overwrite"
	}
}
