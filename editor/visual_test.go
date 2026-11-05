package editor

import (
	"reflect"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Two glyphs share cluster 2; visual and logical ordering differ.
func visualFixture() shaped {
	gs := []ui.Glyph{{Cluster: 0, X: 0, Advance: 10}, {Cluster: 3, X: 10, Advance: 10}, {Cluster: 2, X: 20, Advance: 6}, {Cluster: 2, X: 26, Advance: 4}, {Cluster: 1, X: 30, Advance: 10}, {Cluster: 4, X: 40, Advance: 10}}
	return indexGlyphs(gs, positions(5, gs))
}

func TestUnorderedRunsPreserveVisualOrderAndSharedClusters(t *testing.T) {
	s := visualFixture()
	for a := 0; a <= 5; a++ {
		for b := a; b <= 5; b++ {
			var want []ui.Glyph
			for _, g := range s.glyphs {
				if g.Cluster >= a && g.Cluster < b {
					want = append(want, g)
				}
			}
			if !slices.Equal(s.run(a, b), want) {
				t.Fatalf("[%d,%d): %v, want %v", a, b, s.run(a, b), want)
			}
		}
	}
	gs := []ui.Glyph{{Cluster: 0, Advance: 10}, {Cluster: 0, X: 6, Advance: 4}, {Cluster: 2, X: 10, Advance: 10}}
	ordered := indexGlyphs(gs, positions(3, gs))
	if &ordered.run(0, 2)[0] != &gs[0] {
		t.Fatal("ordered runs must remain zero-copy slices")
	}
}

func TestUnorderedPositionsWrapByLogicalAdvanceIncludingLigatures(t *testing.T) {
	s := visualFixture()
	if !slices.Equal(s.xs, []float32{0, 10, 20, 30, 40, 50}) {
		t.Fatal(s.xs)
	}
	_, starts := layoutParagraph([]rune("甲乙丙丁戊"), ui.Font{}, 20, 0, 0, &s)
	if !slices.Equal(starts, []int{0, 2, 4}) {
		t.Fatal(starts)
	}
	gs := []ui.Glyph{{Cluster: 4, X: 0, Advance: 20}, {Cluster: 2, X: 20, Advance: 10}, {Cluster: 0, X: 30, Advance: 30}}
	if got := positions(6, gs); !slices.Equal(got, []float32{0, 15, 30, 35, 40, 50, 60}) {
		t.Fatal(got)
	}
}

func TestUnorderedWrappedGlyphsKeepVisualOrderAndClusterOffsets(t *testing.T) {
	s := visualFixture()
	before := slices.Clone(s.glyphs)
	v := s.view(2, 4)
	if v.width != 20 || len(v.glyphs) != 3 {
		t.Fatal(v)
	}
	for i, want := range []float32{0, 10, 16} {
		if v.glyphs[i].X != want {
			t.Fatal(v.glyphs)
		}
	}
	if !slices.Equal(before, s.glyphs) {
		t.Fatal("line placement mutated cached paragraph")
	}
	if s.view(2, 4) != v {
		t.Fatal("line geometry was rebuilt")
	}
}

func TestUnorderedCaretAndHitTestingUseDirectedVisualEdges(t *testing.T) {
	s := visualFixture()
	l := layout{lines: []line{{start: 0, end: 5, sh: &s, last: true, dx: 3}}}
	for i, want := range []float32{3, 43, 33, 23, 43, 53} {
		if got := l.x(&l.lines[0], i); got != want {
			t.Fatalf("boundary %d: %v want %v", i, got, want)
		}
	}
	for _, c := range []struct {
		x    float32
		want int
	}{{5, 0}, {15, 4}, {25, 3}, {35, 2}, {51, 5}} {
		if got := l.hitLine(0, c.x); got != c.want {
			t.Fatalf("hit %v: %d want %d", c.x, got, c.want)
		}
	}
	p := Paragraph{sh: s, Lines: []TextLine{{Start: 0, End: 5, X: 3}}}
	for _, c := range []struct {
		x    float32
		want int
	}{{-1, 0}, {8, 1}, {18, 4}, {28, 3}, {38, 2}, {48, 5}, {60, 5}} {
		if got := p.HitAfter(0, c.x); got != c.want {
			t.Fatalf("HitAfter %v: %d want %d", c.x, got, c.want)
		}
	}
	for i := 0; i <= 5; i++ {
		if p.X(0, i) != l.x(&l.lines[0], i) {
			t.Fatal("preview and editor carets disagree")
		}
	}
}

func TestUnorderedSelectionAndMarksSplitAtVisualGaps(t *testing.T) {
	s := visualFixture()
	l := layout{lines: []line{{start: 0, end: 5, sh: &s, last: true, dx: 3, y: 7}}}
	var got [][3]float32
	l.eachVisible(0, 2, 0, 10, func(a, b, y float32) { got = append(got, [3]float32{a, b, y}) })
	if !slices.Equal(got, [][3]float32{{3, 13, 7}, {33, 43, 7}}) {
		t.Fatal(got)
	}
	got = nil
	l.ranges(3, 6, func(a, b, y float32) { got = append(got, [3]float32{a, b, y}) })
	if !slices.Equal(got, [][3]float32{{13, 23, 7}, {43, 53, 7}, {53, 59, 7}}) {
		t.Fatal(got)
	}
}

func TestUnorderedMarkedAndNameHoverExcludeVisualGaps(t *testing.T) {
	s := visualFixture()
	e := New(NewDoc("甲乙丙丁戊"), Options{})
	e.width = 200
	e.shapes = []paragraphShape{{text: e.Doc.text, sh: &s}}
	e.ensureLayout()
	e.SetDoubts([][2]int{{0, 2}})
	supplyTestMarks(e)
	ln := &e.lay.lines[0]
	for _, c := range []struct {
		x   float32
		hit bool
	}{{5, true}, {15, false}, {25, false}, {35, true}, {45, false}} {
		got := e.markedRangeAt(e.lay.left+c.x, ln.y+5)
		if (got == [2]int{0, 2}) != c.hit {
			t.Fatalf("mark x %v: %v", c.x, got)
		}
	}
	e.SetDoubts(nil)
	supplyTestMarks(e)
	e.SetNameLinks([]NameLink{{Name: "甲乙", Entry: "entry"}}, nil)
	for _, c := range []struct {
		x   float32
		hit bool
	}{{5, true}, {15, false}, {35, true}, {45, false}} {
		if got := e.nameAt(e.lay.left+c.x, ln.y+5); (got == "entry") != c.hit {
			t.Fatalf("name x %v: %v", c.x, got)
		}
	}
}

func TestUnorderedProposalGapReservesWidthWithoutDrawingGlyph(t *testing.T) {
	s := visualFixture()
	sh := shapeGlyphs([]rune{'甲', gapRune, '丙', '丁', '戊'}, slices.Clone(s.glyphs), 25)
	if !slices.Equal(sh.xs, []float32{0, 10, 35, 45, 55, 65}) {
		t.Fatal(sh.xs)
	}
	if len(sh.run(1, 2)) != 0 {
		t.Fatal("gap has a drawable glyph")
	}
	v := sh.view(0, 5)
	if v.width != 65 || v.from[1]-v.to[1] != 25 || v.glyphs[len(v.glyphs)-1].X != 55 {
		t.Fatal(v)
	}
}

func TestUnorderedDecorationsPaintOnlyTheirVisualIntervals(t *testing.T) {
	s := visualFixture()
	e := New(NewDoc("甲乙丙丁戊"), Options{})
	e.lay = layout{lines: []line{{start: 0, end: 5, sh: &s, last: true, y: 5}}, lineH: 24}
	for _, kind := range []string{"selection", "highlight", "mark"} {
		t.Run(kind, func(t *testing.T) {
			e.anchor, e.caret = 0, 0
			e.noteHighlight = [2]int{}
			e.Doc.marks = nil
			switch kind {
			case "selection":
				e.caret = 2
			case "highlight":
				e.noteHighlight = [2]int{0, 2}
			case "mark":
				e.Doc.marks = []Mark{{Kind: MarkAI, Start: 0, End: 2}}
			}
			supplyTestMarks(e)
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Box(c).Fill().Background(ui.RGB(255, 255, 255)).Draw(func(p *ui.Painter, r ui.Rect) { e.paintDecorations(p, 10, 0, 0, 100) })
			}, 80, 40)
			tt.Frame()
			img := tt.Image()
			for _, x := range []int{15, 25, 35, 45} {
				painted := false
				for y := 5; y < 30; y++ {
					c := img.RGBAAt(x, y)
					if c.R != 255 || c.G != 255 || c.B != 255 {
						painted = true
					}
				}
				if painted != (x == 15 || x == 45) {
					t.Fatalf("x %d painted %v", x, painted)
				}
			}
		})
	}
}

