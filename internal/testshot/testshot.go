// Package testshot writes named screenshots for Go tests.
package testshot

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// Dir returns SHOT, skipping a test that only draws when it is unset.
func Dir(t testing.TB) string {
	t.Helper()
	dir := os.Getenv("SHOT")
	if dir == "" {
		t.Skip("set SHOT=dir")
	}
	return dir
}

// Enabled lets a caller avoid advancing the UI for an optional screenshot.
func Enabled() bool { return os.Getenv("SHOT") != "" }

// Save writes name into SHOT. An empty SHOT is a no-op.
func Save(t testing.TB, name string, img image.Image) {
	t.Helper()
	dir := os.Getenv("SHOT")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
