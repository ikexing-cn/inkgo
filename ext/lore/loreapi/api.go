package loreapi

import (
	"writer/ext"
	"writer/ext/context/contextapi"
	"writer/workspace"
)

// Revisions coordinates setting writes with derived views on the frame thread.
type Revisions interface {
	// Revise records an author revision in the setting history.
	Revise(workspace.Revision) (workspace.Revision, error)
	// Refresh reloads derived views after setting writes.
	Refresh() error
}

// RevisionsService resolves the setting extension's revision coordinator.
const RevisionsService ext.ServiceKey[Revisions] = "lore.revisions"

// References is the context library used by the setting extension, on the frame thread.
type References = contextapi.Context

// ReferencesService exposes lore's required context library to setting-dependent consumers.
const ReferencesService ext.ServiceKey[References] = "lore.references"
