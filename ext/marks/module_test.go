package marks

import (
	"testing"
	"writer/editor"
	"writer/ext"
	"writer/ext/hostapi"
	"writer/ext/marks/marksapi"
)

func TestSourcesKeepRegistrationOrderAndOptionalSourcesDisappearAndReturn(t *testing.T) {
	source := ext.Module{ID: "source", Optional: []string{"marks"}, Setup: func(r *ext.Registrar) {
		ext.Contribute(r, marksapi.MarginSources, "annotations", marksapi.MarginSource{Notes: func(chapter, text string) []marksapi.MarginItem {
			return []marksapi.MarginItem{{Start: 2, End: 3, Look: editor.MarkAI}, {Start: 2, End: 3, Look: editor.MarkCheck}, {Start: 2, End: 3, Look: editor.MarkForeshadow}}
		}})
	}}
	h, err := ext.New(ext.Options{}, Module, source)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	for _, on := range []bool{true, false, true} {
		if err = h.Set("marks", on); err != nil {
			t.Fatal(err)
		}
		if h.Ledger("source").State != ext.On {
			t.Fatal("optional source stopped")
		}
		values := ext.Items(h, hostapi.ProseMarks)
		if !on {
			if len(values) != 0 {
				t.Fatal("editor output survived")
			}
			continue
		}
		got := values[0].Value.Marks("chapter", "a\nbc")
		if len(got) != 3 || got[0].Kind != editor.MarkAI || got[1].Kind != editor.MarkCheck || got[2].Kind != editor.MarkForeshadow {
			t.Fatal(got)
		}
		reader, _ := ext.Lookup(h, marksapi.Service)
		items := reader.Items("chapter", "a\nbc")
		if items[0].Paragraph != 1 || items[0].Character != 0 {
			t.Fatal(items)
		}
	}
}

func TestSourcesAndOutputAppearInTheLedgerEvenWhenDisabled(t *testing.T) {
	h, err := ext.New(ext.Options{}, Module)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err = h.Set("marks", false); err != nil {
		t.Fatal(err)
	}
	l := h.Ledger("marks")
	if len(l.Points) != 1 || l.Points[0] != string(marksapi.MarginSources) || len(l.Provides) != 1 || len(l.Contributions) != 1 || l.Contributions[0].Point != string(hostapi.ProseMarks) {
		t.Fatal(l)
	}
}

func TestSourceCacheSteadyFramesBuildNothingAndSourceVersionAndSwitchRebuildOnce(t *testing.T) {
	version, calls := 0, 0
	source := ext.Module{ID: "source", Optional: []string{"marks"}, Setup: func(r *ext.Registrar) {
		ext.Contribute(r, marksapi.MarginSources, "notes", marksapi.MarginSource{
			Key: func() any { return version },
			Notes: func(string, string) []marksapi.MarginItem {
				calls++
				return []marksapi.MarginItem{{Start: 3, End: 4, Look: editor.MarkForeshadow}}
			},
		})
	}}
	h, err := ext.New(ext.Options{}, Module, source)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	service, _ := ext.Lookup(h, marksapi.Service)
	library := service.(*library)
	library.Items("chapter", "雨\n林晚")
	steady := func() {
		t.Helper()
		before, sourceCalls := library.builds, calls
		for range 10 {
			library.Revision()
			library.Items("chapter", "雨\n林晚")
		}
		if library.builds != before || calls != sourceCalls {
			t.Fatal("steady frames rebuilt sources", library.builds-before, calls-sourceCalls)
		}
	}
	steady()
	for _, change := range []func(){func() { version++ }, func() { err = h.Set("source", false) }, func() { err = h.Set("source", true) }} {
		before := library.builds
		change()
		if err != nil {
			t.Fatal(err)
		}
		library.Items("chapter", "雨\n林晚")
		if library.builds != before+1 {
			t.Fatal("change must rebuild once", library.builds-before)
		}
		steady()
	}
}

func TestSourcePositionsUseRuneOffsetsAcrossParagraphsAndClampToText(t *testing.T) {
	source := ext.Module{ID: "source", Requires: []string{"marks"}, Setup: func(r *ext.Registrar) {
		ext.Contribute(r, marksapi.MarginSources, "positions", marksapi.MarginSource{Notes: func(string, string) []marksapi.MarginItem {
			var out []marksapi.MarginItem
			for _, start := range []int{-1, 0, 1, 2, 3, 4, 5, 99} {
				out = append(out, marksapi.MarginItem{Start: start})
			}
			return out
		}})
	}}
	h, err := ext.New(ext.Options{}, Module, source)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	reader, _ := ext.Lookup(h, marksapi.Service)
	items := reader.Items("chapter", "雨\n林晚\n")
	for i, want := range [][2]int{{0, 0}, {0, 0}, {0, 1}, {1, 0}, {1, 1}, {1, 2}, {2, 0}, {2, 0}} {
		if got := [2]int{items[i].Paragraph, items[i].Character}; got != want {
			t.Fatal(i, got, want)
		}
	}
}

func TestMarginSourceRegistrationOrderOverridesMarkKind(t *testing.T) {
	source := func(id string, kind editor.MarkKind) ext.Module {
		return ext.Module{ID: id, Optional: []string{"marks"}, Setup: func(r *ext.Registrar) {
			ext.Contribute(r, marksapi.MarginSources, id, marksapi.MarginSource{Notes: func(string, string) []marksapi.MarginItem {
				return []marksapi.MarginItem{{Start: 0, End: 4, Look: kind}}
			}})
		}}
	}
	h, err := ext.New(ext.Options{}, Module, source("first", editor.MarkAI), source("second", editor.MarkForeshadow))
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	values := ext.Items(h, hostapi.ProseMarks)
	got := values[0].Value.Marks("chapter", "text")
	if len(got) != 2 || got[0].Kind != editor.MarkAI || got[1].Kind != editor.MarkForeshadow {
		t.Fatal(got)
	}
}
