package lore

import "writer/workspace"

// Revise records a setting revision on the frame thread.
func (p *Panel) Revise(r workspace.Revision) (workspace.Revision, error) {
	return p.services.Book.Content().Revise(r)
}

// Refresh refreshes all setting consumers on the frame thread.
func (p *Panel) Refresh() error { return p.services.Editing.Refresh() }

func (p *Panel) flashNew() error {
	setting := p.services.Book.Content().Setting()
	for _, e := range setting.Entries {
		if e.Rev > p.seen && e.Rev <= len(setting.Revisions) && setting.Revisions[e.Rev-1].By == workspace.ByTalk {
			p.services.Editing.Flash(e.ID)
		}
	}
	p.seen = len(setting.Revisions)
	return nil
}
