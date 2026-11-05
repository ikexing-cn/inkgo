package aiapi

import (
	"writer/ext"
	"writer/workspace"
)

// ProposalUpdate is a change produced by a discussion worker, copied on the frame thread.
type ProposalUpdate struct {
	Call     string
	Proposal workspace.Proposal
	Writing  bool
	Model    string
}

// ProposalFrame is the discussion's frame-thread snapshot for edit suggestions.
// It contains no worker mailbox or frame objects.
type ProposalFrame struct {
	Active, Looking, Visible bool
	Session, Chapter         string
	Updates                  []ProposalUpdate
	Failed                   []string
	Stop                     func()
}

// Suggestions receives proposal lifecycle events on the frame thread.
type Suggestions interface {
	// Begin resets per-turn proposal deduplication on the frame thread.
	Begin()
	// Follow hands over a copied proposal snapshot on the frame thread.
	Follow(ProposalFrame)
	// Finish removes unfinished proposals on the frame thread.
	Finish(string)
	// Drop discards the book's undecided proposals when discussion stops; frame thread only.
	Drop()
	// State returns whether a proposal is pending and still being written; frame thread only.
	State(string, string) (bool, bool)
	// ShowProposal reveals a pending proposal in the prose on the frame thread.
	ShowProposal(string, string)
}

// Proposals is filled by propose and called by talk on the frame thread.
const Proposals ext.PointKey[Suggestions] = "ai.proposals"
