// Package grapheme provides rune-indexed Unicode grapheme boundaries.
package grapheme

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-text/typesetting/segmenter"
)

// simple deliberately admits only known independent characters. Everything
// else, including all Extend/ZWJ/SpacingMark/Prepend/RI/L/V/T characters,
// takes the Unicode path. Precomposed Hangul is safe only in a simple paragraph.
func simple(r rune) bool {
	if r < 128 {
		return r != '\r'
	}
	return r >= 0x3400 && r <= 0x9fff || r >= 0xac00 && r <= 0xd7a3 ||
		strings.ContainsRune("，。、；：？！）》」』”’…—·（《「『“‘　", r)
}

// Each calls yield for each cluster's first rune and ending byte offset.
// Returning false stops iteration. Newlines stay with their preceding paragraph
// so CRLF and combining sequences on either side retain Unicode semantics.
func Each(s string, yield func(rune, int) bool) {
	base := 0
	for len(s) > 0 {
		end := strings.IndexByte(s, '\n')
		if end < 0 {
			end = len(s)
		} else {
			end++
		}
		p := s[:end]
		fast := true
		for _, r := range p {
			if !simple(r) {
				fast = false
				break
			}
		}
		if fast {
			for i, r := range p {
				if !yield(r, base+i+utf8.RuneLen(r)) {
					return
				}
			}
		} else {
			var seg segmenter.Segmenter
			seg.Init([]rune(p))
			it := seg.GraphemeIterator()
			for it.Next() {
				g := it.Grapheme()
				if !yield(g.Text[0], base+g.OffsetInBytes+g.LengthInBytes) {
					return
				}
			}
		}
		base += end
		s = s[end:]
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

// NonSpace counts clusters whose first rune is not whitespace.
func NonSpace(s string) int {
	n := 0
	for len(s) > 0 {
		end := strings.IndexByte(s, '\n')
		if end < 0 {
			end = len(s)
		} else {
			end++
		}
		p := s[:end]
		fast, local := true, 0
		for _, r := range p {
			if !simple(r) {
				fast = false
				break
			}
			if !unicode.IsSpace(r) {
				local++
			}
		}
		if fast {
			n += local
		} else {
			Each(p, func(r rune, _ int) bool {
				if !unicode.IsSpace(r) {
					n++
				}
				return true
			})
		}
		s = s[end:]
	}
	return n
}
