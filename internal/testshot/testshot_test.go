package testshot

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveWritesNamedPNGInDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "screens")
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 17, A: 255})
	t.Setenv("SHOT", dir)
	Save(t, "sample.png", img)
	f, err := os.Open(filepath.Join(dir, "sample.png"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != img.Bounds() {
		t.Fatalf("bounds = %v, want %v", got.Bounds(), img.Bounds())
	}
}

func TestDirReturnsSHOT(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHOT", dir)
	if !Enabled() {
		t.Fatal("Enabled is false with SHOT set")
	}
	if got := Dir(t); got != dir {
		t.Fatalf("Dir = %q, want %q", got, dir)
	}
}

func TestDirSkipsWhenSHOTIsEmpty(t *testing.T) {
	t.Setenv("SHOT", "")
	called := false
	t.Run("unset", func(t *testing.T) {
		Dir(t)
		called = true
	})
	if called {
		t.Fatal("Dir did not skip")
	}
}

func TestSaveWithoutSHOTDoesNothing(t *testing.T) {
	t.Setenv("SHOT", "")
	if Enabled() {
		t.Fatal("Enabled with empty SHOT")
	}
	Save(t, "unused.png", nil)
}
