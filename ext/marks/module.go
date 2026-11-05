// Package marks aggregates persistent prose decorations and margin notes.
package marks

import (
	"slices"
	"sort"

	"github.com/egoist/mygo/i18n"
	"writer/editor"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/ext/marks/marksapi"
	"writer/ext/marks/markstext"
)

// Module registers the prose marks prerequisite and its editor adapter.
var Module = ext.Module{ID: "marks", Kind: ext.Library, Name: markstext.ModMarks, Desc: markstext.ModMarksAbout, Setup: func(r *ext.Registrar) {
	ext.Declare(r, marksapi.MarginSources)
	ext.DeclareCalls(r, marksapi.MarginSources, sourceCall)
	reader := &library{sources: ext.ReadPoint(r, marksapi.MarginSources)}
	ext.Provide(r, marksapi.Service, func(*ext.Session) marksapi.Reader { return reader })
	ext.Contribute(r, hostapi.ProseMarks, "editor", hostapi.ProseMarkProvider{Revision: reader.Revision, Marks: func(chapter, text string) []editor.ProseMark {
		var out []editor.ProseMark
		for _, v := range reader.Items(chapter, text) {
			if v.Look != 0 && v.End > v.Start {
				out = append(out, editor.ProseMark{Kind: v.Look, Start: v.Start, End: v.End, Dropped: v.Dropped, Open: v.Open})
			}
		}
		slices.SortStableFunc(out, func(a, b editor.ProseMark) int { return priority(a.Kind) - priority(b.Kind) })
		return out
	}})
}}

func sourceCall(s marksapi.MarginSource) func(marksapi.Chapter) []marksapi.MarginItem {
	return func(a marksapi.Chapter) []marksapi.MarginItem {
		if s.Notes == nil {
			return nil
		}
		return s.Notes(a.Chapter, a.Text)
	}
}

type library struct {
	sources *ext.PointReader[marksapi.MarginSource]
	active  []ext.Item[marksapi.MarginSource]
	keys    []any
	changes uint64
	locale  string
	serial  uint64
	items   []marksapi.MarginItem
	chapter string
	text    string
	built   uint64
	builds  int
}

func (l *library) Items(chapter, text string) []marksapi.MarginItem {
	revision := l.Revision()
	if l.built == revision && l.chapter == chapter && l.text == text {
		return l.items
	}
	l.built, l.chapter, l.text = revision, chapter, text
	l.builds++
	var out []marksapi.MarginItem
	runes := []rune(text)
	starts := []int{0}
	for i, r := range runes {
		if r == '\n' {
			starts = append(starts, i+1)
		}
	}
	for _, item := range l.active {
		values, err := ext.CallReadPoint(l.sources, item.ID, marksapi.Chapter{Chapter: chapter, Text: text}, sourceCall(item.Value))
		if err != nil {
			continue
		}
		for _, v := range values {
			start := max(0, min(v.Start, len(runes)))
			v.Paragraph = sort.SearchInts(starts, start+1) - 1
			v.Character = start - starts[v.Paragraph]
			if v.Open != nil {
				f := v.Open
				v.Open = func() { _ = l.sources.Guard(item.Owner, v.ID, f) }
			}
			v.Actions = slices.Clone(v.Actions)
			for i, act := range v.Actions {
				f := act.Run
				v.Actions[i].Run = func() { _ = l.sources.Guard(item.Owner, v.ID, f) }
			}
			out = append(out, v)
		}
	}
	l.items = out
	return l.items
}
func priority(k editor.MarkKind) int {
	switch k {
	case editor.MarkForeshadow, editor.MarkPayoff:
		return 0
	case editor.MarkCheck:
		return 1
	default:
		return 2
	}
}

func (l *library) Revision() uint64 {
	changes, locale := l.sources.Revision(), i18n.Locale()
	dirty := l.serial == 0 || l.changes != changes || l.locale != locale
	if l.serial == 0 || l.changes != changes {
		l.active = l.sources.Items()
		l.keys = make([]any, len(l.active))
	}
	l.changes, l.locale = changes, locale
	for i, item := range l.active {
		if item.Value.Key == nil {
			continue
		}
		_ = l.sources.Guard(item.Owner, item.ID, func() {
			key := item.Value.Key()
			dirty = dirty || l.keys[i] != key
			l.keys[i] = key
		})
	}
	if dirty {
		l.serial++
	}
	return l.serial
}
