package lore

import (
	"errors"
	"slices"

	"writer/locales"
	"writer/workspace"
)

// checkEdit is the one chapter result changed with a setting decision.
type checkEdit struct {
	node          string
	before, after workspace.Check
}

// UndoRevision applies a setting decision with reversible check updates.
func (a *Panel) UndoRevision(what string, ops ...workspace.Op) {
	st := a.services.Book.Store()
	var check *checkEdit
	for _, op := range ops {
		if op.Op != workspace.OpEditChange || op.Place != workspace.PlaceBody {
			continue
		}
		e, _ := a.services.Book.Content().Setting().Entry(op.Entry)
		for _, g := range e.Changes {
			if g.ID != op.Change {
				continue
			}
			before := a.services.Book.Content().Checks()[g.From]
			after := before
			after.Findings = slices.DeleteFunc(slices.Clone(before.Findings), func(f workspace.Finding) bool {
				return f.Kind == workspace.FindConflict && f.Entry == op.Entry
			})
			if len(before.Findings) != len(after.Findings) {
				check = &checkEdit{g.From, before, after}
			}
		}
	}
	rev, err := a.services.Book.Content().Revise(workspace.Revision{By: workspace.ByAuthor, Ops: ops})
	if err != nil {
		a.services.Notices.Notices().Warn(err)
		return
	}
	if check != nil {
		if err := a.services.Book.Content().SaveCheckOver(check.node, check.after, check.before); err != nil {
			a.services.Notices.Notices().Warn(errors.Join(err, a.services.Book.Content().Unrevise(rev.ID)))
			a.services.Notices.Notices().Warn(a.services.Editing.Refresh())
			return
		}
	}
	a.services.Notices.Notices().Warn(a.services.Editing.Refresh())
	a.services.Notices.Notices().Did(what, func() error {
		if a.services.Book.Store() != st {
			return errors.New(locales.StatusWorkSwitchedUndo())
		}
		if check != nil {
			if err := a.services.Book.Content().CheckUnchanged(check.node, check.after); err != nil {
				return err
			}
		}
		if err := a.services.Book.Content().Unrevise(rev.ID); err != nil {
			return err
		}
		if check != nil {
			if err := a.services.Book.Content().SaveCheckOver(check.node, check.before, check.after); err != nil {
				// Keep the accepted state if the check cannot be restored.
				_, again := a.services.Book.Content().Revise(rev)
				return errors.Join(err, again)
			}
		}
		return a.services.Editing.Refresh()
	})
}
