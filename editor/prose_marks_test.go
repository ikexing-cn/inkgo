package editor

import (
	"slices"
	"testing"
)

func TestOverlapOrderSelectionProposingFindHooksCheckAIWrittenNames(t *testing.T) {
	e := New(NewDoc("abcdefgh"), Options{})
	e.SetNameLinks([]NameLink{{"abcdefgh", "entry"}}, nil)
	e.SetProseMarks([]ProseMark{{Kind: MarkForeshadow, Start: 0, End: 8}, {Kind: MarkCheck, Start: 0, End: 8}, {Kind: MarkAI, Start: 0, End: 8}})
	if got := e.decorations(); len(got) != 1 || got[0].Kind != MarkForeshadow {
		t.Fatal(got)
	}
	if len(e.nameRanges()) != 0 {
		t.Fatal("names overlapped hooks")
	}
	e.SetProseMarks(e.proseMarks[1:])
	if e.decorations()[0].Kind != MarkCheck {
		t.Fatal("check order")
	}
	e.SetProseMarks(e.proseMarks[1:])
	if e.decorations()[0].Kind != MarkAI {
		t.Fatal("AI order")
	}
	e.SetProseMarks(nil)
	if len(e.nameRanges()) != 1 {
		t.Fatal("names absent")
	}
	for _, kind := range []string{"find", "proposing", "selection"} {
		e.SetProseMarks([]ProseMark{{Kind: MarkForeshadow, Start: 0, End: 8}})
		switch kind {
		case "find":
			e.SetFound([][2]int{{0, 8}}, 0)
		case "proposing":
			e.SetFound(nil, 0)
			e.props = []Proposal{{A: 0, B: 8}}
		case "selection":
			e.props = nil
			e.Select(0, 8)
		}
		if len(e.decorations()) != 0 || len(e.nameRanges()) != 0 {
			t.Fatal(kind)
		}
	}
}
func TestEmptyHostMarksNeverDrawStoredDocumentMarks(t *testing.T) {
	e := New(NewDoc("abc"), Options{})
	e.Doc.LoadMarks([]Mark{{Kind: MarkForeshadow, Start: 0, End: 2}})
	before := e.Doc.Marks()
	e.SetProseMarks(nil)
	if len(e.decorations()) != 0 || !slices.Equal(before, e.Doc.Marks()) {
		t.Fatal("display changed storage or retained marks")
	}
}
