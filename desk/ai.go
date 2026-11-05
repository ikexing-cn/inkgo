package desk

import (
	"writer/ext/ai/aiapi"
	"writer/workspace"
)

func (a *App) modelLabel(r workspace.AIRecord) string { return aiapi.RecordLabel(a.ai, r) }
func (a *App) serviceModel(name string) string        { return aiapi.ModelLabel(a.ai, name) }
