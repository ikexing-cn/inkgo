package writing

import (
	"errors"
	"fmt"
)

// ErrNoModelReason reports writing model not selected.
var ErrNoModelReason = errors.New("writing: writing model not selected")

// ErrChapterMissing reports chapter not found.
var ErrChapterMissing = errors.New("writing: chapter not found")

// ErrNoSetting reports lore is empty.
var ErrNoSetting = errors.New("writing: lore is empty")

// ShortChapterError records the minimum character count for a check.
type ShortChapterError struct{ Count int }

func (e *ShortChapterError) Error() string {
	return fmt.Sprintf("writing: chapter has fewer than %d characters", e.Count)
}

// CheckResultError wraps unreadable check results.
type CheckResultError struct{ Err error }

func (e *CheckResultError) Error() string {
	return fmt.Sprintf("writing: cannot read check results: %v", e.Err)
}

func (e *CheckResultError) Unwrap() error { return e.Err }