func TestUnorderedColoredRunsShareWholeLinePlacement(t *testing.T) {
	s := visualFixture()
	v := s.view(1, 4)
	run := v.run(1, 3)
	if len(run) != 3 {
		t.Fatal(run)
	}
	for i, want := range []float32{10, 16, 20} {
		if run[i].X != want {
			t.Fatal(run)
		}
	}
	if run[0].Cluster != 2 || run[2].Cluster != 1 {
		t.Fatal("color span changed visual order", run)
	}
}

func TestReversedClustersKeepDirectedLigatureCaretsAfterWrapping(t *testing.T) {
	gs := []ui.Glyph{{Cluster: 2, X: 0, Advance: 10}, {Cluster: 0, X: 10, Advance: 20}}
	s := indexGlyphs(gs, positions(3, gs))
	v := s.view(0, 2)
	if v.width != 20 || v.boundary(0) != 20 || v.boundary(1) != 10 || v.boundary(2) != 0 || v.glyphs[0].X != 0 {
		t.Fatal(v)
	}
}

func TestLogicalOrderWithDescendingXMatchesVisualOrderForWrappingCaretsHitsAndSelections(t *testing.T) {
	// Cluster 2 has offset glyphs, including equal X positions whose order is stable.
	visual := []ui.Glyph{{Cluster: 0, X: 0, Advance: 10}, {Cluster: 3, X: 10, Advance: 10}, {Cluster: 2, X: 20, Advance: 10}, {Cluster: 2, X: 22, Advance: 0}, {Cluster: 2, X: 22, Advance: 1}, {Cluster: 1, X: 30, Advance: 10}, {Cluster: 4, X: 40, Advance: 10}}
	logical := slices.Clone(visual)
	slices.SortStableFunc(logical, func(a, b ui.Glyph) int { return a.Cluster - b.Cluster })
	// Offset marks may precede their base even within a shared cluster.
	logical[2], logical[3], logical[4] = logical[3], logical[4], logical[2]
	before := slices.Clone(logical)
	a := shapeGlyphs([]rune("甲乙丙丁戊"), visual, 0)
	b := shapeGlyphs([]rune("甲乙丙丁戊"), logical, 0)
	if !slices.Equal(logical, before) {
		t.Fatal("mutated shaper input")
	}
	if !slices.Equal(a.glyphs, b.glyphs) || !slices.Equal(a.xs, b.xs) {
		t.Fatal(a, b)
	}
	for _, width := range []float32{10, 20, 30, 100} {
		_, as := layoutParagraph([]rune("甲乙丙丁戊"), ui.Font{}, width, 0, 0, &a)
		_, bs := layoutParagraph([]rune("甲乙丙丁戊"), ui.Font{}, width, 0, 0, &b)
		if !slices.Equal(as, bs) {
			t.Fatal(as, bs)
		}
		for i, start := range as {
			end := 5
			if i+1 < len(as) {
				end = as[i+1]
			}
			av, bv := a.view(start, end), b.view(start, end)
			if !reflect.DeepEqual(av, bv) {
				t.Fatal(av, bv)
			}
			la := layout{lines: []line{{start: start, end: end, sh: &a, last: true}}}
			lb := layout{lines: []line{{start: start, end: end, sh: &b, last: true}}}
			for d := start; d <= end; d++ {
				if la.x(&la.lines[0], d) != lb.x(&lb.lines[0], d) {
					t.Fatal("caret", d)
				}
			}
			for x := float32(-1); x <= av.width+1; x += 0.5 {
				if la.hitLine(0, x) != lb.hitLine(0, x) {
					t.Fatal("hit", x)
				}
			}
			for lo := 0; lo <= end-start; lo++ {
				for hi := lo; hi <= end-start; hi++ {
					var ar, br [][2]float32
					av.ranges(lo, hi, func(x, y float32) { ar = append(ar, [2]float32{x, y}) })
					bv.ranges(lo, hi, func(x, y float32) { br = append(br, [2]float32{x, y}) })
					if !slices.Equal(ar, br) {
						t.Fatal(ar, br)
					}
				}
			}
		}
	}
}
