package aiapi

import (
	"writer/workspace"
)

// ModelLabel formats a model reference using configured provider names.
func ModelLabel(backend Backend, name string) string {
	if ref, ok := workspace.ParseModelRef(name); ok {
		for _, p := range backend.Providers() {
			if p.ID == ref.Provider && p.Name != "" {
				ref.Provider = p.Name
				return ref.String()
			}
		}
	}
	return name
}

// RecordLabel formats historical model metadata in the current interface language.
func RecordLabel(backend Backend, r workspace.AIRecord) string {
	return ModelLabel(backend, r.Model) + " · " + backend.EffortLabel(r.Effort)
}
