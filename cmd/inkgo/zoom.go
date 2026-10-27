package main

import (
	"math"

	"github.com/egoist/mygo"
)

const (
	windowWidth       = 1280
	windowHeight      = 820
	windowMinWidth    = 1000
	windowMinHeight   = 680
	windowTitleHeight = 40
)

// zoomMinimum is the window's least size, rounded up to whole DIPs.
func zoomMinimum(z float32) (int, int) {
	return int(math.Ceil(float64(windowMinWidth * z))), int(math.Ceil(float64(windowMinHeight * z)))
}

// zoomFits compares window and screen DIPs, without applying system DPI again.
func zoomFits(area *mygo.Rectangle, z float32) bool {
	if area == nil {
		return false
	}
	w, h := zoomMinimum(z)
	return w <= area.Width && h <= area.Height
}

type zoomWindow interface {
	Bounds() mygo.Rectangle
	SetMinimumSize(int, int)
	SetSize(int, int)
}

// resizeForZoom updates the limit and only grows dimensions that need room.
// Windows SetMinimumSize records a limit without resizing the current window.
func resizeForZoom(w zoomWindow, z float32) {
	width, height := zoomMinimum(z)
	w.SetMinimumSize(width, height)
	b := w.Bounds()
	if b.Width < width || b.Height < height {
		w.SetSize(max(b.Width, width), max(b.Height, height))
	}
}
