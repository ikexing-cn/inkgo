package desk

import (
	"writer/ext"
	aimod "writer/ext/ai"
	"writer/ext/ai/aiapi"
	"writer/ext/backup"
	contextmod "writer/ext/context"
	"writer/ext/talk"
)

var controllers = map[*App]*backup.View{}

func backupOf(a *App) *backup.View { return controllers[a] }

func enableBackupQuestion(a *App) {
	a.extensions.Close()
	mod, view := backup.New(true)
	a.initExtensions([]ext.Module{aimod.New(aimod.Options{ConfigFile: a.configFile, View: aiViewOf(a), Factory: func() aiapi.Backend { return a.ai }}), contextmod.Module, talk.Module, mod})
	controllers[a] = view
}
