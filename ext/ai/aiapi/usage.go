package aiapi

import (
	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/workspace"
)

// Usage draws contributed usage sections on the frame thread.
const Usage ext.PointKey[func(*ui.Context)] = "ai.usage"

// Call describes one finished model call, without request or response text.
type Call struct {
	Store  *workspace.Store
	Record workspace.Record
}

// Calls observes every finished model call on its worker, through ext.Guard.
const Calls ext.PointKey[func(Call)] = "ai.calls"

// ModelChanges observes provider configuration changes on the frame thread.
const ModelChanges ext.PointKey[func()] = "ai.model-changes"
