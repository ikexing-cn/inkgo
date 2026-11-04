package desk

import (
	"errors"
	"fmt"
	"strings"

	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/ext/hostapi"
	"writer/locales"
	"writer/shelf"
	"writer/workspace"
)

// ErrorTranslator formats a known error, using reason for its nested causes.
type ErrorTranslator = hostapi.ErrorTranslator

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
	case workspace.Error:
		switch cause {
		case workspace.ErrTitleEmpty:
			return locales.WorkspaceTitleEmpty()
		case workspace.ErrMoveIntoSelf:
			return locales.WorkspaceMoveIntoSelf()
		case workspace.ErrCheckChanged:
			return locales.WorkspaceCheckChanged()
		case workspace.ErrSettingChanged:
			return locales.WorkspaceSettingChanged()
		case workspace.ErrLegacySchema:
			return locales.WorkspaceLegacySchema()
		case workspace.ErrChangedOutside:
			return locales.WorkspaceChangedOutside()
		}
	case *workspace.TrashMissingError:
		return locales.WorkspaceTrashMissing(fmt.Sprintf("%q", cause.ID))
	case *shelf.NoBookError:
		return locales.ShelfNoBook(cause.Path)
	case *aiapi.SetupError:
		return a.errText(cause.Err)
	case openBookError:
		return locales.AppOpenBookFailed(cause.book, a.errText(cause.err))
	case extensionFailedError:
		return locales.ExtFailed(cause.name(), a.errText(cause.err))

	}
	if a.extensions != nil {
		for _, item := range ext.Items(a.extensions, hostapi.ErrorTexts) {
			var text string
			var ok bool
			if ext.Guard(a.extensions, item.Owner, item.ID, func() { text, ok = item.Value(err, a.errText) }) == nil && ok {
				return text
			}
		}
	}
	for _, translate := range a.ErrorText {
		if text, ok := translate(err, a.errText); ok {
			return text
		}
	}
	if inner := errors.Unwrap(err); inner != nil {
		return strings.Replace(err.Error(), inner.Error(), a.errText(inner), 1)
	}
	return err.Error()
}
