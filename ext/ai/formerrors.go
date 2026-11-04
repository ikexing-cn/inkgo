package ai

import (
	"fmt"
	"writer/ext/ai/aitext"
)

// modelRuleError is a local service-form constraint.
type modelRuleError struct{ id, rule string }

// Error runs on the frame thread.
func (e *modelRuleError) Error() string {
	return fmt.Sprintf("desk: invalid %s for model %q", e.rule, e.id)
}

type providerIDTakenError struct{ name string }

// Error runs on the frame thread.
func (e *providerIDTakenError) Error() string {
	return fmt.Sprintf("desk: provider id already used by %q", e.name)
}

func formErrorText(err error, reason func(error) string) (string, bool) {
	switch cause := err.(type) {
	case *modelRuleError:
		switch cause.rule {
		case "numbers":
			return aitext.SettingsModelNumbers(cause.id), true
		case "integer":
			return aitext.SettingsModelAtInt(cause.id), true
		case "output":
			return aitext.SettingsModelOutMax(cause.id), true
		case "threshold":
			return aitext.SettingsModelAtRange(cause.id), true
		}
	case *providerIDTakenError:
		return aitext.SettingsIDTaken(cause.name), true
	}
	return "", false
}
