package desk

import (
	"errors"
	"slices"

	"writer/workspace"
)

// checkEdit is the one chapter result changed with a setting decision.
type checkEdit struct {
	node          string
	before, after workspace.Check
}

func (a *App) undoSetting(what string, ops ...workspace.Op) {
	st := a.Store
	var check *checkEdit
	for _, op := range ops {
		if op.Op != workspace.OpEditChange || op.Place != workspace.PlaceBody {
			continue
		}
		e, _ := a.book.Setting().Entry(op.Entry)
		for _, g := range e.Changes {
			if g.ID != op.Change {
				continue
			}
			before := a.book.Checks()[g.From]
			after := before
			after.Findings = slices.DeleteFunc(slices.Clone(before.Findings), func(f workspace.Finding) bool {
				return f.Kind == workspace.FindConflict && f.Entry == op.Entry
			})
			if len(before.Findings) != len(after.Findings) {
				check = &checkEdit{g.From, before, after}
			}
		}
	}
	rev, err := a.book.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: ops})
	if err != nil {
		a.warn(err)
		return
	}
	if check != nil {
		if err := a.book.SaveCheckOver(check.node, check.after, check.before); err != nil {
			a.warn(errors.Join(err, a.book.Unrevise(rev.ID)))
			a.warn(a.refreshFromSetting())
			return
		}
	}
	a.warn(a.refreshFromSetting())
	a.status.Did(what, func() error {
		if a.Store != st {
			return errors.New("作品已切换，无法撤销")
		}
		if check != nil {
			if err := a.book.CheckUnchanged(check.node, check.after); err != nil {
				return err
			}
		}
		if err := a.book.Unrevise(rev.ID); err != nil {
			return err
		}
		if check != nil {
			if err := a.book.SaveCheckOver(check.node, check.before, check.after); err != nil {
				// Keep the accepted state if the check cannot be restored.
				_, again := a.book.Revise(rev)
				return errors.Join(err, again)
			}
		}
		return a.refreshFromSetting()
	})
}
