package editor

import (
	"strings"
	"sync"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/go-text/typesetting/segmenter"
)

// positions returns the x of each rune boundary of a text of n runes from
// the glyphs Shape placed. For visual-order glyphs these are cumulative
// logical advances for wrapping; line views supply the visual boundaries.
// Runes inside a cluster (ligatures) share its width evenly.
func positions(n int, gs []ui.Glyph) []float32 {
	if !orderedGlyphs(gs) {
		xs := make([]float32, n+1)
		for _, c := range visualClusters(n, gs) {
			for i := c.a; i < c.b; i++ {
				xs[i+1] = c.width / float32(c.b-c.a)
			}
		}
		for i := 1; i <= n; i++ {
			xs[i] += xs[i-1]
		}
		return xs
	}
	xs := make([]float32, n+1)
	known := make([]bool, n+1)
	var end float32
	for _, g := range gs {
		if g.Cluster >= 0 && g.Cluster < n {
			if !known[g.Cluster] || g.X < xs[g.Cluster] {
				xs[g.Cluster] = g.X
			}
			known[g.Cluster] = true
		}
		end = max(end, g.X+g.Advance)
	}
	xs[n], known[n] = end, true
	if !known[0] {
		xs[0], known[0] = 0, true
	}
	prev := 0
	for i := 1; i <= n; i++ {
		if !known[i] {
			continue
		}
		for k := prev + 1; k < i; k++ {
			xs[k] = xs[prev] + (xs[i]-xs[prev])*float32(k-prev)/float32(i-prev)
		}
		prev = i
	}
	for i := 1; i <= n; i++ {
		xs[i] = max(xs[i], xs[i-1])
	}
	return xs
}

// Reuse the segmenter's rune and attribute buffers across paragraph layouts.
var wrapSegmenters = sync.Pool{New: func() any { return new(segmenter.Segmenter) }}

// breakLines uses UAX #14 opportunities filtered by the local punctuation rules.
// An overwide unbreakable run falls back to grapheme boundaries; a single
// overwide cluster or punctuation group may overflow rather than be split.
func breakLines(r []rune, xs []float32, width, indent float32) []int {
	starts := []int{0}
	if width <= 0 || len(r) == 0 {
		return starts
	}
	return breakLinesAt(r, xs, width, indent, lineBreakFlags(r))
}

func lineBreakFlags(r []rune) []uint8 {
	seg := wrapSegmenters.Get().(*segmenter.Segmenter)
	defer wrapSegmenters.Put(seg)
	seg.Init(r)
	// 1: grapheme boundary, 2: line break, 4: mandatory, 8: local rules allow.
	flags := make([]uint8, len(r)+1)
	gi := seg.GraphemeIterator()
	for gi.Next() {
		g := gi.Grapheme()
		flags[g.Offset+len(g.Text)] |= 1
	}
	li := seg.LineIterator()
	for li.Next() {
		l := li.Line()
		end := l.Offset + len(l.Text)
		flags[end] |= 2
		if l.IsMandatoryBreak {
			flags[end] |= 4
		}
	}
	for i := 1; i <= len(r); i++ {
		if flags[i]&1 != 0 && (i == len(r) || (!noLineStart(r[i]) && !noLineEnd(r[i-1]))) {
			flags[i] |= 8
		}
	}
	return flags
}

func breakLinesAt(r []rune, xs []float32, width, indent float32, flags []uint8) []int {
	starts := []int{0}
	if width <= 0 {
		return starts
	}
	ls, last, fallback := 0, 0, 0
	for i := 1; i <= len(r); i++ {
		if flags[i]&1 == 0 {
			continue
		}
		w := width
		if ls == 0 {
			w -= indent
		}
		// Spaces belong to the preceding line and do not force an earlier break.
		if xs[i]-xs[ls] > w && r[i-1] != ' ' {
			brk := last
			if brk <= ls {
				brk = fallback
			}
			if brk <= ls && flags[i]&8 != 0 {
				brk = i
			}
			if brk > ls {
				if brk >= len(r) {
					break
				}
				starts = append(starts, brk)
				ls, last, fallback = brk, brk, brk
				i = brk
				continue
			}
		}
		if flags[i]&8 != 0 {
			fallback = i
			if flags[i]&2 != 0 {
				last = i
			}
			if flags[i]&4 != 0 && i < len(r) {
				starts = append(starts, i)
				ls, last, fallback = i, i, i
			}
		}
	}
	return starts
}

func isWordRune(r rune) bool {
	return r < 0x2E80 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '\'' || r == '-')
}

// noLineStart tells the runes a line may not start with: closing marks,
// and the room after what is proposed, which keeps to the words before it.
func noLineStart(r rune) bool {
	return r == gapRune || strings.ContainsRune("，。、；：？！）》」』”’…—·%,.;:?!)]}", r)
}
func noLineEnd(r rune) bool { return strings.ContainsRune("（《「『“‘([{", r) }

// runeClass groups runes for double-click selection.
func runeClass(r rune) int {
	switch {
	case unicode.Is(unicode.Han, r):
		return 1
	case unicode.IsLetter(r) || unicode.IsDigit(r):
		return 2
	case unicode.IsSpace(r):
		return 3
	}
	return 4
}
