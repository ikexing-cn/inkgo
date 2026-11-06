package grapheme

import (
	"math/rand"
	"testing"
	"unicode"

	"github.com/go-text/typesetting/segmenter"

	"writer/internal/grapheme/tablegen"
)

func TestTables(t *testing.T) {
	if tablegen.Tables() != bmpSimple {
		t.Fatal("run go generate")
	}
	var seg segmenter.Segmenter
	for r := rune(0); r < 65536; r++ {
		if bmpSimple[r>>3]&(1<<uint(r&7)) == 0 {
			continue
		}
		for _, p := range []rune{'a', '\n', 0x4e00, 0xac00, 0xac01, 0xb7, 0x3000, 0xff01, r} {
			for _, pair := range [][2]rune{{r, p}, {p, r}} {
				seg.Init(pair[:])
				it := seg.GraphemeIterator()
				n := 0
				for it.Next() {
					n++
				}
				if n != 2 {
					t.Fatalf("%U %U", pair[0], pair[1])
				}
			}
		}
	}
}

var initSink [65536]uint8

func BenchmarkKindInit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		initSink = makeKind()
	}
}

func TestMalformedAndNonSpace(t *testing.T) {
	pool := []string{"汉", "，", "\xff", "\xe4\xb8", "\xe0\x80\x80", "\xed\xa0\x80", "\r\n", "é", "\u3099", "\u200d", "\u1161", "　", "\u00a0", "�", "か", "é", "👍🏽"}
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 20000; trial++ {
		s := ""
		for j := rng.Intn(20); j > 0; j-- {
			s += pool[rng.Intn(len(pool))]
		}
		var seg segmenter.Segmenter
		seg.Init([]rune(s))
		it := seg.GraphemeIterator()
		n, non := 0, 0
		for it.Next() {
			n++
			if !unicode.IsSpace(it.Grapheme().Text[0]) {
				non++
			}
		}
		if Count(s) != n || NonSpace(s) != non {
			t.Fatalf("%q", s)
		}
		for limit := 0; limit <= n+1; limit++ {
			x, y := Prefix(s, limit)
			a, b := prefixBefore(s, limit)
			if x != a || y != b {
				t.Fatalf("prefix %q %d", s, limit)
			}
			x, y = Suffix(s, limit)
			a, b = suffixBefore(s, limit)
			if x != a || y != b {
				t.Fatalf("suffix %q %d", s, limit)
			}
		}
		calls := 0
		Each(s, func(rune, int) bool { calls++; return false })
		if n > 0 && calls != 1 {
			t.Fatal("early stop")
		}
	}
}

// byteEnds is every cluster's end as a byte offset, straight from the segmenter.
func byteEnds(s string) []int {
	var seg segmenter.Segmenter
	seg.Init([]rune(s))
	it := seg.GraphemeIterator()
	// Rune i of the slice starts at byte at[i] of s; an invalid byte is one rune.
	at := []int{}
	for i := range s {
		at = append(at, i)
	}
	at = append(at, len(s))
	ends := []int{0}
	for it.Next() {
		g := it.Grapheme()
		ends = append(ends, at[g.Offset+len(g.Text)])
	}
	return ends
}

func TestInvalidBytesKeepTheirOwnWidth(t *testing.T) {
	if end, more := Prefix("\xff\xff", 1); end != 1 || !more {
		t.Fatal("prefix", end, more)
	}
	if start, more := Suffix("\xff\xffe\u0301", 1); start != 2 || !more {
		t.Fatal("suffix", start, more)
	}
}

func prefixBefore(s string, n int) (int, bool) {
	if n <= 0 {
		return 0, len(s) > 0
	}
	ends := byteEnds(s)
	if n >= len(ends)-1 {
		return len(s), false
	}
	return ends[n], true
}

func suffixBefore(s string, n int) (int, bool) {
	if n <= 0 {
		return len(s), len(s) > 0
	}
	ends := byteEnds(s)
	cut := ends[max(0, len(ends)-1-n)]
	return cut, cut > 0
}
