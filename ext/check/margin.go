package check

import (
	"strings"
	"unicode/utf8"
	"writer/editor"
	"writer/ext/marks/marksapi"

	"writer/ext/check/checktext"
	"writer/workspace"
)

func (a *Panel) checkMarginSource(chapter, text string) []marksapi.MarginItem {
	if !a.checksOn() {
		return nil
	}
	var out []marksapi.MarginItem
	for _, it := range a.checkItems(false) {
		if it.Node != chapter || it.F.Quote == "" {
			continue
		}
		at := strings.Index(text, it.F.Quote)
		if at < 0 {
			continue
		}
		start := utf8.RuneCountInString(text[:at])
		n := marksapi.MarginItem{ID: "check-" + it.F.ID, Kind: kindLabel(it.F.Kind), Title: it.F.Title, Start: start, End: start + utf8.RuneCountInString(it.F.Quote), Warn: true, Look: editor.MarkCheck, Open: func() { a.State.Tab = 0; a.openCheck(it.F.ID) }}
		switch it.F.Kind {
		case workspace.FindConflict:
			n.Kind, n.Text = checktext.NotesKindInconsistent(), checktext.NotesSays(it.F.Says)
		case workspace.FindNew:
			n.Text = it.F.Text
		}
		for _, act := range a.checkActions(it) {
			n.Actions = append(n.Actions, marksapi.NoteAction{Label: act.label, Run: act.run})
		}
		out = append(out, n)
	}
	return out
}
