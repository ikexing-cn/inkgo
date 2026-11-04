package discussion

import (
	"errors"
	"fmt"
)

// ErrChapterGone reports chat chapter no longer exists.
var ErrChapterGone = errors.New("discussion: chat chapter no longer exists")

// ErrNoModelReason reports chat model not configured.
var ErrNoModelReason = errors.New("discussion: chat model not configured")

// ErrInputRequired reports chat input required.
var ErrInputRequired = errors.New("discussion: chat input required")

// ErrNewChatRequired reports draft and rewrite require a new chat.
var ErrNewChatRequired = errors.New("discussion: draft and rewrite require a new chat")

// ErrAlreadyReplied reports message already has a reply.
var ErrAlreadyReplied = errors.New("discussion: message already has a reply")

// ErrRetryMissing reports message to retry not found.
var ErrRetryMissing = errors.New("discussion: message to retry not found")

// ErrEmptyReply reports empty reply; original saved.
var ErrEmptyReply = errors.New("discussion: empty reply; original saved")

// ErrSelectionInvalid reports invalid source selection.
var ErrSelectionInvalid = errors.New("discussion: invalid source selection")

// ErrSelectionBlank reports selection contains only whitespace.
var ErrSelectionBlank = errors.New("discussion: selection contains only whitespace")

// ErrSourceMissing reports source message not found.
var ErrSourceMissing = errors.New("discussion: source message not found")

// ErrEmptyDigest reports empty summary.
var ErrEmptyDigest = errors.New("discussion: empty summary")

// CompactError wraps a failed compaction while preserving the original chat.
type CompactError struct{ Err error }

func (e *CompactError) Error() string {
	return fmt.Sprintf("discussion: cannot compact chat; original saved: %v", e.Err)
}

func (e *CompactError) Unwrap() error { return e.Err }

// ReplyError wraps a failed reply while preserving the original chat.
type ReplyError struct{ Err error }

func (e *ReplyError) Error() string {
	return fmt.Sprintf("discussion: chat failed; original saved: %v", e.Err)
}

func (e *ReplyError) Unwrap() error { return e.Err }
