package loreapi

import (
	"writer/ext"
	"writer/workspace"
)

// Revisions coordinates setting writes with derived views on the frame thread.
type Revisions interface {
	// UndoRevision applies a setting decision together with reversible check changes.
	UndoRevision(string, ...workspace.Op)
	// Revise records an author revision in the setting history.
	Revise(workspace.Revision) (workspace.Revision, error)
	// Refresh reloads derived views after setting writes.
	Refresh() error
}

// RevisionsService resolves the setting extension's revision coordinator.
const RevisionsService ext.ServiceKey[Revisions] = "lore.revisions"

// Selection accepts prose into the setting on the frame thread.
type Selection interface {
	// OpenSelection starts a form using the current editor selection.
	OpenSelection()
	// SelectionOpen reports whether that form is open.
	SelectionOpen() bool
}

// SelectionService resolves the setting's selection form.
const SelectionService ext.ServiceKey[Selection] = "lore.selection"
