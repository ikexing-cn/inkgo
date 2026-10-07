package main

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"writer/editor"
)

// TestLayout checks the editor fills the window, and writes a screenshot
// when SHOT is set.
func TestLayout(t *testing.T) {
	a := &app{ed: editor.New(editor.NewDoc(sample), editor.Options{})}
	var r ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		a.view(c)
		r = a.lastEditor
	}, 1086, 680)
	tt.Frame()
	tt.Frame()
	if r.Y > 1 || r.H < 600 {
		t.Errorf("editor bounds %+v, want the full height", r)
	}
	if path := os.Getenv("SHOT"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		png.Encode(f, tt.Image())
	}
}
