package book

import "time"

// SaveAfter and CheckEvery are production defaults, not shared test knobs.
const SaveAfter = 1500 * time.Millisecond

// CheckEvery is the foreground interval for checking changes made outside.
const CheckEvery = 2 * time.Second

// Timing belongs to one open book, shared by its writing and outline pages.
// Zero SaveAfter means immediate saving; it is not a request for a default.
type Timing struct {
	SaveAfter, CheckEvery time.Duration
}
