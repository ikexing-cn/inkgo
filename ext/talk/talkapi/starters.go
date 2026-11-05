package talkapi

import (
	"github.com/egoist/mygo/ui"
	"writer/ext"
	"writer/workspace"
)

// TalkStarter starts a chapter talk and exposes its frame-thread navigation.
// Callbacks borrow the current frame and must not retain it.
type TalkStarter struct {
	Start        func(ui.Services, workspace.Event) bool
	DraftContext func() (DraftContext, error)
	Open         func(string)
	// Running reports a conversation reply in flight.
	Running         func() bool
	ProgressVisible func() bool
}

// TalkStarters declares talk’s injectable operations. Talk contributes its
// controller and its Service dispatches Start and Open through this point.
const TalkStarters ext.PointKey[TalkStarter] = "talk.starters"

// DraftContext describes the fixed chapter context without constructing UI text.
type DraftContext struct {
	Tokens, Cards            int
	Previous, VolumeChapters bool
}
