package desk

import (
	"writer/ext/hostapi"
	"writer/workspace"
)

// Begin resets the edit-suggestion deduplication on the frame thread.
func (h extensionHost) Begin() { h.a.write.seen = map[string]bool{} }

// Follow receives a copied discussion frame, without its mailbox.
func (h extensionHost) Follow(f hostapi.ProposalFrame) { h.a.proposalFrame = f; h.a.followProps() }

// Finish drops unfinished text from this conversation on the frame thread.
func (h extensionHost) Finish(session string) {
	h.a.proposalFrame.Session = session
	h.a.unfinishedProps()
	h.a.proposalFrame.Active = false
}

// Drop discards the book's undecided proposals on the frame thread.
func (h extensionHost) Drop() {
	h.a.dropProps(h.a.write.props, workspace.FateClosed)
	h.a.proposalFrame.Active = false
}

// State queries a pending edit suggestion on the frame thread.
func (h extensionHost) State(session, call string) (bool, bool) {
	p := h.a.write.propOf(session, call)
	if p == nil {
		return false, false
	}
	return true, p.writing
}

// ShowProposal brings a pending edit suggestion into view on the frame thread.
func (h extensionHost) ShowProposal(session, call string) {
	if p := h.a.write.propOf(session, call); p != nil {
		h.a.showProp(p)
	}
}
