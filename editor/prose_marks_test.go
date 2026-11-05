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
			e.SetProposals([]Proposal{{A: 0, B: 8}}, 0)
		case "selection":
			e.SetProposals(nil, 0)
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

func TestPartialSelectionDropsWholeMarkAndNameYieldsToMark(t *testing.T) {
	e := New(NewDoc("abcdefghij"), Options{})
	e.SetProseMarks([]ProseMark{{Kind: MarkCheck, Start: 0, End: 8}})
	e.SetNameLinks([]NameLink{{"cdef", "entry"}}, nil)
	if len(e.nameRanges()) != 0 {
		t.Fatal("name under mark survived")
	}
	e.Select(1, 2)
	if len(e.decorations()) != 0 {
		t.Fatal("part of overlapping mark survived")
	}
	if got := e.nameRanges(); len(got) != 1 {
		t.Fatal("dropped mark still hid name", got)
	}
}

func TestProseLayersWholeSpanPriority(t *testing.T) {
	e := New(NewDoc("abcdefghijklmnop"), Options{})
	e.SetProposals([]Proposal{{ID: "swap", A: 0, B: 4, Text: "new"}}, 0)
	e.Select(2, 6)
	e.SetFound([][2]int{{5, 8}, {7, 10}}, 0)
	e.SetProseMarks([]ProseMark{{Kind: MarkAI, Start: 4, End: 5}, {Kind: MarkCheck, Start: 9, End: 12}, {Kind: MarkForeshadow, Start: 10, End: 14}})
	layers := e.proseLayers()
	if !layers.selection || len(layers.proposals) != 1 || !slices.Equal(layers.found, [][2]int{{5, 8}}) || len(layers.marks) != 1 || layers.marks[0].Kind != MarkCheck {
		t.Fatalf("%+v", layers)
	}
	e.SetRewriting(true)
	layers = e.proseLayers()
	if !layers.rewrite || !layers.selection || len(layers.proposals) != 0 || !slices.Equal(layers.found, [][2]int{{7, 10}}) || len(layers.marks) != 1 || layers.marks[0].Kind != MarkForeshadow {
		t.Fatalf("%+v", layers)
	}
}

func TestHiddenEarlierNameDoesNotSuppressLaterName(t *testing.T) {
	e := New(NewDoc("abcdef"), Options{})
	e.SetNameLinks([]NameLink{{"abcd", "first"}, {"cdef", "second"}}, nil)
	e.Select(0, 1)
	if got := e.nameRanges(); len(got) != 1 || got[0].entry != "second" {
		t.Fatal(got)
	}
}

func TestRewriteOverlapDropsProposalTextAndRestoresItOnClose(t *testing.T) {
	e := New(NewDoc("abcdefgh"), Options{})
	e.width = 300
	e.SetProposals([]Proposal{{ID: "swap", A: 0, B: 4, Text: "NEW"}}, 0)
	e.Select(2, 3)
	e.SetRewriting(true)
	e.ensureLayout()
	if string(e.lay.disp) != "abcdefgh" {
		t.Fatalf("hidden proposal text remains: %q", string(e.lay.disp))
	}
	e.SetRewriting(false)
	e.ensureLayout()
	if !slices.Contains(e.lay.disp, 'N') {
		t.Fatal("closing rewrite did not restore proposal")
	}
	if !e.proseLayers().selection {
		t.Fatal("live selection disappeared over swap")
	}
}

func TestSelectedCurrentFindHitKeepsStrongFindLayer(t *testing.T) {
	e := New(NewDoc("one two one"), Options{})
	e.SetFound([][2]int{{0, 3}, {8, 11}}, 1)
	e.Select(8, 11)
	layers := e.proseLayers()
	if !layers.selection || !slices.Contains(layers.found, e.found[e.current]) {
		t.Fatalf("selected current hit lost find colour: %+v", layers)
	}
}

func TestLiveSelectionOverSwapIsPainted(t *testing.T) {
	e := New(NewDoc("abcdefgh"), Options{})
	e.SetProposals([]Proposal{{ID: "swap", A: 0, B: 4, Text: "NEW"}}, 0)
	e.Select(2, 6)
	layers := e.proseLayers()
	if !layers.selection || len(layers.proposals) != 1 {
		t.Fatalf("selection or swap suppressed: %+v", layers)
	}
}

func TestProseCacheTracksSelectionFindMarksNamesAndText(t *testing.T) {
	e := New(NewDoc("Mara rain"), Options{})
	e.SetNameLinks([]NameLink{{Name: "Mara", Entry: "mara"}}, nil)
	if len(e.nameRanges()) != 1 {
		t.Fatal("initial name missing")
	}
	e.SetFound([][2]int{{0, 4}}, 0)
	if len(e.nameRanges()) != 0 {
		t.Fatal("find did not invalidate names")
	}
	e.SetFound(nil, 0)
	if len(e.nameRanges()) != 1 {
		t.Fatal("clearing find left stale names")
	}
	e.SetProseMarks([]ProseMark{{Kind: MarkAI, Start: 0, End: 4}})
	if len(e.nameRanges()) != 0 {
		t.Fatal("mark did not invalidate names")
	}
	e.SetProseMarks(nil)
	e.Select(1, 2)
	if len(e.nameRanges()) != 0 {
		t.Fatal("selection did not invalidate names")
	}
	e.Select(5, 5)
	if len(e.nameRanges()) != 1 {
		t.Fatal("caret left stale name visibility")
	}
	e.SetNameLinks([]NameLink{{Name: "rain", Entry: "rain"}}, nil)
	if got := e.nameRanges(); len(got) != 1 || got[0].a != 5 {
		t.Fatal(got)
	}
	e.Replace(5, 9, "snow")
	if len(e.nameRanges()) != 0 {
		t.Fatal("text edit left stale name")
	}
}

func TestSpanIndexKeepsSameWholeSpansAsFirstOverlapRule(t *testing.T) {
	index := newSpanIndex(80)
	var kept [][2]int
	// Adjacent, nested and crossing spans, supplied in priority order.
	for i := 0; i < 300; i++ {
		a := (i * 37) % 80
		b := min(80, a+1+(i*19)%17)
		want := !slices.ContainsFunc(kept, func(r [2]int) bool { return r[0] < b && a < r[1] })
		if got := index.keep(a, b); got != want {
			t.Fatalf("[%d,%d): got %v, want %v", a, b, got, want)
		}
		if want {
			kept = append(kept, [2]int{a, b})
		}
	}
}
