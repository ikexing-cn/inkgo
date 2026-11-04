package talkapi

import (
	"github.com/egoist/mygo/ui"
	"writer/desk/kit"
	"writer/ext"
	"writer/ext/ai/aiapi"
	"writer/workspace"
)

// Service resolves the active conversation feature on the frame thread.
const Service ext.ServiceKey[ServiceAPI] = "talk.service"

// Markdown draws conversation text on the frame thread.
type Markdown interface {
	Draw(*ui.Context, string, kit.MdStyle) ui.Element
	Stream(*ui.Context, string, kit.MdStyle) ui.Element
	Digest(string) string
}

// ServiceAPI resolves conversation capabilities on the frame thread.
// Returned engines run on workers without retaining frame objects.
type ServiceAPI interface {
	Renderer() Markdown
	// Choices returns the conversation settings draft on the frame thread.
	Choices() *ModelChoices
	// SaveChoices persists complete conversation model choices on the frame thread.
	SaveChoices() error
	OpenTalk(string)
	TalkTitle(string) (string, bool)
	StartTalk(*ui.Context, workspace.Event) bool
	Build(*workspace.Store, aiapi.Material, []aiapi.Tool, *Beside, func(Progress), func(workspace.Event)) (Talk, error)
	Preloaded(*workspace.Store, aiapi.Material, string, string) (Preload, error)
	Extract(*workspace.Store, string, string, int, int, string) (workspace.Entry, error)
}

// ModelChoices holds talk's editable model selection on the frame thread.
type ModelChoices struct{ TalkProvider, TalkModel, TalkEffort string }
