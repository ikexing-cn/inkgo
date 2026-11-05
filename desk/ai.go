package desk

import "writer/workspace"

// modelLabel names the model of r and its effort: DeepSeek/deepseek-flash ·
// Default.
func (a *App) modelLabel(r workspace.AIRecord) string {
	return a.serviceModel(r.Model) + " · " + a.ai.EffortLabel(r.Effort)
}

// serviceModel names a model, written service ID/model, with the service by
// its name as the settings show it: DeepSeek/deepseek-flash.
func (a *App) serviceModel(name string) string {
	if ref, ok := workspace.ParseModelRef(name); ok {
		for _, p := range a.ai.Providers() {
			if p.ID == ref.Provider && p.Name != "" {
				ref.Provider = p.Name
				return ref.String()
			}
		}
	}
	return name
}
