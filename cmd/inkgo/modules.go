package main

import (
	"writer/ext"
	aimod "writer/ext/ai"
	"writer/ext/backup"
	"writer/ext/cats"
	"writer/ext/check"
	"writer/ext/context"
	"writer/ext/export"
	"writer/ext/find"
	"writer/ext/focus"
	"writer/ext/goal"
	"writer/ext/hooks"
	"writer/ext/lore"
	"writer/ext/marks"
	"writer/ext/names"
	"writer/ext/notes"
	"writer/ext/outline"
	"writer/ext/phone"
	"writer/ext/snaps"
	"writer/ext/talk"
	"writer/ext/trash"
)

var backupModule, _ = backup.New(true)

var modules = []ext.Module{focus.Module, goal.Module, names.Module, find.Module, notes.Module, lore.Module, outline.Module, hooks.Module, talk.Module, check.Module, snaps.Module, trash.Module, cats.Module, backupModule, export.Module, phone.Module, context.Module, marks.Module}

// assembledModules passes machine parameters to the AI prerequisite.
func assembledModules(configFile, trace string) []ext.Module {
	return append(append([]ext.Module(nil), modules...), aimod.New(aimod.Options{ConfigFile: configFile, Trace: trace}))
}
