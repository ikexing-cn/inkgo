package talkapi

import (
	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/workspace"
)

// Service resolves the active conversation feature on the frame thread.
const Service ext.ServiceKey[ServiceAPI] = "talk.service"

// ServiceAPI contains the conversation capabilities used by optional consumers.
// All methods run on the frame thread; StartTalk dispatches its worker internally.
type ServiceAPI interface {
	// OpenTalk navigates to the saved conversation on the frame thread.
	OpenTalk(string)
	// TalkTitle returns its localized title on the frame thread.
	TalkTitle(string) (string, bool)
	// StartTalk opens a chapter conversation with the author's first words on the frame thread.
	StartTalk(*ui.Context, workspace.Event) bool
	// Preloaded describes the fixed context before drafting on the frame thread.
	Preloaded(*workspace.Store, aiapi.Material, string, string) (Preload, error)
	// ProgressVisible reports whether the running reply displays its own progress on the frame thread.
	ProgressVisible() bool
}

// ModelChoices holds talk's editable model selection on the frame thread.
type ModelChoices struct{ TalkProvider, TalkModel, TalkEffort string }
