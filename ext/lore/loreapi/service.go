// Package loreapi contains stateless entry navigation contracts.
package loreapi

import (
	"writer/ext"
	"writer/workspace"
)

// EntryNavigation is an optional frame-thread entry UI service. Host callers
// use ext.Guard; absence means the corresponding action is unavailable.
type EntryNavigation interface {
	// Open shows an entry, or pending entries when it is not settled.
	Open(workspace.Entry)
	// Reveal shows an entry over the current route without discarding its layers.
	Reveal(string)
	// Jump shows a particular entry directly.
	Jump(string)
	// Edit opens an entry form.
	Edit(workspace.Entry)
	// Waiting opens pending entries.
	Waiting()
	// Answer opens the form answering an entry's question.
	Answer(workspace.Entry)
	// CloseWaiting closes the pending list when AI is disabled.
	CloseWaiting()
}

// EntryNavigator identifies the optional service's owner for ext.Guard.
// Navigation methods run on the frame thread; host-owned panics escape.
type EntryNavigator struct {
	Owner      string
	Navigation EntryNavigation
}

// EntryNavigationService identifies the optional entry UI. Calls require ext.Guard.
const EntryNavigationService ext.ServiceKey[EntryNavigator] = "lore.entry-navigation"

// Active is present while the setting extension is active.
const Active ext.ServiceKey[struct{}] = "lore.active"
