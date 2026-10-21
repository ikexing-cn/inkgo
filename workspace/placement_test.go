package workspace

import "testing"

func TestPlaceProposal(t *testing.T) {
	for _, tc := range []struct {
		name, at, quote, text, base string
		near                        int
		want                        Placement
		ok                          bool
		wantNear                    int
	}{
		{"replace", ProposeReplace, "雨🙂", "她说雨🙂停了", "", 0, Placement{A: 2, B: 4}, true, 2},
		{"after", ProposeAfter, "雨🙂", "她说雨🙂停了", "", 0, Placement{A: 4, B: 4, Keep: true}, true, 2},
		{"empty quote", ProposeReplace, "", "正文", "", 7, Placement{}, false, 7},
		{"missing quote", ProposeReplace, "雨", "正文", "", 7, Placement{}, false, 7},
		{"nearest quote", ProposeReplace, "雨", "雨停雨落雨", "", 3, Placement{A: 2, B: 3}, true, 2},
		{"append empty", ProposeEnd, "", "", "", 3, Placement{Keep: true}, true, 3},
		{"append paragraph", ProposeEnd, "", "正文", "", 3, Placement{A: 2, B: 2, Keep: true, Text: "\n新文"}, true, 3},
		{"append after newline", ProposeEnd, "", "正文\n", "", 3, Placement{A: 3, B: 3, Keep: true}, true, 3},
		{"whole chapter", ProposeAll, "", "正文", "正文", 3, Placement{B: 2, Apart: true}, true, 3},
		{"whole empty", ProposeAll, "", "", "", 3, Placement{}, true, 3},
		{"whole changed", ProposeAll, "", "改动", "正文", 3, Placement{B: 2, Apart: true}, false, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Proposal{At: tc.at, Quote: tc.quote, Text: "新文"}
			near := tc.near
			got, near, ok := p.Place(tc.text, tc.base, near)
			want := tc.want
			if want.Text == "" {
				want.Text = p.Text
			}
			if got != want || ok != tc.ok || near != tc.wantNear {
				t.Fatalf("got %+v, %v, near %d; want %+v, %v, near %d", got, ok, near, want, tc.ok, tc.wantNear)
			}
		})
	}
}

func TestFindQuote(t *testing.T) {
	for _, tc := range []struct {
		name, text, quote string
		near, want        int
		ok                bool
	}{
		{"rune offsets", "她🙂说雨停了", "雨", 0, 3, true},
		{"nearest", "雨停雨落雨", "雨", 4, 4, true},
		{"equidistant takes first", "雨停雨落雨", "雨", 3, 2, true},
		{"nonoverlapping matches", "aaa", "aa", 1, 0, true},
		{"missing", "正文", "雨", 9, 0, false},
		{"empty quote", "正文", "", 9, 0, false},
		{"empty text", "", "雨", 9, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			near := tc.near
			got, ok := findQuote(tc.quote, tc.text, near)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("got %d, %v; want %d, %v", got, ok, tc.want, tc.ok)
			}
			_, next, placed := (Proposal{At: ProposeReplace, Quote: tc.quote}).Place(tc.text, "", near)
			wantNear := tc.near
			if tc.ok {
				wantNear = tc.want
			}
			if placed != tc.ok || next != wantNear {
				t.Fatalf("returned offset %d, want %d; placed %v", next, wantNear, placed)
			}
		})
	}
}

func TestProposalPlacementFollowsEdits(t *testing.T) {
	p := Proposal{At: ProposeReplace, Quote: "雨", Text: "雪"}
	near := 4
	for _, tc := range []struct {
		text string
		want int
	}{
		{"雨停，夜里雨落", 5},
		{"雨停，雨又落，夜里雨落", 3},
		{"雨停，雨又落", 3},
	} {
		got, next, ok := p.Place(tc.text, "", near)
		near = next
		if !ok || got.A != tc.want || got.B != tc.want+1 || near != tc.want {
			t.Fatalf("in %q: %+v, %v, near %d", tc.text, got, ok, near)
		}
	}
}
