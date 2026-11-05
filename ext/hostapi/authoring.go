package hostapi

import (
	"writer/ext"
	"writer/workspace"
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

// AI exposes only the AI switch state. Calls are direct on the frame thread,
// without ext.Call or ext.Guard; host panics are not recovered.
type AI interface {
	// AIEnabled reports the AI switch state on the frame thread, directly without ext.Call or ext.Guard.
	AIEnabled() bool
}

// AIService resolves direct frame-thread AI switch state. The AI library will
// provide it under its own key when extracted in migration step 3.
const AIService ext.ServiceKey[AI] = "host.ai"

// Revisions coordinates content refresh and undo with other host features.
// Methods run directly on the frame thread, without ext.Call or ext.Guard.
type Revisions interface {
	// Refresh reloads derived content after a revision on the frame thread.
	Refresh() error
	// UndoRevision applies author operations with the host's undo coordination.
	UndoRevision(string, ...workspace.Op)
}

// RevisionsService resolves direct frame-thread revision coordination.
const RevisionsService ext.ServiceKey[Revisions] = "host.revisions"

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
