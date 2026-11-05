package editor

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNameLinksYieldToAnnotations(t *testing.T) {
	e := New(NewDoc("林晚 林晚 林晚 林晚 林晚 林晚"), Options{})
	e.SetNameLinks([]NameLink{{"林晚", "entry"}}, func(string) {})
	e.Doc.LoadMarks([]Mark{{Kind: MarkForeshadow, Start: 0, End: 2}, {Kind: MarkAI, Start: 3, End: 5}})
	e.found = [][2]int{{6, 8}}
	e.doubts = [][2]int{{9, 11}}
	e.props = []Proposal{{ID: "prop", A: 12, B: 14}}
	supplyTestMarks(e)
	if spans := e.nameRanges(); len(spans) != 1 || spans[0].a != 15 {
		t.Fatal(spans)
	}
	e.Select(15, 17)
	if len(e.nameRanges()) != 0 {
		t.Fatal("selection did not win")
	}
	e.Select(0, 0)
	e.opts.NoAI = true
	supplyTestMarks(e)
	if len(e.nameRanges()) != 2 {
		t.Fatal("hidden AI annotation suppressed names")
	}
	if len(e.Doc.Marks()) != 2 {
		t.Fatal("name links entered document marks")
	}
}

func TestNameLinksFollowEditedProse(t *testing.T) {
	e := New(NewDoc("林晚推门，晚晚回头。"), Options{})
	opened := ""
	e.SetNameLinks([]NameLink{{"林晚", "entry"}, {"晚晚", "entry"}}, func(id string) { opened = id })
	tt := newTester(e)
	if len(e.nameRanges()) != 2 {
		t.Fatal(e.nameRanges())
	}
	r, ok := e.RangeBox(0, 2)
	if !ok {
		t.Fatal("not visible")
	}
	caret := e.Caret()
	tt.Move(r.X+e.FontSize()/2, r.Y+r.H/2)
	if tt.Cursor() != ui.CursorPointer {
		t.Fatal("no name pointer")
	}
	tt.ClickAt(r.X+e.FontSize()/2, r.Y+r.H/2)
	if opened != "entry" || e.Caret() != caret {
		t.Fatal(opened, e.Caret())
	}
	tt.Type("雨")
	if e.Doc.String() != "雨林晚推门，晚晚回头。" || len(e.Doc.Marks()) != 0 {
		t.Fatal(e.Doc.String(), e.Doc.Marks())
	}
	supplyTestMarks(e)
	if spans := e.nameRanges(); len(spans) != 2 || spans[0].a != 1 {
		t.Fatal(spans)
	}
	e.SetNameLinks(nil, nil)
	opened = ""
	tt.Frame()
	tt.ClickAt(r.X+e.FontSize(), r.Y+r.H/2)
	if opened != "" {
		t.Fatal("hidden name opened")
	}
}

func TestNamesKeepInputPriority(t *testing.T) {
	e := New(NewDoc("甲乙丙甲乙丙甲乙"), Options{})
	e.SetNameLinks([]NameLink{{"乙丙", "first"}, {"甲乙", "second"}, {"乙", "third"}}, nil)
	want := []nameRange{{1, 3, "first"}, {4, 6, "first"}, {6, 8, "second"}}
	if got := e.nameRanges(); !slices.Equal(got, want) {
		t.Fatal(got)
	}
}
