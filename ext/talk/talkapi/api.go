package talkapi

import (
	"writer/ext"
)

// Service resolves the active conversation feature on the frame thread.
const Service ext.ServiceKey[ServiceAPI] = "talk.service"

// ServiceAPI contains conversation capabilities for required and optional consumers.
// All methods run on the frame thread.
type ServiceAPI interface {
	// Start creates a conversation through talk's declared Start operation.
	Start(StartTalk) bool
	// Open reveals a conversation beside the prose through the Open operation.
	Open(string)
	// DraftContext returns the fixed chapter context on the frame thread.
	DraftContext() (DraftContext, error)
	// Running reports an active reply on the frame thread.
	Running() bool
	// ProgressVisible reports whether the reply already shows its progress.
	ProgressVisible() bool
	// OpenTalk navigates to the saved conversation on the frame thread.
	OpenTalk(string)
	// TalkTitle returns its localized title on the frame thread.
	TalkTitle(string) (string, bool)
}

// ModelChoices holds talk's editable model selection on the frame thread.
type ModelChoices struct{ TalkProvider, TalkModel, TalkEffort string }
