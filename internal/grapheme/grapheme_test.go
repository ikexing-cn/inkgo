package grapheme

import (
	"math/rand"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/go-text/typesetting/segmenter"
)

// ends is every cluster's end as a rune offset.
func ends(s string) []int {
	out, offset, runes := []int{}, 0, 0
	Each(s, func(_ rune, end int) bool {
		runes += utf8.RuneCountInString(s[offset:end])
		out, offset = append(out, runes), end
		return true
	})
	return out
}

func TestAgainstSegmenter(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	pool := []rune("汉字abc12 -'\r\n👨‍👩‍👧🇨🇳👍🏽1️⃣é한\u0600\u0890\u0903\uff9e\u200c\u200d\U0001f600")
	for trial := 0; trial < 1500; trial++ {
		r := make([]rune, rng.Intn(120))
		for i := range r {
			r[i] = pool[rng.Intn(len(pool))]
		}
		s := string(r)
		var seg segmenter.Segmenter
		seg.Init(r)
		it := seg.GraphemeIterator()
		want := []int{}
		for it.Next() {
			g := it.Grapheme()
			want = append(want, g.Offset+len(g.Text))
		}
		if got := ends(s); !slices.Equal(got, want) {
			t.Fatalf("%q: %v want %v", s, got, want)
		}
		if Count(s) != len(want) {
			t.Fatal("count", s)
		}
		for n := 0; n <= len(want)+1; n++ {
			end, more := Prefix(s, n)
			idx := len(r)
			if n == 0 {
				idx = 0
			} else if n <= len(want) {
				idx = want[n-1]
			}
			if end != len(string(r[:idx])) || more != (n < len(want)) {
				t.Fatalf("prefix %q n=%d end=%d more=%v", s, n, end, more)
			}
			start, more := Suffix(s, n)
			idx = 0
			if n < len(want) {
				if n == 0 {
					idx = len(r)
				} else {
					idx = want[len(want)-n-1]
				}
			}
			if utf8.RuneCountInString(s[:start]) != idx || more != (n < len(want)) {
				t.Fatalf("suffix %q n=%d start=%d", s, n, start)
			}
		}
	}
}
