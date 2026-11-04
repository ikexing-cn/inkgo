package desk

import (
	"fmt"

	"writer/aiport"
	"writer/locales"
)

// ErrorTranslator formats a known error, using reason for its nested causes.
type ErrorTranslator func(err error, reason func(error) string) (string, bool)

// setupFailure is shared by AI configuration errors without importing their packages.
type setupFailure interface {
	error
	Setup()
}

func (a *App) errText(err error) string {
	if err == nil {
		return ""
	}
	switch cause := err.(type) {
	case *aiport.SetupError:
		return a.errText(cause.Err)
	case openBookError:
		return locales.AppOpenBookFailed(cause.book, a.errText(cause.err))
	case extensionFailedError:
		return locales.ExtFailed(cause.name(), a.errText(cause.err))
	case *modelRuleError:
		switch cause.rule {
		case "numbers":
			return locales.SettingsModelNumbers(cause.id)
		case "integer":
			return locales.SettingsModelAtInt(cause.id)
		case "output":
			return locales.SettingsModelOutMax(cause.id)
		case "threshold":
			return locales.SettingsModelAtRange(cause.id)
		}
	case *providerIDTakenError:
		return locales.SettingsIDTaken(cause.name)
	}
	for _, translate := range a.ErrorText {
		if text, ok := translate(err, a.errText); ok {
			return text
		}
	}
	return err.Error()
}

// modelRuleError is a local service-form constraint.
type modelRuleError struct{ id, rule string }

func (e *modelRuleError) Error() string {
	return fmt.Sprintf("desk: invalid %s for model %q", e.rule, e.id)
}

type providerIDTakenError struct{ name string }

func (e *providerIDTakenError) Error() string {
	return fmt.Sprintf("desk: provider id already used by %q", e.name)
}
