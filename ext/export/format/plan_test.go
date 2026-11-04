package format

import (
	"slices"
	"testing"

	"writer/workspace"
)

func TestPlanRanges(t *testing.T) {
	pieces := []workspace.Piece{
		{ID: "outer", Title: "外卷", Folder: true},
		{ID: "inner", Title: "内卷", Folder: true, Depth: 1},
		{ID: "a", Title: "甲", Depth: 2, Text: "一 二", Outline: "纲"},
		{ID: "b", Title: "乙", Depth: 1, Text: "三"},
		{ID: "c", Title: "丙", Text: "四五"},
	}
	for _, tc := range []struct {
		name, current, scope, from, to, part string
		ids                                  []string
		words                                int
		whole                                bool
	}{
		{"whole", "a", ScopeWork, "", "", "", []string{"outer", "inner", "a", "b", "c"}, 5, true},
		{"inner volume", "a", ScopeVolume, "", "", "内卷", []string{"outer", "inner", "a"}, 2, false},
		{"open folder", "outer", ScopeVolume, "", "", "外卷", []string{"outer", "inner", "a", "b"}, 3, false},
		{"chapter", "a", ScopeChapter, "", "", "甲", []string{"outer", "inner", "a"}, 2, false},
		{"span", "a", ScopeSpan, "乙", "丙", "乙至丙", []string{"outer", "b", "c"}, 3, false},
		{"single span", "a", ScopeSpan, "乙", "乙", "乙", []string{"outer", "b"}, 1, false},
		{"no current chapter", "", ScopeChapter, "", "", "", []string{"outer", "inner", "a", "b", "c"}, 5, true},
		{"unknown scope", "a", "unknown", "", "", "", []string{"outer", "inner", "a", "b", "c"}, 5, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := BuildPlan(pieces, nil, tc.current, Options{Prose: true}, Selection{Scope: tc.scope, From: tc.from, To: tc.to}, func(from, to string) string { return from + "至" + to })
			var ids []string
			for _, piece := range p.Pieces {
				ids = append(ids, piece.ID)
			}
			if !slices.Equal(ids, tc.ids) || p.Part != tc.part || p.Whole != tc.whole || p.Words != tc.words {
				t.Fatalf("plan %+v, ids %v", p, ids)
			}
			want := []string{ScopeWork, ScopeSpan}
			switch tc.current {
			case "a":
				want = []string{ScopeWork, ScopeVolume, ScopeChapter, ScopeSpan}
			case "outer":
				want = []string{ScopeWork, ScopeVolume, ScopeSpan}
			}
			scope := tc.scope
			if !slices.Contains(want, scope) {
				scope = ScopeWork
			}
			if p.Scope != scope {
				t.Fatalf("scope %q, want %q", p.Scope, scope)
			}
			if !slices.Equal(p.Scopes, want) || p.Current != tc.current || !slices.Equal(p.Names, []string{"甲", "乙", "丙"}) {
				t.Fatal(p)
			}
		})
	}
}

func TestPlanContents(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		prose, outline, setting := mask&1 != 0, mask&2 != 0, mask&4 != 0
		p := BuildPlan(book, []workspace.Entry{{Place: workspace.PlaceBody}}, "1", Options{Prose: prose, Outline: outline, Setting: setting}, Selection{Scope: ScopeWork}, func(from, to string) string { return from + "至" + to })
		var want []string
		for i, name := range []string{ProseContent, OutlineContent, SettingContent} {
			if mask&(1<<i) != 0 {
				want = append(want, name)
			}
		}
		if !slices.Equal((Options{Prose: prose, Outline: outline, Setting: setting}).Ticked(), want) || !slices.Equal(p.Ticked, want) || !slices.Equal(p.Going, want) {
			t.Fatalf("selection %d: %+v", mask, p)
		}
	}
	for _, tc := range []struct {
		pieces                  []workspace.Piece
		entries                 []workspace.Entry
		going                   []string
		prose, outline, setting string
	}{
		{nil, nil, nil, SkipNoChapters, SkipNoChapters, SkipNoLore},
		{book[2:], []workspace.Entry{{Place: workspace.PlaceOpen}}, []string{ProseContent}, "", SkipNoOutlines, SkipNoLore},
		{nil, []workspace.Entry{{Place: workspace.PlaceBody}}, []string{SettingContent}, SkipNoChapters, SkipNoChapters, ""},
	} {
		p := BuildPlan(tc.pieces, tc.entries, "", Options{Prose: true, Outline: true, Setting: true, Text: true}, Selection{Scope: ScopeWork}, func(from, to string) string { return from + "至" + to })
		if !slices.Equal(p.Going, tc.going) || p.Skip[ProseContent] != tc.prose || p.Skip[OutlineContent] != tc.outline || p.Skip[SettingContent] != tc.setting {
			t.Fatal(p)
		}
	}
	for _, tc := range []struct {
		prose, outline, setting, text bool
		about                         string
	}{
		{true, true, false, false, AboutMany},
		{false, false, true, true, AboutLore},
		{false, true, false, true, AboutOutline},
		{true, false, false, true, AboutText},
		{true, false, false, false, AboutWord},
	} {
		p := BuildPlan(book, nil, "", Options{Prose: tc.prose, Outline: tc.outline, Setting: tc.setting, Text: tc.text}, Selection{Scope: ScopeWork}, func(from, to string) string { return from + "至" + to })
		if p.About != tc.about {
			t.Fatal(p.About)
		}
	}
}

func TestChapterNames(t *testing.T) {
	pieces := []workspace.Piece{{ID: "folder", Title: "同名", Folder: true}, {ID: "a", Title: "同名"}, {ID: "b", Title: "同名"}, {ID: "c", Title: "同名 (2)"}}
	names, ids := ChapterNames(pieces)
	if !slices.Equal(names, []string{"同名", "同名 (2)", "同名 (2) (2)"}) || ids[names[0]] != "a" || ids[names[1]] != "b" || ids[names[2]] != "c" {
		t.Fatal(names, ids)
	}
	p := BuildPlan(pieces[1:2], nil, "a", Options{Prose: true}, Selection{Scope: ScopeSpan}, func(from, to string) string { return from + "至" + to })
	if !p.Whole || !slices.Equal(p.Scopes, []string{ScopeWork, ScopeChapter}) {
		t.Fatal(p)
	}
}
