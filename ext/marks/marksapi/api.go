// Package marksapi defines persistent prose marks and their margin notes.
package marksapi

import (
	"writer/editor"
	"writer/ext"
	"writer/workspace"
)

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
	// Paragraph and Character locate the start in the chapter, in runes.
	Paragraph, Character int
	// Look is zero for a note without a prose mark.
	Look    editor.MarkKind
	Dropped bool
	Warn    bool
	Open    func()
	Actions []NoteAction
}

// Chapter identifies the live prose supplied to a mark source.
type Chapter struct{ Chapter, Text string }

// MarginSource supplies marks and notes on the frame thread through ext.Call.
type MarginSource struct {
	// Key cheaply identifies all inputs other than chapter and text. Its result
	// must be comparable; a source may return its own version. Nil means Notes
	// reads only chapter and text. Key runs through the source owner's guard.
	Key   func() any
	Notes func(string, string) []MarginItem
}

// MarginSources collects contributed prose margin notes.
const MarginSources ext.PointKey[MarginSource] = "marks.margin-sources"

// Reader reads active sources on the frame thread, including guarded actions.
// Items returns borrowed, immutable results shared by the editor and notes.
type Reader interface {
	Items(chapter, text string) []MarginItem
	// Revision cheaply tracks source keys, availability and the display language.
	Revision() uint64
}

// Service resolves the active prose marks prerequisite on the frame thread.
const Service ext.ServiceKey[Reader] = "marks.service"
