package editor

import (
	"image"
	"strings"
	"testing"
)

func TestRangeBoxIncludesTheMarginOutsideTheViewport(t *testing.T) {
	e := New(NewDoc(strings.Repeat("雨落在门前。\n", 100)), Options{})
	if _, ok := e.RangeBoxUnclipped(0, 2); ok {
		t.Fatal("geometry before paint")
	}
	tt := newTester(e)
	below, ok := e.RangeBoxUnclipped(630, 634)
	if !ok || below.Y <= e.Height() {
		t.Fatal("missing offscreen range", below, ok)
	}
	if _, ok := e.RangeBox(630, 634); ok {
		t.Fatal("RangeBox must retain its visible-only contract")
	}
	first, _ := e.RangeBoxUnclipped(0, 2)
	e.scrollY = 500
	e.reveal = false
	tt.Frame()
	above, ok := e.RangeBoxUnclipped(0, 2)
	if !ok || above.Y != first.Y-500 || above.Y >= 0 {
		t.Fatal("offscreen geometry must follow scrolling", first, above)
	}
	if _, ok := e.RangeBox(0, 2); ok {
		t.Fatal("offscreen range is visible")
	}
}

func TestMarginHighlightAndMarkedGlyphHoverAreDisplayOnly(t *testing.T) {
	e := New(NewDoc("林晚推开门，旧钥匙掉了出来。\n"+strings.Repeat("她没有回头。", 25)), Options{})
	e.SetDoubts([][2]int{{0, 2}})
	e.AddMark(Mark{Kind: MarkForeshadow, Start: 6, End: 9})
	e.SetNameLinks([]NameLink{{"回头", "entry"}}, nil)
	tt := newTester(e)
	tt.Move(590, 390)
	version, marks := e.Doc.Version(), e.Doc.Marks()
	base := tt.Image()
	e.SetNoteHighlight(0, 2)
	tt.Frame()
	r, _ := e.RangeBox(0, 2)
	if !nameLineChanged(base, tt.Image(), image.Rect(int(r.X), int(r.Y), int(r.X+e.FontSize()*2), int(r.Y+r.H))) {
		t.Fatal("host range has no background")
	}
	for _, span := range [][2]int{{0, 2}, {6, 9}} {
		r, _ := e.RangeBox(span[0], span[1])
		tt.Move(r.X+2, r.Y+r.H/2)
		a, b := e.HoveredLine()
		if a != span[0] || b != span[1] {
			t.Fatal("marked glyph hover", a, b, span)
		}
	}
	tt.Move(e.Width()-20, r.Y+r.H/2)
	if a, b := e.HoveredLine(); a != b {
		t.Fatal("empty space hit a marked range")
	}
	e.SetNoteHighlight(0, 0)
	tt.Frame()
	if e.Doc.Version() != version || len(e.Doc.Marks()) != len(marks) || e.Doc.String() != "林晚推开门，旧钥匙掉了出来。\n"+strings.Repeat("她没有回头。", 25) {
		t.Fatal("display state entered the document")
	}
}
