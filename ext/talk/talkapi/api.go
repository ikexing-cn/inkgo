package talkapi

import (
	"writer/ext"
)

// Service resolves the active conversation feature on the frame thread.
const Service ext.ServiceKey[ServiceAPI] = "talk.service"

// ServiceAPI contains the conversation capabilities used by optional consumers.
// All methods run on the frame thread.
type ServiceAPI interface {
	// OpenTalk navigates to the saved conversation on the frame thread.
	OpenTalk(string)
	// TalkTitle returns its localized title on the frame thread.
	TalkTitle(string) (string, bool)
}

// ModelChoices holds talk's editable model selection on the frame thread.
type ModelChoices struct{ TalkProvider, TalkModel, TalkEffort string }
