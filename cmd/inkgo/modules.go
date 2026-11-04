package main

import (
	"writer/ext"
	aimod "writer/ext/ai"
	"writer/ext/backup"
	"writer/ext/cats"
	"writer/ext/context"
	"writer/ext/export"
	"writer/ext/focus"
	"writer/ext/goal"
	"writer/ext/lore"
	"writer/ext/outline"
	"writer/ext/phone"
	"writer/ext/talk"
)

var backupModule, _ = backup.New(true)

var modules = []ext.Module{context.Module, talk.Module, lore.Module, outline.Module, backupModule, export.Module, phone.Module, focus.Module, goal.Module, cats.Module}

// assembledModules passes machine parameters to the AI prerequisite.
func assembledModules(configFile, trace string) []ext.Module {
	return append([]ext.Module{aimod.New(aimod.Options{ConfigFile: configFile, Trace: trace})}, modules...)
}
