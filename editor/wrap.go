package editor

import (
	"strings"
	"unicode"
)

// positions returns the x of each rune boundary of a text of n runes from
// the glyphs Shape placed: xs[i] is where rune i starts, xs[n] the end.
// Runes inside a cluster (ligatures) share its width evenly.
func positions(n int, gs []glyphPos) []float32 {
	xs := make([]float32, n+1)
	known := make([]bool, n+1)
	var end float32
	for _, g := range gs {
		if g.cluster >= 0 && g.cluster < n {
			if !known[g.cluster] || g.x < xs[g.cluster] {
				xs[g.cluster] = g.x
			}
			known[g.cluster] = true
		}
		end = max(end, g.x+g.advance)
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

// glyphPos is what positions needs of a glyph.
type glyphPos struct {
	cluster    int
	x, advance float32
}

// breakLines breaks a paragraph into lines no wider than width and returns
// the rune index each line starts at. Chinese breaks between any two
// characters, but for punctuation that must not start or end a line
// (避头尾); runs of Latin letters and digits break between words.
func breakLines(r []rune, xs []float32, width, indent float32) []int {
	starts := []int{0}
	if width <= 0 {
		return starts
	}
	ls := 0
	for i := 0; i < len(r); i++ {
		w := width
		if ls == 0 {
			w -= indent
		}
		if i == ls || xs[i+1]-xs[ls] <= w {
			continue
		}
		brk := i
		if isWordRune(r[i]) && isWordRune(r[i-1]) {
			j := i
			for j > ls && isWordRune(r[j-1]) {
				j--
			}
			if j > ls {
				brk = j
			}
		}
		// Pull a character down to the next line rather than start it
		// with a closing mark or end a line with an opening one; twice at
		// most, for marks that come in pairs such as ”。.
		for k := 0; k < 2 && brk > ls+1; k++ {
			if !noLineStart(r[brk]) && !noLineEnd(r[brk-1]) {
				break
			}
			brk--
		}
		for brk < len(r) && r[brk] == ' ' {
			brk++ // spaces stay at the end of the line they follow
		}
		if brk >= len(r) {
			break
		}
		starts = append(starts, brk)
		ls = brk
		i = brk - 1
	}
	return starts
}

func isWordRune(r rune) bool {
	return r < 0x2E80 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '\'' || r == '-')
}

func noLineStart(r rune) bool {
	return strings.ContainsRune("，。、；：？！）》」』”’…—·%,.;:?!)]}", r)
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
