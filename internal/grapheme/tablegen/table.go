// Package tablegen derives fast-path tables through the public segmenter API.
package tablegen

import "github.com/go-text/typesetting/segmenter"

// Tables excludes surrogates and admits X exactly when XX, aX, Xa, LF-X,
// and X-LF each contain two clusters. Under UAX #29, aX excludes Extend,
// ZWJ and SpacingMark; Xa excludes Prepend; XX excludes RI and L/V/T;
// X-LF excludes CR. Remaining Other, LV, LVT, Control except CR, and LF
// break pairwise, so paragraphs consisting entirely of admitted runes are safe.
func Tables() (simple [8192]byte) {
	var seg segmenter.Segmenter
	for r := rune(0); r < 65536; r++ {
		if r >= 0xd800 && r <= 0xdfff {
			continue
		}
		ok := true
		for _, pair := range [][2]rune{{r, r}, {'a', r}, {r, 'a'}, {'\n', r}, {r, '\n'}} {
			seg.Init(pair[:])
			it := seg.GraphemeIterator()
			n := 0
			for it.Next() {
				n++
			}
			if n != 2 {
				ok = false
				break
			}
		}
		if ok {
			simple[r>>3] |= 1 << uint(r&7)
		}
	}
	return
}
