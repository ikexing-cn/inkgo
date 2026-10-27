package editor

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTheScrollBarPaintsAndTakesInputAtItsOffset(t *testing.T) {
	e := New(NewDoc(strings.Repeat("雨还没有停。\n", 200)), Options{})
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Children(func() { e.View(c).Width(400).FillHeight() })
	}, 600, 400)
	for _, offset := range []float32{-80, 80, 0} {
		e.SetScrollBarOffset(offset)
		e.MoveTo(0, false)
		tt.Move(0, 0)
		tt.Frame()
		tt.Frame()
		right := float32(400) + offset
		bar, ok := tt.Find("正文滚动条")
		if !ok || bar.X+bar.W != right {
			t.Fatalf("offset %v: bar at %v", offset, bar)
		}
		img := tt.Image()
		if img.At(int(right)-6, 12) == img.At(int(right)-14, 12) {
			t.Fatal("scroll bar was clipped or did not move")
		}
		tt.Move(right-6, 12)
		tt.Frame()
		if !e.barHot || tt.Cursor() != ui.CursorDefault {
			t.Fatal("moved scroll bar did not take hover")
		}
		_, h := e.thumb()
		tt.Press(right-6, h/2)
		tt.Move(right-6, h/2+80)
		tt.Release(right-6, h/2+80)
		if e.scrollY == 0 || e.barDrag {
			t.Fatal("moved thumb did not drag and release")
		}
		e.MoveTo(0, false)
		tt.Frame()
		tt.ClickAt(right-6, 200)
		if e.scrollY == 0 {
			t.Fatal("moved track did not page")
		}
		if offset != 0 && e.onBar(398) {
			t.Fatal("old scroll bar position still takes presses")
		}
	}
}
