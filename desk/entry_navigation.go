package desk

import (
	"writer/ext"
	"writer/ext/hostapi"
	"writer/workspace"
)

func (a *App) entryNavigation(run func(hostapi.EntryNavigation)) bool {
	if a.extensions == nil {
		return false
	}
	service, ok := ext.Lookup(a.extensions, hostapi.EntryNavigationService)
	if !ok || service.Navigation == nil {
		return false
	}
	return ext.Guard(a.extensions, service.Owner, "entry-navigation", func() { run(service.Navigation) }) == nil
}

func (a *App) showEntry(e workspace.Entry) {
	a.entryNavigation(func(n hostapi.EntryNavigation) { n.Open(e) })
}

func (a *App) seeName(id string) { a.entryNavigation(func(n hostapi.EntryNavigation) { n.Jump(id) }) }

func (a *App) entryNavigationAvailable() bool {
	if a.extensions == nil {
		return false
	}
	service, ok := ext.Lookup(a.extensions, hostapi.EntryNavigationService)
	if !ok || service.Navigation == nil {
		return false
	}
	return ext.Guard(a.extensions, service.Owner, "entry-navigation.available", func() {}) == nil
}
