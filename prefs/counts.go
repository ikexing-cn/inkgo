package prefs

import (
	"strconv"
	"strings"
)

// CountOrNone reads a count the author may leave empty, as a goal of words
// is: none when empty, or a whole number above nought; ok is false for
// anything else.
func CountOrNone(text string) (n int, ok bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, true
	}
	n, err := strconv.Atoi(text)
	return n, err == nil && n > 0
}
