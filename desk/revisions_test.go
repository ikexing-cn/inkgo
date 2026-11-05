package desk

import "writer/workspace"

func (a *App) revise(ops ...workspace.Op) {
	_, err := a.book.Revise(workspace.Revision{By: workspace.ByAuthor, Ops: ops})
	if err == nil {
		err = a.refreshFromSetting()
	}
	a.SetError(err)
}
