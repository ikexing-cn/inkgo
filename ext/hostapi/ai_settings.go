package hostapi

import (
	"context"
	"github.com/egoist/mygo/ui"
	"writer/ext"
)

// ModelChoices holds editable model choices on the frame thread.
type ModelChoices struct{ TalkProvider, TalkModel, TalkEffort, PenProvider, PenModel, PenEffort string }

// SettingsHost bridges features still in desk until they become extensions.
// All methods run on the frame thread; Run captures Services in its caller.
type SettingsHost interface {
	Choices() *ModelChoices
	Held() bool
	Busy() bool
	ModelsChanged()
	ErrorText(error) string
	Run(*ui.Context, string, func(context.Context) (func(), error))
	WritingSettings(*ui.Context)
}

// SettingsHostService resolves the temporary settings bridge.
const SettingsHostService ext.ServiceKey[SettingsHost] = "host.ai-settings"

// AIUsage draws the usage-record entry on the frame thread.
const AIUsage ext.PointKey[func(*ui.Context)] = "host.ai-usage"
