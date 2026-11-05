package desk

import "writer/workspace"

func (a *App) ideas() []workspace.Idea { return lorePanel(a).Ideas() }
