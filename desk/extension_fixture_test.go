package desk

import (
	"writer/ext"
	"writer/ext/backup"
)

var controllers = map[*App]*backup.View{}

func backupOf(a *App) *backup.View { return controllers[a] }

func enableBackupQuestion(a *App) {
	a.extensions.Close()
	mod, view := backup.New(true)
	a.initExtensions([]ext.Module{mod})
	controllers[a] = view
}
