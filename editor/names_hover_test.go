package editor

import (
	"image"
	"testing"
)

func nameLineChanged(a, b image.Image, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				return true
			}
		}
	}
	return false
}

func TestOnlyTheHoveredNameHasALine(t *testing.T) {
	e := New(NewDoc("林晚走来，林晚推门。"), Options{})
	e.SetNameLinks([]NameLink{{"林晚", "entry"}}, nil)
	tt := newTester(e)
	tt.Move(590, 390)
	base := tt.Image()
	first, _ := e.RangeBox(0, 2)
	second, _ := e.RangeBox(5, 7)
	line := func(x float32) image.Rectangle {
		return image.Rect(int(x), int(first.Y+e.lay.lineH-6), int(x+e.FontSize()*2), int(first.Y+e.lay.lineH-3))
	}
	tt.Move(first.X+e.FontSize()/2, first.Y+first.H/2)
	hover := tt.Image()
	if e.nameHover.a != 0 || e.nameHover.b != 2 || !nameLineChanged(base, hover, line(first.X)) || nameLineChanged(base, hover, line(second.X)) {
		t.Fatal("hover must underline this occurrence alone", e.nameHover)
	}
	tt.Move(second.X+e.FontSize()/2, second.Y+second.H/2)
	hover = tt.Image()
	if e.nameHover.a != 5 || nameLineChanged(base, hover, line(first.X)) || !nameLineChanged(base, hover, line(second.X)) {
		t.Fatal("underline did not follow pointer", e.nameHover)
	}
	tt.Move(590, 390)
	if e.nameHover.entry != "" || nameLineChanged(base, tt.Image(), line(second.X)) {
		t.Fatal("underline remained after pointer left")
	}
}
