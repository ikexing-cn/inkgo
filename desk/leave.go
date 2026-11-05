package desk

import (
	"writer/ext"
	"writer/ext/hostapi"
)

// Leave asks extensions about undecided author work before leaving.
func (a *App) Leave(then func()) bool {
	for _, i := range ext.Items(a.extensions, hostapi.LeaveQuestions) {
		asked := false
		_ = ext.Guard(a.extensions, i.Owner, i.ID, func() { asked = i.Value(then) })
		if asked {
			return true
		}
	}
	return false
}
