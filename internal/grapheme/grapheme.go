// Package grapheme provides rune-indexed Unicode grapheme boundaries.
package grapheme

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-text/typesetting/segmenter"
)

//go:generate go run gen.go

var kind = makeKind()

func makeKind() (k [65536]uint8) {
	for r := range k {
		if bmpSimple[r>>3]&(1<<uint(r&7)) != 0 {
			k[r] = 1
			if unicode.IsSpace(rune(r)) {
				k[r] = 2
			}
		}
	}
	return
}

// decode handles the common three-byte BMP spelling without a decoder call.
// Invalid spellings take the slow paragraph path, preserving segmenter offsets.
func decode(s string, i int) (r rune, size int) {
	b := s[i]
	if b < utf8.RuneSelf {
		return rune(b), 1
	}
	if b&0xf0 == 0xe0 && i+2 < len(s) && s[i+1]&0xc0 == 0x80 && s[i+2]&0xc0 == 0x80 {
		r = rune(b&15)<<12 | rune(s[i+1]&63)<<6 | rune(s[i+2]&63)
		if r >= 0x800 && (r < 0xd800 || r > 0xdfff) {
			return r, 3
		}
		return -1, 1
	}
	r, size = utf8.DecodeRuneInString(s[i:])
	if r == utf8.RuneError && size == 1 {
		return -1, 1
	}
	return
}

func paragraphEnd(s string, i int) int {
	if n := strings.IndexByte(s[i:], '\n'); n >= 0 {
		return i + n + 1
	}
	return len(s)
}

// segment hands s to the segmenter. Its byte offsets are those of the rune
// slice, where an invalid byte became a three-byte U+FFFD, so ends are counted
// in s itself, one rune at a time.
func segment(s string, base int, yield func(rune, int) bool) bool {
	var seg segmenter.Segmenter
	seg.Init([]rune(s))
	it := seg.GraphemeIterator()
	b, runes := 0, 0
	for it.Next() {
		g := it.Grapheme()
		for end := g.Offset + len(g.Text); runes < end; runes++ {
			_, size := utf8.DecodeRuneInString(s[b:])
			b += size
		}
		if !yield(g.Text[0], base+b) {
			return false
		}
	}
	return true
}

// Each calls yield for each cluster's first rune and ending byte offset.
// Returning false stops iteration. Newlines stay with their preceding paragraph
// so CRLF and combining sequences on either side retain Unicode semantics.
func Each(s string, yield func(rune, int) bool) {
	for start := 0; start < len(s); {
		end, fast := start, true
		for end < len(s) {
			r, size := decode(s, end)
			if r < 0 || r >= 65536 || kind[r] == 0 {
				fast = false
				end = paragraphEnd(s, end)
				break
			}
			end += size
			if r == '\n' {
				break
			}
		}
		if fast {
			for i := start; i < end; {
				r, size := decode(s, i)
				i += size
				if !yield(r, i) {
					return
				}
			}
		} else if !segment(s[start:end], start, yield) {
			return
		}
		start = end
	}
}

func Count(s string) int { n := 0; Each(s, func(_ rune, _ int) bool { n++; return true }); return n }

// Prefix returns the byte end after n clusters and whether more text follows.
// Only a bounded rune prefix is segmented; grow it when the last cluster might
// continue past the buffer. Keeping a following cluster proves the nth end.
func Prefix(s string, n int) (int, bool) {
	if n <= 0 {
		return 0, len(s) > 0
	}
	limit := min(len(s), n+2)
	end, runes := 0, 0
	for {
		for end < len(s) && runes < limit {
			_, size := utf8.DecodeRuneInString(s[end:])
			end += size
			runes++
		}
		count, cut := 0, 0
		Each(s[:end], func(_ rune, b int) bool {
			count++
			if count == n {
				cut = b
			}
			return count <= n
		})
		if count > n {
			return cut, true
		}
		if end == len(s) {
			return len(s), false
		}
		limit = min(len(s), limit*2)
	}
}

// Suffix segments only enough trailing complete paragraphs to retain n clusters.
func Suffix(s string, n int) (int, bool) {
	if n <= 0 {
		return len(s), len(s) > 0
	}
	start := len(s)
	for {
		prev := strings.LastIndexByte(s[:max(0, start-1)], '\n') + 1
		start = prev
		ends := []int{start}
		Each(s[start:], func(_ rune, b int) bool { ends = append(ends, start+b); return true })
		if len(ends)-1 >= n || start == 0 {
			cut := ends[max(0, len(ends)-1-n)]
			return cut, cut > 0
		}
	}
}

// NonSpace counts clusters whose first rune is not whitespace. It spells out
// decode inline: the counter runs on every keystroke and decode is too big for
// the compiler to inline.
func NonSpace(s string) int {
	n, start, saved := 0, 0, 0
	for i := 0; i < len(s); {
		var r rune
		size := 1
		b := s[i]
		if b < utf8.RuneSelf {
			r = rune(b)
		} else if b&0xf0 == 0xe0 && i+2 < len(s) && s[i+1]&0xc0 == 0x80 && s[i+2]&0xc0 == 0x80 {
			r = rune(b&15)<<12 | rune(s[i+1]&63)<<6 | rune(s[i+2]&63)
			size = 3
			if r < 0x800 || r >= 0xd800 && r <= 0xdfff {
				r = -1
			}
		} else {
			r, size = utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				r = -1
			}
		}
		if r >= 0 && r < 65536 && kind[r] != 0 {
			n += int(kind[r] & 1)
			i += size
			if r == '\n' {
				start, saved = i, n
			}
			continue
		}
		end := paragraphEnd(s, i)
		n = saved
		segment(s[start:end], 0, func(r rune, _ int) bool {
			if !unicode.IsSpace(r) {
				n++
			}
			return true
		})
		i, start, saved = end, end, n
	}
	return n
}
