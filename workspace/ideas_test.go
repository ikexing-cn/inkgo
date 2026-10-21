package workspace

import (
	"reflect"
	"testing"
)

func TestIdeas(t *testing.T) {
	s := Setting{Entries: []Entry{
		{ID: "lin", Place: PlaceBody, Names: []string{"林", "林晚"}, Changes: []Change{
			{ID: "later", From: "b", Place: PlaceOpen},
			{ID: "earlier", From: "a", Place: PlaceOpen},
			{ID: "gone", From: "gone", Place: PlaceOpen},
			{ID: "settled", From: "a", Place: PlaceBody},
		}},
		{ID: "zhou", Place: PlaceBody, Names: []string{"周祥", "小祥", "saki"}},
		{ID: "tie", Place: PlaceBody, Names: []string{"小祥"}},
		{ID: "dropped", Place: PlaceDropped, Names: []string{"一个很长的名字"}, Changes: []Change{{ID: "ignored", From: "a", Place: PlaceOpen}}},
		{ID: "longest", Place: PlaceOpen, Text: "林晚和林一起走。"},
		{ID: "shared", Place: PlaceOpen, Names: []string{"林"}, Text: "周祥和小祥还有saki的另一种叫法。"},
		{ID: "mention", Place: PlaceOpen, Text: "小祥怎么称呼？"},
		{ID: "fresh", Place: PlaceOpen, Text: "一个很长的名字"},
	}}
	got := s.Ideas(Order{"a": 0, "b": 1})
	var ids, about []string
	for _, i := range got {
		ids = append(ids, i.ID())
		about = append(about, i.About)
	}
	if !reflect.DeepEqual(ids, []string{"longest", "shared", "mention", "fresh", "later", "earlier"}) || !reflect.DeepEqual(about, []string{"lin", "lin", "zhou", "", "lin", "lin"}) {
		t.Fatal(ids, about)
	}
	if got[0].Entry.ID != "longest" || got[0].Change.ID != "" || got[4].Entry.ID != "lin" || got[4].Change.ID != "later" {
		t.Fatal(got)
	}
	if got := s.Ideas(nil); len(got) != 4 {
		t.Fatal(got)
	}
	if got := (Setting{}).Ideas(nil); got != nil {
		t.Fatal(got)
	}
}

func TestIdeaID(t *testing.T) {
	for _, tc := range []struct {
		i    Idea
		want string
	}{
		{Idea{}, ""},
		{Idea{Entry: Entry{ID: "entry"}}, "entry"},
		{Idea{Entry: Entry{ID: "entry"}, Change: Change{ID: "change"}}, "change"},
	} {
		if tc.i.ID() != tc.want {
			t.Fatal(tc)
		}
	}
}
