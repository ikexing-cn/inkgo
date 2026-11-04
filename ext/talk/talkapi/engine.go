// Package talkapi defines conversation contracts for optional consumers.
package talkapi

import (
	"context"
	"writer/workspace"
)

// OutlineSource says where the outlines a talk carries before its chapter
// come from.
type OutlineSource int

const (
	// BookVolumes is the outlines of the book's volumes.
	BookVolumes OutlineSource = iota
	// VolumeChapters is the outlines of the chapters before this one in its volume.
	VolumeChapters
)

// Preload describes a talk's fixed context: the tokens it takes, the
// chapter's cards carried, whether the chapter before goes whole, and where
// the outlines come from, even when the outlines are all empty.
type Preload struct {
	Tokens, Cards int
	Previous      bool
	Outlines      OutlineSource
}

// Progress is a piece of a reply as it streams: of its reasoning or its
// text, or a change of the prose it proposes as far as it came, by the
// call that proposes it.
type Progress struct {
	Reasoning, Text string
	Call            string
	Proposal        *workspace.Proposal
}

// Talk is the discussion of one work.
type Talk interface {
	// Say saves the author's words, then asks for a reply. When that
	// fails, saved tells whether the words were kept, for Retry.
	Say(ctx context.Context, session string, words workspace.Event) (saved bool, err error)
	// Retry asks again for the reply to the author's words userID.
	Retry(ctx context.Context, session, userID string) error
}

// Beside is what the author is writing as they talk beside the prose: a
// chapter, or a volume when a folder is open.
type Beside struct {
	Title, Outline string // the chapter's, or the volume's
	Volume         string // the title of the volume a chapter lies in
	VolumeOutline  string
	Folder         bool
	Before         string // the chapter's text before the caret
}

// NoOutlines means no outline provider contributed fixed context.
const NoOutlines OutlineSource = -1
