package editor

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Changing the column must reflow even when the viewport, font and text
// do not change. Widening the font can also widen a 36-character column.
func TestTextColumnReflowsInsideTheViewport(t *testing.T) {
	e := New(NewDoc(strings.Repeat("雨从傍晚开始下。", 300)), Options{})
	tt := newTester(e)
	tt.SetSize(1800, 600)
	for _, size := range []float32{17, 28} {
		e.SetFontSize(size)
		for _, margin := range []float32{40, 24} {
			column := TextColumn{MaxWidth: size * 36, Margin: margin}
			e.SetTextColumn(column)
			tt.Frame()
			left, width := column.Layout(1800)
			if e.width != 1800 || e.lay.textW != width || e.lay.left != left || width != size*36 {
				t.Fatalf("column %+v: viewport %v, text %v at %v", column, e.width, e.lay.textW, e.lay.left)
			}
		}
	}
	tt.SetSize(600, 400)
	for _, margin := range []float32{40, 24} {
		e.SetTextColumn(TextColumn{MaxWidth: 17 * 36, Margin: margin})
		tt.Frame()
		if e.lay.left != margin || e.lay.textW != 600-2*margin {
			t.Fatalf("margin %v: text %v at %v", margin, e.lay.textW, e.lay.left)
		}
	}
}

// The column only places text: both blank margins take input, and the
// visible scroll bar remains at the viewport's right edge.
func TestTextColumnKeepsTheMarginsAndScrollBar(t *testing.T) {
	for _, width := range []int{600, 1800} {
		e := New(NewDoc(strings.Repeat("雨从傍晚开始下。\n", 200)), Options{Column: TextColumn{MaxWidth: 17 * 36, Margin: 24}})
		tt := newTester(e)
		tt.SetSize(width, 400)
		tt.Frame()
		for _, x := range []float32{2, float32(width - 20)} {
			before := e.scrollY
			tt.Scroll(x, 200, 0, 120)
			if e.scrollY <= before {
				t.Fatalf("%d wide: blank margin at %v did not scroll", width, x)
			}
			tt.ClickAt(x, 200)
			if !e.focused {
				t.Fatalf("%d wide: blank margin at %v did not take the caret", width, x)
			}
		}
		e.MoveTo(0, false)
		tt.Frame()
		tt.Move(float32(width-6), 12)
		tt.Frame()
		img := tt.Image()
		if !e.barHot || tt.Cursor() != ui.CursorDefault || img.At(width-6, 12) == img.At(width-14, 12) {
			t.Fatalf("%d wide: scroll bar is not visible at the viewport edge", width)
		}
		_, h := e.thumb()
		tt.Press(float32(width-6), h/2)
		tt.Move(float32(width-6), h/2+80)
		tt.Release(float32(width-6), h/2+80)
		if e.scrollY == 0 {
			t.Fatalf("%d wide: scroll bar at the viewport edge did not drag", width)
		}
	}
}

func TestTextColumnTranslatesWithoutReflow(t *testing.T) {
	e := New(NewDoc(strings.Repeat("雨从傍晚开始下。\n", 100)), Options{Column: TextColumn{MaxWidth: 612, Margin: 24}})
	tt := newTester(e)
	tt.Frame()
	lines, width, left, scroll := &e.lay.lines[0], e.lay.textW, e.lay.left, e.scrollY
	before, _ := e.RangeBoxUnclipped(0, 2)
	for _, offset := range []float32{-80, -40, 12, 0} {
		e.SetTextColumn(TextColumn{MaxWidth: 612, Margin: 24, OffsetX: offset})
		tt.Frame()
		after, _ := e.RangeBoxUnclipped(0, 2)
		if &e.lay.lines[0] != lines || e.lay.textW != width || e.lay.left != left+offset || e.scrollY != scroll || after.X != before.X+offset {
			t.Fatalf("offset %v changed the layout or did not move its range: %+v", offset, after)
		}
		x, y := e.caretPos()
		if got := e.hit(x+1, y-e.scrollY+1); got != e.caret {
			t.Fatalf("translated pointer found %d, want %d", got, e.caret)
		}
	}
}
