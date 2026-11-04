package hostapi

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

// Suggestions is the temporary frame-thread bridge to edit suggestions in desk.
// It is removed when edit suggestions become an extension.
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

// SuggestionsService resolves the temporary edit-suggestion bridge on the frame thread.
const SuggestionsService ext.ServiceKey[Suggestions] = "host.suggestions"
