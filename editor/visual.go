package editor

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

func orderedGlyphs(gs []ui.Glyph) bool {
	for i := 1; i < len(gs); i++ {
		if gs[i].Cluster < gs[i-1].Cluster || gs[i].X < gs[i-1].X {
			return false
		}
	}
	return true
}

type glyphCluster struct {
	a, b        int
	left, width float32
	rtl         bool
}

// visualClusters keeps the shaper's visual order. Logical advances are
// separate from visual coordinates: wrapping must not subtract RTL carets.
func visualClusters(n int, gs []ui.Glyph) []glyphCluster {
	var cs []glyphCluster
	byRune := make(map[int]int)
	for _, g := range visualGlyphs(gs) {
		if g.Cluster < 0 || g.Cluster >= n {
			continue
		}
		i, ok := byRune[g.Cluster]
		if !ok {
			i = len(cs)
			byRune[g.Cluster] = i
			cs = append(cs, glyphCluster{a: g.Cluster, left: g.X})
		}
		c := &cs[i]
		right := max(c.left+c.width, g.X+g.Advance)
		c.left = min(c.left, g.X)
		c.width = right - c.left
	}
	logical := make([]int, 0, len(cs))
	for a := range byRune {
		logical = append(logical, a)
	}
	slices.Sort(logical)
	for i, a := range logical {
		b := n
		if i+1 < len(logical) {
			b = logical[i+1]
		}
		cs[byRune[a]].b = b
	}
	for i := range cs {
		cs[i].rtl = i > 0 && cs[i-1].a > cs[i].a || i+1 < len(cs) && cs[i+1].a < cs[i].a
	}
	return cs
}

type visualLine struct {
	glyphs   []ui.Glyph
	from, to []float32 // directed edges of each logical rune
	width    float32
}

// view packs only this line's clusters, preserving visual order and glyph
// offsets within clusters. Wrapped RTL fragments cannot use xs[start] as
// their drawing origin. Cache the result for painting and pointer input.
func (s *shaped) view(a, b int) *visualLine {
	key := [2]int{a, b}
	if v := s.views[key]; v != nil {
		return v
	}
	v := &visualLine{from: make([]float32, b-a), to: make([]float32, b-a)}
	shift := make(map[int]float32)
	for _, c := range s.visual {
		lo, hi := max(a, c.a), min(b, c.b)
		if lo >= hi {
			continue
		}
		step := c.width / float32(c.b-c.a)
		for d := lo; d < hi; d++ {
			x := v.width + float32(d-lo)*step
			if c.rtl {
				x = v.width + float32(hi-1-d)*step
				v.from[d-a], v.to[d-a] = x+step, x
			} else {
				v.from[d-a], v.to[d-a] = x, x+step
			}
		}
		shift[c.a] = v.width - c.left
		v.width += float32(hi-lo) * step
	}
	v.glyphs = s.run(a, b)
	for i := range v.glyphs {
		v.glyphs[i].X += shift[v.glyphs[i].Cluster]
	}
	s.views[key] = v
	return v
}

// run filters a color span without repacking it: colors within one line
// must share the same visual placement.
func (v *visualLine) run(a, b int) []ui.Glyph {
	var out []ui.Glyph
	for _, g := range v.glyphs {
		if g.Cluster >= a && g.Cluster < b {
			out = append(out, g)
		}
	}
	return out
}

func (v *visualLine) boundary(d int) float32 {
	if len(v.from) == 0 {
		return 0
	}
	if d == len(v.from) {
		return v.to[d-1]
	}
	return v.from[d]
}

// ranges emits disjoint visual intervals; a logical range crossing a bidi
// boundary need not be one rectangle. Adjacent intervals are merged.
func (v *visualLine) ranges(a, b int, fn func(float32, float32)) {
	var spans [][2]float32
	for d := a; d < b; d++ {
		spans = append(spans, [2]float32{min(v.from[d], v.to[d]), max(v.from[d], v.to[d])})
	}
	slices.SortFunc(spans, func(a, b [2]float32) int {
		if a[0] < b[0] {
			return -1
		}
		if a[0] > b[0] {
			return 1
		}
		return 0
	})
	for i := 0; i < len(spans); {
		lo, hi := spans[i][0], spans[i][1]
		i++
		for i < len(spans) && spans[i][0] <= hi {
			hi = max(hi, spans[i][1])
			i++
		}
		fn(lo, hi)
	}
}

// visualGlyphs normalizes DirectWrite's logical order without changing the
// cached input or glyph offsets. Equal X positions retain their shaping order.
func visualGlyphs(gs []ui.Glyph) []ui.Glyph {
	for i := 1; i < len(gs); i++ {
		if gs[i].X < gs[i-1].X {
			gs = slices.Clone(gs)
			slices.SortStableFunc(gs, func(a, b ui.Glyph) int {
				if a.X < b.X {
					return -1
				}
				if a.X > b.X {
					return 1
				}
				return 0
			})
			break
		}
	}
	return gs
}
