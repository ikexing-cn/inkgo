package ai

import "errors"

// ErrOff reports ai disabled.
var ErrOff = errors.New("ai: ai disabled")

// ErrChatModel reports chat model not selected.
var ErrChatModel = errors.New("ai: chat model not selected")
