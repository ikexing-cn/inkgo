package hostapi

import (
	"writer/ext"
)

// Search exposes prose search on the frame thread. Methods are direct host
// calls, without ext.Call or ext.Guard; host panics are not recovered.
type Search interface {
	// FindInChapter opens and locates prose on the frame thread, directly without ext.Call or ext.Guard.
	FindInChapter(string, string)
	// CountText counts live whole-book matches on the frame thread, directly without ext.Call or ext.Guard.
	CountText(string) int
}

// SearchService resolves direct frame-thread prose search.
const SearchService ext.ServiceKey[Search] = "host.search"

// Extensions reads host activation state on the frame thread.
type Extensions interface {
	// Name returns a registered extension’s localized name, even while disabled.
	Name(string) (string, bool)
	// Enabled reports whether a registered extension is active.
	Enabled(string) bool
}

// ExtensionsService resolves generic activation queries, including on the shelf.
const ExtensionsService ext.ServiceKey[Extensions] = "host.extensions"

// SelectionAction describes a prose menu item or submenu. Run executes on the
// frame thread through ext.Guard. Children are plain data, built with the menu.
type SelectionAction struct {
	Label string
	// Command names a contributed command whose shortcut the host displays; frame-thread metadata.
	Command  string
	Disabled bool
	Children []SelectionAction
	Run      func()
}

// SelectionMenu builds prose actions on the frame thread through ext.Call.
// Build must not retain frame objects; actions run through ext.Guard.
type SelectionMenu struct {
	// Order sorts contributions on the frame thread; negative values precede legacy host selection actions.
	// Equal values retain registration order.
	Order int
	// Group separates extension actions from the foreshadowing group.
	Group string
	Build func(string) []SelectionAction
}

// SelectionMenus collects prose actions between discussion and foreshadow actions.
const SelectionMenus ext.PointKey[SelectionMenu] = "host.selection-menu"

// BesideItems draws the right column beside non-writing pages on the frame
// thread through ext.Call. Each contribution must not retain frame objects.
// This drawing point is Go-only; a data-layer adapter must return UI descriptions.
const BesideItems ext.PointKey[DrawItem] = "host.beside"
