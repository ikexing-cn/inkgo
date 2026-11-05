package lore

import "writer/workspace"

// Revise records a setting revision on the frame thread.
func (p *Panel) Revise(r workspace.Revision) (workspace.Revision, error) {
	return p.services.Book.Content().Revise(r)
}

// Refresh refreshes all setting consumers on the frame thread.
func (p *Panel) Refresh() error { return p.services.Edits.Refresh() }
