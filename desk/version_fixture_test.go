package desk

import (
	findmod "writer/ext/find"
	"writer/ext/snaps"
	"writer/ext/trash"
)

var snapControllers = map[*App]*snaps.View{}
var trashControllers = map[*App]*trash.View{}
var findControllers = map[*App]*findmod.View{}

func snapsOf(a *App) *snaps.View  { return snapControllers[a] }
func trashOf(a *App) *trash.View  { return trashControllers[a] }
func findOf(a *App) *findmod.View { return findControllers[a] }
func (a *App) openFind() {
	for _, r := range a.extensionCommands() {
		if r.id == "find.find" {
			r.cmd.run(a)
		}
	}
}
func (a *App) openSnapshots() {
	for _, r := range a.extensionCommands() {
		if r.id == "snaps.open" {
			r.cmd.run(a)
		}
	}
}
func (a *App) openTrash() {
	for _, r := range a.extensionCommands() {
		if r.id == "trash.open" {
			r.cmd.run(a)
		}
	}
}
