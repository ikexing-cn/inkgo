package desk

import (
	"strings"
	"writer/workspace"
)

type marginNote struct {
	idea                  *workspace.Idea
	id, kind, title, text string
	start, end            int
	warn                  bool
	check                 *bool
	fore                  string
	open                  func()
	actions               []noteAction
}

func (a *App) marginNotes() []marginNote {
	var out []marginNote
	if notesOf(a) == nil {
		return nil
	}
	for _, v := range notesOf(a).Notes() {
		n := marginNote{idea: v.Idea, id: v.ID, kind: v.Kind, title: v.Title, text: v.Text, start: v.Start, end: v.End, warn: v.Warn, open: v.Open}
		if strings.HasPrefix(v.ID, "check-") {
			flag := true
			n.check = &flag
		}
		if strings.HasPrefix(v.ID, "fore-") {
			n.fore = strings.TrimPrefix(v.ID, "fore-")
		}
		for _, act := range v.Actions {
			n.actions = append(n.actions, noteAction{act.Label, act.Run})
		}
		out = append(out, n)
	}
	return out
}
func (a *App) openMarginNote(n marginNote) {
	if n.open != nil {
		n.open()
	}
}
