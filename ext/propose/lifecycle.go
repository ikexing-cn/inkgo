package propose

import (
	"writer/ext/ai/aiapi"
	"writer/workspace"
)

// Begin resets the edit-suggestion deduplication on the frame thread.
func (h *View) Begin() { h.seen = map[string]bool{} }

// Follow receives a copied discussion frame, without its mailbox.
func (h *View) Follow(f aiapi.ProposalFrame) { h.Frame = f; h.receiveCompleted(); h.followProps() }

// Finish drops unfinished text from this conversation on the frame thread.
func (h *View) Finish(session string) {
	h.Frame.Session = session
	h.unfinishedProps()
	h.Frame.Active = false
}

// Drop discards the book's undecided proposals on the frame thread.
func (h *View) Drop() {
	h.dropProps(h.Props, workspace.FateClosed)
	h.Frame.Active = false
}

// State queries a pending edit suggestion on the frame thread.
func (h *View) State(session, call string) (bool, bool) {
	p := h.propOf(session, call)
	if p == nil {
		return false, false
	}
	return true, p.Writing
}

// ShowProposal brings a pending edit suggestion into view on the frame thread.
func (h *View) ShowProposal(session, call string) {
	if p := h.propOf(session, call); p != nil {
		h.showProp(p)
	}
}
