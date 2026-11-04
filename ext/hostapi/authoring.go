package hostapi

import (
	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/workspace"
)

// Talk exposes conversation navigation on the frame thread. Methods are direct
// host calls, without ext.Call or ext.Guard; host panics are not recovered.
type Talk interface {
	// StopTalk cancels the conversation worker during extension shutdown on the frame thread.
	StopTalk()
	// ShowEntry opens a digest reference on the frame thread (temporary UI bridge).
	ShowEntry(workspace.Entry)
	// StartTalk opens a conversation with an origin and first words on the frame thread.
	StartTalk(*ui.Context, workspace.Event) bool
	// OpenTalk opens a conversation on the frame thread, directly without ext.Call or ext.Guard.
	OpenTalk(string)
	// TalkTitle returns a conversation title on the frame thread, directly without ext.Call or ext.Guard.
	TalkTitle(string) (string, bool)
}

// TalkService resolves direct frame-thread conversation access. Its key moves
// with discussion when discussion becomes an extension in migration step 3.
const TalkService ext.ServiceKey[Talk] = "host.talk"

// Search exposes prose search on the frame thread. Methods are direct host
// calls, without ext.Call or ext.Guard; host panics are not recovered.
type Search interface {
	// FindInChapter opens and locates prose on the frame thread, directly without ext.Call or ext.Guard.
	FindInChapter(string, string)
	// CountText counts live whole-book matches on the frame thread, directly without ext.Call or ext.Guard.
	CountText(string) int
	// ReplaceText opens whole-book replacement on the frame thread, directly without ext.Call or ext.Guard.
	ReplaceText(string, string)
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
	Label    string
	Disabled bool
	Children []SelectionAction
	Run      func()
}

// SelectionMenu builds prose actions on the frame thread through ext.Call.
// Build must not retain frame objects; actions run through ext.Guard.
type SelectionMenu struct {
	Build func(string) []SelectionAction
}

// SelectionMenus collects prose actions between discussion and foreshadow actions.
const SelectionMenus ext.PointKey[SelectionMenu] = "host.selection-menu"

// BesideItems draws the right column beside non-writing pages on the frame
// thread through ext.Call. Each contribution must not retain frame objects.
// This drawing point is Go-only; a data-layer adapter must return UI descriptions.
const BesideItems ext.PointKey[DrawItem] = "host.beside"

// NoteAction is a frame-thread action on a contributed margin note.
// Run is invoked through ext.Guard by the host.
type NoteAction struct {
	Label string
	Run   func()
}

// MarginItem describes a note attached to a prose range; all callbacks run
// through ext.Guard on the frame thread. It contains no frame objects.
type MarginItem struct {
	// Idea optionally identifies an entry change for diagnostics and inspection.
	Idea                  *workspace.Idea
	ID, Kind, Title, Text string
	Start, End            int
	Warn                  bool
	Open                  func()
	Actions               []NoteAction
}

// MarginSource supplies notes on the frame thread through ext.Guard.
type MarginSource struct {
	Notes func(string, string) []MarginItem
}

// MarginSources collects contributed prose margin notes.
const MarginSources ext.PointKey[MarginSource] = "host.margin-sources"
