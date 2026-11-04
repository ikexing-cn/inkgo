package hostapi

import "writer/ext"

// WritingKey observes Escape and selection on the frame thread through ext.Call.
// Close is also called by host navigation; it must clear editor highlights.
type WritingKey struct {
	// Escape consumes Escape before the host clears the prose selection.
	Escape func() bool
	// HideSelection suppresses the prose selection toolbar while navigating matches.
	HideSelection func() bool
	// Close closes the bar on host navigation.
	Close func()
}

// WritingKeys collects WritingKeys callbacks on the frame thread.
const WritingKeys ext.PointKey[WritingKey] = "host.writing-keys"

// BeforeProse draws the find band above prose on the frame thread through ext.Call.
// This is Go-only; frame objects must not escape.
const BeforeProse ext.PointKey[DrawItem] = "host.before-prose"
