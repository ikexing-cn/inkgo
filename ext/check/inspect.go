package check

import (
	"github.com/egoist/mygo/ui"
	"writer/ext/hostapi"
)

// Items returns the cached findings for frame-thread diagnostics.
func (a *Panel) Items(meant bool) []Item { return a.checkItems(meant) }

// Listed returns the findings grouped by new name on the frame thread.
func (a *Panel) Listed() []Item { return a.listedChecks() }

// Load refreshes stored checks on the frame thread.
func (a *Panel) Load() { a.loadChecks() }

// Open selects a finding's page on the frame thread.
func (a *Panel) Open(id string) { a.openCheck(id) }

// Pending returns the pending findings on the frame thread.
func (a *Panel) Pending() (hostapi.PendingGroup, bool) { return a.checkPending() }

// LiveText returns cached live prose on the frame thread.
func (a *Panel) LiveText() (string, bool) { return a.liveText() }

// Start begins a copied check request for runner diagnostics.
func (a *Panel) Start(p chapterChecker, chapter string, s ui.Services) {
	a.State.Runner.start(p, a.store(), chapter, s)
}

// Actions returns the available actions for a finding on the frame thread.
func (a *Panel) Actions(it Item) []noteAction { return a.checkActions(it) }
