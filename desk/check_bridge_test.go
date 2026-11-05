package desk

import (
	"context"
	"writer/ext"
	checkmod "writer/ext/check"
	"writer/workspace"
)

var checkControllers = map[*App]*checkmod.Panel{}

func checkOf(a *App) *checkmod.Panel { return checkControllers[a] }

const (
	checkOnLeave  = ""
	checkOnDemand = "manual"
	checkOnPause  = "always"
)

func (a *App) loadChecks()                           { checkOf(a).Load() }
func (a *App) openCheck(id string)                   { checkOf(a).Open(id) }
func (a *App) checkItems(meant bool) []checkmod.Item { return checkOf(a).Items(meant) }
func (a *App) listedChecks() []checkmod.Item         { return checkOf(a).Listed() }
func (a *App) checkPending() (pendGroup, bool) {
	g, ok := checkOf(a).Pending()
	out := pendGroup{id: g.ID, name: g.Name, color: g.Color}
	for _, it := range g.Items {
		out.items = append(out.items, pendItem{key: it.Key, text: it.Text, sub: it.Sub, here: it.Here, open: it.Open})
	}
	return out, ok
}
func (a *App) checksOn() bool           { return a.extensions.Ledger("check").State == ext.On }
func (a *App) liveText() (string, bool) { return checkOf(a).LiveText() }

type noteAction struct {
	label string
	run   func()
}

func checkActionsWithoutNavigation() []int {
	p := new(checkmod.Panel)
	return make([]int, len(p.Actions(checkmod.Item{F: workspace.Finding{Kind: workspace.FindConflict}})))
}

type checkFunc func(context.Context, string) (workspace.Check, error)

func (f checkFunc) Check(c context.Context, s string) (workspace.Check, error) { return f(c, s) }
func checkRanges(a *App) []marginNote {
	var out []marginNote
	if !a.checksOn() {
		return nil
	}
	for _, n := range a.marginNotes() {
		if n.check != nil {
			out = append(out, n)
		}
	}
	return out
}
