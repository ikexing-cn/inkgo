package main

import (
	"testing"

	"github.com/egoist/mygo"
)

type sizedWindow struct {
	bounds                     mygo.Rectangle
	minWidth, minHeight, sizes int
}

func (w *sizedWindow) Bounds() mygo.Rectangle           { return w.bounds }
func (w *sizedWindow) SetMinimumSize(width, height int) { w.minWidth, w.minHeight = width, height }
func (w *sizedWindow) SetSize(width, height int) {
	w.bounds.Width, w.bounds.Height = width, height
	w.sizes++
}

// A saved scale grows a restored window; reducing it only lowers the limit.
func TestZoomGrowsTheWindowWithoutShrinkingIt(t *testing.T) {
	w := &sizedWindow{bounds: mygo.Rectangle{Width: windowWidth, Height: windowHeight}}
	resizeForZoom(w, 1.5)
	if w.minWidth != 1500 || w.minHeight != 1020 || w.bounds.Width != 1500 || w.bounds.Height != 1020 || w.sizes != 1 {
		t.Fatalf("grown window: %+v", w)
	}
	resizeForZoom(w, 1)
	if w.minWidth != windowMinWidth || w.minHeight != windowMinHeight || w.bounds.Width != 1500 || w.bounds.Height != 1020 || w.sizes != 1 {
		t.Fatalf("reduced limit: %+v", w)
	}
	w.bounds.Width = 2000
	resizeForZoom(w, 1.75)
	if w.bounds.Width != 2000 || w.bounds.Height != 1190 || w.sizes != 2 {
		t.Fatalf("only the short axis grows: %+v", w)
	}
}

// Work areas and window sizes are both DIPs, regardless of screen density.
func TestZoomFitsTheDisplayWorkArea(t *testing.T) {
	area := &mygo.Rectangle{Width: 1500, Height: 1020}
	if !zoomFits(area, 1.5) || zoomFits(area, 1.75) || zoomFits(nil, 1) {
		t.Fatal("the work area's limit was not used")
	}
	area.Width = 1499
	if zoomFits(area, 1.5) {
		t.Fatal("the width does not fit")
	}
	area.Width, area.Height = 1500, 1019
	if zoomFits(area, 1.5) {
		t.Fatal("the height does not fit")
	}
	w, h := zoomMinimum(1.1)
	if w != 1100 || h != 748 {
		t.Fatalf("whole DIPs rounded up: %d x %d", w, h)
	}
}
