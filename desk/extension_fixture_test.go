package desk

import (
	"writer/ext"
	aimod "writer/ext/ai"
	"writer/ext/ai/aiapi"
	"writer/ext/backup"
)

var controllers = map[*App]*backup.View{}

func backupOf(a *App) *backup.View { return controllers[a] }

func enableBackupQuestion(a *App) {
	a.extensions.Close()
	mod, view := backup.New(true)
	a.initExtensions([]ext.Module{aimod.New(aimod.Options{ConfigFile: a.configFile, View: aiViewOf(a), Factory: func() aiapi.Backend { return a.ai }}), mod})
	controllers[a] = view
}
