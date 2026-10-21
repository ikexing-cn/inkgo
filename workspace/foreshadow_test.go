package workspace

import (
	"reflect"
	"testing"
)

func foreTree() Tree {
	return Tree{Root: &Node{Children: []*Node{
		{ID: "volume", Folder: true, Children: []*Node{{ID: "a", Title: "甲"}, {ID: "b", Title: "乙"}}},
		{ID: "c", Title: "丙"}, {ID: "d", Title: "丁"},
	}}}
}

func TestForeshadowStatus(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		f := Foreshadow{Dropped: mask&1 != 0, Lost: mask&2 != 0}
		if mask&4 != 0 {
			f.Payoffs = []Payoff{{ID: "pay"}}
		}
		want := ForeOpen
		switch {
		case f.Dropped:
			want = ForeDropped
		case f.Lost:
			want = ForeLost
		case len(f.Payoffs) > 0:
			want = ForeDone
		}
		if f.Status() != want {
			t.Fatal(mask, f.Status(), want)
		}
	}
}

func TestForeshadows(t *testing.T) {
	stored := map[string][]ChapterMark{
		"volume": {{ID: "folder", Kind: MarkForeshadow}},
		"a": {
			{ID: "first", Kind: MarkForeshadow, Note: "备注", Quote: "原句", Start: 2, End: 4, At: "time"},
			{ID: "lost", Kind: MarkForeshadow, Orphaned: true},
			{ID: "dropped", Kind: MarkForeshadow, Orphaned: true, Dropped: true},
			{ID: "before", Kind: MarkPayoff, Of: "later"},
		},
		"b": {
			{ID: "pay-one", Kind: MarkPayoff, Of: "first", Quote: "第一次回收", Start: 5, End: 9, At: "pay-time"},
			{ID: "orphan", Kind: MarkPayoff, Of: "first", Orphaned: true},
			{ID: "unlinked", Kind: MarkPayoff},
			{ID: "lost-pay", Kind: MarkPayoff, Of: "lost"},
		},
		"c":       {{ID: "pay-two", Kind: MarkPayoff, Of: "first"}, {ID: "later", Kind: MarkForeshadow}},
		"d":       {{ID: "open", Kind: MarkForeshadow}},
		"deleted": {{ID: "gone", Kind: MarkForeshadow}},
	}
	got := foreTree().Foreshadows(stored, "", nil)
	var ids []string
	for _, f := range got {
		ids = append(ids, f.ID)
	}
	if !reflect.DeepEqual(ids, []string{"first", "lost", "dropped", "later", "open"}) {
		t.Fatal(ids)
	}
	first := got[0]
	if first.Node != "a" || first.Title != "甲" || first.Note != "备注" || first.Quote != "原句" || first.Start != 2 || first.End != 4 || first.At != "time" || first.Chapter != 1 || first.Here || first.Gap != 1 || first.Status() != ForeDone {
		t.Fatal(first)
	}
	if len(first.Payoffs) != 2 || first.Payoffs[0] != (Payoff{ID: "pay-one", Node: "b", Title: "乙", Quote: "第一次回收", At: "pay-time", Chapter: 2, Start: 5, End: 9}) || first.Payoffs[1].ID != "pay-two" {
		t.Fatal(first.Payoffs)
	}
	if got[1].Status() != ForeLost || len(got[1].Payoffs) != 1 || got[2].Status() != ForeDropped || got[3].Gap != 0 || got[3].Payoffs[0].Chapter != 1 || got[4].Gap != 0 || got[4].Status() != ForeOpen {
		t.Fatal(got)
	}
	if got := (Tree{}).Foreshadows(stored, "", nil); got != nil {
		t.Fatal(got)
	}
	if len(stored["b"]) != 4 || stored["a"][0].Quote != "原句" {
		t.Fatal("stored marks changed", stored)
	}
}

func TestForeshadowsUseLiveMarks(t *testing.T) {
	tree := foreTree()
	stored := map[string][]ChapterMark{
		"a": {{ID: "f", Kind: MarkForeshadow, Quote: "old"}},
		"b": {{ID: "pay", Kind: MarkPayoff, Of: "f"}},
	}
	live := []ChapterMark{{ID: "f", Kind: MarkForeshadow, Quote: "正在写", Start: 1, End: 4}}
	f := tree.Foreshadows(stored, "a", live)[0]
	if !f.Here || f.Quote != "正在写" || f.Start != 1 || f.End != 4 || len(f.Payoffs) != 1 || f.Payoffs[0].Here {
		t.Fatal(f)
	}
	f = tree.Foreshadows(stored, "b", nil)[0]
	if f.Here || len(f.Payoffs) != 0 || f.Status() != ForeOpen || f.Gap != 3 {
		t.Fatal(f)
	}
	if got := tree.Foreshadows(stored, "a", nil); len(got) != 0 {
		t.Fatal("empty live marks fell back to disk", got)
	}
	f = tree.Foreshadows(stored, "b", []ChapterMark{{ID: "live-pay", Kind: MarkPayoff, Of: "f"}})[0]
	if !f.Payoffs[0].Here || f.Payoffs[0].ID != "live-pay" {
		t.Fatal(f)
	}
}

func TestForeshadowCandidates(t *testing.T) {
	tree := foreTree()
	all := []Foreshadow{
		{ID: "open", Chapter: 1, Note: "Needle"},
		{ID: "paid", Chapter: 1, Quote: "Needle", Payoffs: []Payoff{{ID: "pay"}}},
		{ID: "here", Chapter: 2},
		{ID: "future", Chapter: 3, Note: "Needle"},
		{ID: "lost", Chapter: 1, Lost: true},
		{ID: "dropped", Chapter: 1, Dropped: true},
	}
	for _, tc := range []struct {
		current, query string
		ids            []string
		here           int
	}{
		{"b", "", []string{"open", "paid", "here"}, 2},
		{"b", " Needle ", []string{"open", "paid"}, 2},
		{"b", "needle", nil, 2}, {"a", "", []string{"open", "paid"}, 1},
		{"volume", "", nil, 0}, {"missing", "", nil, 0}, {"", "", nil, 0},
	} {
		got, here := tree.ForeshadowCandidates(all, tc.current, tc.query)
		var ids []string
		for _, f := range got {
			ids = append(ids, f.ID)
		}
		if !reflect.DeepEqual(ids, tc.ids) || here != tc.here {
			t.Fatal(tc, ids, here)
		}
	}
}

func TestForeshadowLate(t *testing.T) {
	for _, after := range []int{-1, 0, 5, 10, 30, 120} {
		threshold := after
		if threshold <= 0 {
			threshold = 30
		}
		if LateThreshold(after) != threshold {
			t.Fatal(after, LateThreshold(after))
		}
		for _, gap := range []int{threshold - 1, threshold, threshold + 1} {
			for mask := 0; mask < 8; mask++ {
				f := Foreshadow{Gap: gap, Lost: mask&1 != 0, Dropped: mask&2 != 0}
				if mask&4 != 0 {
					f.Payoffs = []Payoff{{ID: "pay"}}
				}
				if f.Late(after) != (mask == 0 && gap >= threshold) {
					t.Fatal(after, f)
				}
			}
		}
	}
}
