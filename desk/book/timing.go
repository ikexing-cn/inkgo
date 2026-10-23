package book

import "time"

// SaveAfter is how long a pause lasts before the book is written. Tests
// shorten it.
var SaveAfter = 1500 * time.Millisecond

// CheckEvery is how often the book is read again for changes made outside.
// Tests shorten it.
var CheckEvery = 2 * time.Second
