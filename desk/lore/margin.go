package lore

import (
	"strings"
	"unicode/utf8"
	"writer/ext/hostapi"
	"writer/locales"
)

func (a *Panel) marginNotes(chapter, text string) []hostapi.MarginItem {
	var out []hostapi.MarginItem
	for _, it := range a.ideas() {
		if it.Change.ID == "" || it.Change.From != chapter {
			continue
		}
		start, end := 0, 0
		for _, name := range it.Entry.Names {
			at := strings.Index(text, name)
			if name != "" && at >= 0 {
				s := utf8.RuneCountInString(text[:at])
				e := s + utf8.RuneCountInString(name)
				if end == 0 || s < start {
					start, end = s, e
				}
			}
		}
		name := shortName(it.Entry)
		if len(it.Entry.Names) > 0 {
			name = it.Entry.Names[0]
		}
		n := hostapi.MarginItem{Idea: &it, ID: "idea-" + it.ID(), Kind: locales.NotesKindIdea(), Title: locales.NotesLoreChanged(name), Text: it.Change.Text, Start: start, End: end, Warn: true, Open: a.Waiting}
		for _, act := range a.changeActions(it) {
			n.Actions = append(n.Actions, hostapi.NoteAction{Label: act.label, Run: act.run})
		}
		out = append(out, n)
	}
	return out
}
